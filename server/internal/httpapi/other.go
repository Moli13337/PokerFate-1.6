package httpapi

import (
	"context"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"

	"poker-fate-server/internal/gamedata"
)

func (r *Router) OKHandler(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"code": 0})
}

func (r *Router) EmptyListHandler(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"code": 0, "list": []interface{}{}})
}

// EmptyDataHandler returns an empty object for `data` instead of nil, which
// would cause Lua pairs()/indexing errors on the client.
func (r *Router) EmptyDataHandler(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"code": 0, "data": gin.H{}})
}

// --- Mail ---

// MailNumHandler returns the count structure EmailModel expects:
// {normal:{total,unread_total,del_total,item_total}, special:{...}}
func (r *Router) MailNumHandler(c *gin.Context) {
	uid, _ := c.Get("uid")
	var normalTotal, normalUnread int
	r.DB.QueryRowContext(context.Background(),
		`SELECT COUNT(*), COUNT(*) FILTER (WHERE is_read=false) FROM mails WHERE uid=$1 AND type=0`, uid).
		Scan(&normalTotal, &normalUnread)
	zero := gin.H{"total": 0, "unread_total": 0, "del_total": 0, "item_total": 0}
	c.JSON(http.StatusOK, gin.H{
		"code": 0,
		"normal": gin.H{
			"total":        normalTotal,
			"unread_total": normalUnread,
			"del_total":    0,
			"item_total":   normalTotal,
		},
		"special": zero,
	})
}

// MailListHandler returns the flat structure EmailModel.reqEmailList expects:
// top-level total/unread_total/del_total/item_total + list array.
func (r *Router) MailListHandler(c *gin.Context) {
	uid, _ := c.Get("uid")
	rows, err := r.DB.QueryContext(context.Background(),
		`SELECT id, type, title, content, rewards, is_read, is_received, created_at FROM mails WHERE uid=$1 ORDER BY created_at DESC LIMIT 50`, uid)
	if err != nil {
		c.JSON(http.StatusOK, gin.H{"code": 0, "list": []interface{}{}, "total": 0, "unread_total": 0, "del_total": 0, "item_total": 0})
		return
	}
	defer rows.Close()

	var list []gin.H
	for rows.Next() {
		var id int64
		var mType int
		var title, content, rewards string
		var isRead, isReceived bool
		var createdAt time.Time
		rows.Scan(&id, &mType, &title, &content, &rewards, &isRead, &isReceived, &createdAt)
		list = append(list, gin.H{
			"id":          id,
			"type":        mType,
			"title":       title,
			"content":     content,
			"rewards":     rewards,
			"is_read":     isRead,
			"is_received": isReceived,
			"created_at":  createdAt.Unix(),
		})
	}

	total := len(list)
	unread := 0
	for _, m := range list {
		if !m["is_read"].(bool) {
			unread++
		}
	}
	c.JSON(http.StatusOK, gin.H{
		"code":         0,
		"list":         list,
		"total":        total,
		"unread_total": unread,
		"del_total":    0,
		"item_total":   total,
	})
}

func (r *Router) MailDetailHandler(c *gin.Context) {
	uidVal, _ := c.Get("uid")
	uid, _ := uidVal.(int64)

	req, _ := c.Get("body")
	params, _ := req.(map[string]interface{})
	mailID := intVal(params, "mail_id")
	if mailID <= 0 {
		c.JSON(http.StatusOK, gin.H{"code": -1, "reason": "invalid mail_id"})
		return
	}

	var id int64
	var mType int
	var title, content, rewards string
	var isRead, isReceived bool
	var createdAt time.Time
	err := r.DB.QueryRowContext(context.Background(),
		`SELECT id, type, title, content, rewards::TEXT, is_read, is_received, created_at
		 FROM mails WHERE id=$1 AND uid=$2`, mailID, uid).
		Scan(&id, &mType, &title, &content, &rewards, &isRead, &isReceived, &createdAt)
	if err != nil {
		c.JSON(http.StatusOK, gin.H{"code": -1, "reason": "not found"})
		return
	}

	// Mark read on detail view.
	r.DB.ExecContext(context.Background(),
		`UPDATE mails SET is_read=true WHERE id=$1 AND uid=$2`, mailID, uid)

	c.JSON(http.StatusOK, gin.H{
		"code": 0,
		"data": gin.H{
			"id":          id,
			"type":        mType,
			"title":       title,
			"content":     content,
			"rewards":     jsonRawList([]byte(rewards)),
			"is_read":     true,
			"is_received": isReceived,
			"created_at":  createdAt.Unix(),
		},
	})
}

