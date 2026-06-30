package httpapi

import (
	"context"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"

	"poker-fate-server/internal/gamedata"
)

// VipRewardHandler claims the VIP level reward for the user. Prevents replay
// via the user_vip_claims table (PRIMARY KEY uid+level_id). Rewards are sourced
// from tpl_vip_level.rewards (flat [item_id, count, ...] array).
func (r *Router) VipRewardHandler(c *gin.Context) {
	uidVal, _ := c.Get("uid")
	uid, _ := uidVal.(int64)

	req, _ := c.Get("body")
	params, _ := req.(map[string]interface{})
	levelID := intVal(params, "reward_level")

	if uid == 0 || levelID < 0 {
		c.JSON(http.StatusOK, gin.H{"code": -1, "reason": "invalid params"})
		return
	}

	// Look up the VIP level config and its rewards.
	vipRow, ok := gamedata.VipLevelByID(int32(levelID))
	if !ok || len(vipRow.Rewards) == 0 {
		c.JSON(http.StatusOK, gin.H{"code": -1, "reason": "invalid level"})
		return
	}

	// Anti-replay: check if already claimed.
	var already bool
	r.DB.QueryRowContext(context.Background(),
		`SELECT EXISTS(SELECT 1 FROM user_vip_claims WHERE uid=$1 AND level_id=$2)`,
		uid, levelID).Scan(&already)
	if already {
		c.JSON(http.StatusOK, gin.H{"code": -1, "reason": "already claimed"})
		return
	}

	// Insert claim record (ON CONFLICT double protection against races).
	_, err := r.DB.ExecContext(context.Background(),
		`INSERT INTO user_vip_claims (uid, level_id, claimed_at) VALUES ($1, $2, $3)
		 ON CONFLICT DO NOTHING`,
		uid, levelID, time.Now().Unix())
	if err != nil {
		c.JSON(http.StatusOK, gin.H{"code": -1, "reason": "db error"})
		return
	}

	// Parse and grant rewards from the flat [item_id, count, ...] array.
	rewards := gamedata.ParseFlatRewards(vipRow.Rewards)
	rewardList := make([]gin.H, 0, len(rewards))
	for _, rw := range rewards {
		// Apply gold (item_id 10100001) to DB.
		if rw.ItemID == 10100001 && rw.Num > 0 {
			r.DB.ExecContext(context.Background(),
				`UPDATE users SET gold = gold + $2 WHERE uid=$1`, uid, rw.Num)
		}
		rewardList = append(rewardList, gin.H{
			"major_type": rw.MajorType,
			"item_id":    rw.ItemID,
			"num":        rw.Num,
		})
	}

	c.JSON(http.StatusOK, gin.H{
		"code":         0,
		"reward_level": levelID,
		"reward_list":  rewardList,
	})
	go func() {
		time.Sleep(200 * time.Millisecond)
		r.pushGoldUpdate(uid)
	}()
}

// loadClaimedVipLevels returns the list of VIP level IDs the user has already
// claimed rewards for.
func (r *Router) loadClaimedVipLevels(uid int64) []interface{} {
	if uid == 0 {
		return []interface{}{}
	}
	rows, err := r.DB.QueryContext(context.Background(),
		`SELECT level_id FROM user_vip_claims WHERE uid=$1 ORDER BY level_id`, uid)
	if err != nil {
		return []interface{}{}
	}
	defer rows.Close()
	out := make([]interface{}, 0)
	for rows.Next() {
		var lv int
		_ = rows.Scan(&lv)
		out = append(out, lv)
	}
	return out
}

// loadClaimableVipLevels returns VIP levels <= currentLevel that the user has
// not yet claimed. Drives the client's "reward available" red dot.
func (r *Router) loadClaimableVipLevels(uid int64, currentLevel int32) []interface{} {
	if uid == 0 {
		return []interface{}{}
	}
	rows, err := r.DB.QueryContext(context.Background(),
		`SELECT level_id FROM user_vip_claims WHERE uid=$1`, uid)
	if err != nil {
		return []interface{}{}
	}
	claimed := make(map[int32]bool)
	for rows.Next() {
		var lv int
		_ = rows.Scan(&lv)
		claimed[int32(lv)] = true
	}
	rows.Close()

	out := make([]interface{}, 0)
	for _, lv := range gamedata.VipLevels() {
		if lv.ID <= currentLevel && !claimed[lv.ID] && len(lv.Rewards) > 0 {
			out = append(out, lv.ID)
		}
	}
	return out
}