// MailCollHandler batch-collects all unclaimed mail rewards.
func (r *Router) MailCollHandler(c *gin.Context) {
	uidVal, _ := c.Get("uid")
	uid, _ := uidVal.(int64)

	rows, err := r.DB.QueryContext(context.Background(),
		`SELECT id, rewards::TEXT FROM mails WHERE uid=$1 AND is_received=false`, uid)
	if err != nil {
		c.JSON(http.StatusOK, gin.H{"code": 0, "list": []interface{}{}})
		return
	}
	var collected []int64
	var allRewards []interface{}
	for rows.Next() {
		var id int64
		var rewardsRaw string
		_ = rows.Scan(&id, &rewardsRaw)
		collected = append(collected, id)
		parsed := jsonRawList([]byte(rewardsRaw))
		if arr, ok := parsed.([]interface{}); ok {
			allRewards = append(allRewards, arr...)
		}
	}
	rows.Close()

	for _, id := range collected {
		r.DB.ExecContext(context.Background(),
			`UPDATE mails SET is_received=true, is_read=true WHERE id=$1 AND uid=$2`, id, uid)
	}

	c.JSON(http.StatusOK, gin.H{"code": 0, "list": allRewards})
}

func (r *Router) MailReceiveHandler(c *gin.Context) {
	uid, _ := c.Get("uid")
	req, _ := c.Get("body")
	params, _ := req.(map[string]interface{})
	mailID := intVal(params, "mail_id")
	if mailID > 0 {
		r.DB.ExecContext(context.Background(),
			`UPDATE mails SET is_received=true, is_read=true WHERE id=$1 AND uid=$2`, mailID, uid)
	}
	c.JSON(http.StatusOK, gin.H{"code": 0})
}

func (r *Router) MailDeleteHandler(c *gin.Context) {
	uid, _ := c.Get("uid")
	req, _ := c.Get("body")
	params, _ := req.(map[string]interface{})
	mailID := intVal(params, "mail_id")
	if mailID > 0 {
		r.DB.ExecContext(context.Background(),
			`DELETE FROM mails WHERE id=$1 AND uid=$2`, mailID, uid)
	}
	c.JSON(http.StatusOK, gin.H{"code": 0})
}

// --- VIP ---

func (r *Router) VipDataHandler(c *gin.Context) {
	uidVal, _ := c.Get("uid")
	uid, _ := uidVal.(int64)

	// Read vip_exp and monthly_card_exp from DB. vip_exp drives the level
	// lookup against tpl_vip_level.upgrade_exp thresholds.
	var vipExp, monthlyCardExp int64
	if uid > 0 {
		_ = r.DB.QueryRowContext(context.Background(),
			`SELECT COALESCE(vip_exp, 0), COALESCE(monthly_card_exp, 0) FROM users WHERE uid=$1`,
			uid).Scan(&vipExp, &monthlyCardExp)
	}

	// Resolve current level from config. Falls back to level 0 if the table
	// is missing (e.g. gamedata failed to load).
	level := int32(0)
	if lv, ok := gamedata.VipLevelByExp(int(vipExp)); ok {
		level = lv.ID
	}

	c.JSON(http.StatusOK, gin.H{
		"code":             0,
		"level":            level,
		"exp":              vipExp,
		"claimed_level":    r.loadClaimedVipLevels(uid),
		"reward_levels":    r.loadClaimableVipLevels(uid, level),
		"monthly_card_exp": monthlyCardExp,
	})
}

// --- Redemption ---

// RedemptionHandler redeems a gift code. Anti-replay: each (uid, code) pair is
// recorded in redemption_codes (PRIMARY KEY) so the same code can only be
// redeemed once per account.
func (r *Router) RedemptionHandler(c *gin.Context) {
	uidVal, _ := c.Get("uid")
	uid, _ := uidVal.(int64)

	req, _ := c.Get("body")
	params, _ := req.(map[string]interface{})
	code := strVal(params, "code")
	if code == "" {
		c.JSON(http.StatusOK, gin.H{"code": -1, "reason": "empty code"})
		return
	}

	// Check replay.
	var already bool
	r.DB.QueryRowContext(context.Background(),
		`SELECT EXISTS(SELECT 1 FROM redemption_codes WHERE uid=$1 AND code=$2)`,
		uid, code).Scan(&already)
	if already {
		c.JSON(http.StatusOK, gin.H{"code": -1, "reason": "already redeemed"})
		return
	}

	if code == "POKERFATE" {
		tx, err := r.DB.BeginTx(context.Background(), nil)
		if err != nil {
			c.JSON(http.StatusOK, gin.H{"code": -1, "reason": "db error"})
			return
		}
		_, _ = tx.Exec(
			`INSERT INTO redemption_codes (uid, code) VALUES ($1, $2) ON CONFLICT DO NOTHING`,
			uid, code)
		_, _ = tx.Exec(`UPDATE users SET gold = gold + 1000000 WHERE uid = $1`, uid)
		_ = tx.Commit()
		c.JSON(http.StatusOK, gin.H{"code": 0, "rewards": []gin.H{{"item_id": 10100001, "num": 1000000}}})
		go func() {
			time.Sleep(200 * time.Millisecond)
			r.pushGoldUpdate(uid)
		}()
		return
	}

	c.JSON(http.StatusOK, gin.H{"code": -1, "reason": "invalid code"})
}
