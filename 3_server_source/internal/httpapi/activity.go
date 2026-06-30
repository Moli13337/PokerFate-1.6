package httpapi

import (
	"context"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"

	"poker-fate-server/internal/gamedata"
	"poker-fate-server/internal/model"
	"poker-fate-server/internal/ws"
)

// Activity module handlers. Covers sign-in/check-in cycles, the development
// fund (rebate), soccer betting, theme & festival activities, rankings, the
// retention quiz, share/bind rewards, app comments, notices and the gacha pool.
//
// Activity-closed contract: ThemeModel.lua and SpringFestivalModel.lua assign
// data.data.start_ts/end_ts to self._start_time/_end_time without nil-checking,
// then call isActivityOpen() which compares `self._start_time > 0` every frame.
// Returning zero timestamps makes the comparison false (0 > 0) so the activity
// is treated as closed without nil-comparison crashes.
//
// Sign-in / check-in state is persisted in user_signin (migration 002). Soccer
// bets are persisted in user_soccer_bets. Quiz answers in user_question_answers.

// =============================================================================
// Sign-in / check-in
// =============================================================================

// EventCheckInDataHandler returns the newman check-in activity data.
// ActivityNewmanCheckinModel reads data.id and data.data (guarded).
func (r *Router) EventCheckInDataHandler(c *gin.Context) {
	uidVal, _ := c.Get("uid")
	uid, _ := uidVal.(int64)

	data := r.loadSignIn(uid, model.SignInTypeNewman, 1)
	c.JSON(http.StatusOK, gin.H{
		"code": 0,
		"id":   1,
		"data": data,
	})
}

// RecEventCheckInHandler performs a newman check-in. The server records the
// signed day and advances next_sign_ts by 24h.
func (r *Router) RecEventCheckInHandler(c *gin.Context) {
	r.performSignIn(c, model.SignInTypeNewman, 1)
}

// SevenSignDataHandler returns the seven-day sign-in state. cur_ts/cycle_ts
// must be numbers; days is the per-day reward array (empty is fine for a fresh
// account). SevenSignModel reads these fields directly.
func (r *Router) SevenSignDataHandler(c *gin.Context) {
	uidVal, _ := c.Get("uid")
	uid, _ := uidVal.(int64)

	data := r.loadSignIn(uid, model.SignInTypeSevenDay, 2)
	now := time.Now().Unix()
	c.JSON(http.StatusOK, gin.H{
		"code":            0,
		"cur_ts":          now,
		"cycle_ts":        data["cycle_ts"],
		"days":            data["days"],
		"next_sign_ts":    data["next_sign_ts"],
		"miss_sign_cnt":   data["miss_sign_cnt"],
		"sign_item_list":  []interface{}{},
		"total_item_list": []interface{}{},
	})
}

// SevenSignHandler performs a seven-day sign-in.
func (r *Router) SevenSignHandler(c *gin.Context) {
	r.performSignIn(c, model.SignInTypeSevenDay, 2)
}

// =============================================================================
// Development fund (rebate)
// =============================================================================

// RebateStatusHandler returns the development-fund status. ClientDataModel
// reads data.status. 0 = inactive.
func (r *Router) RebateStatusHandler(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"code": 0, "status": 0})
}

// RebateHandler returns the development-fund detail. DevelopmentFund.lua does
// arithmetic on data.expires_in, data.total_flow, data.user_flow,
// data.rewards_lower_ratio, data.rewards_upper_ratio — nil crashes. The private
// server returns a valid but inactive payload so the screen renders cleanly.
func (r *Router) RebateHandler(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{
		"code":                   0,
		"status":                 0,
		"expires_in":             86400,
		"user_flow":              0,
		"participate_conditions": 1000,
		"total_flow":             0,
		"start_time":             time.Now().Unix(),
		"reward":                 0,
		"rewards_lower_ratio":    5000,
		"rewards_upper_ratio":    10000,
	})
}

// =============================================================================
// Theme & festival activities
// =============================================================================

// ThemeActivityHandler returns the theme-activity state. Closed via zero
// timestamps (see package doc).
func (r *Router) ThemeActivityHandler(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{
		"code": 0,
		"data": gin.H{
			"id":       0,
			"start_ts": 0,
			"end_ts":   0,
			"list":     []interface{}{},
		},
	})
}

// FestivalActivityHandler returns the spring-festival activity state. Closed
// via zero timestamps.
func (r *Router) FestivalActivityHandler(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{
		"code": 0,
		"data": gin.H{
			"id":       0,
			"start_ts": 0,
			"end_ts":   0,
		},
	})
}

// ThemeStoryHandler returns the theme-story unlock state.
func (r *Router) ThemeStoryHandler(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"code": 0, "list": []interface{}{}})
}

// RecThemeActStoryRwHandler claims a theme-story reward.
func (r *Router) RecThemeActStoryRwHandler(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"code": 0})
}

// FestivalOpenPoolHandler opens a festival pool draw. No live festival.
func (r *Router) FestivalOpenPoolHandler(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"code": 0, "list": []interface{}{}})
}

// FestivalRewardListHandler returns the festival reward list.
func (r *Router) FestivalRewardListHandler(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"code": 0, "list": []interface{}{}})
}

// =============================================================================
// Soccer betting
// =============================================================================

// BetStatusHandler returns the soccer-guessing activity status. open=0 disables
// the feature cleanly without breaking SoccerGuessingModel.
func (r *Router) BetStatusHandler(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"open":    0,
		"live":    0,
		"bet_ids": []interface{}{},
	})
}

// BetListHandler returns the available matches to bet on. No live matches.
func (r *Router) BetListHandler(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"code": 0, "list": []interface{}{}})
}

// BetHistoryHandler returns the user's bet history.
func (r *Router) BetHistoryHandler(c *gin.Context) {
	uidVal, _ := c.Get("uid")
	uid, _ := uidVal.(int64)

	list := []interface{}{}
	if uid > 0 {
		rows, err := r.DB.QueryContext(context.Background(),
			`SELECT bet_id, bet_area, amount, EXTRACT(EPOCH FROM created_at)::BIGINT
			 FROM user_soccer_bets WHERE uid=$1 ORDER BY created_at DESC LIMIT 50`, uid)
		if err == nil {
			defer rows.Close()
			for rows.Next() {
				var betID, betArea int
				var amount int64
				var ts int64
				_ = rows.Scan(&betID, &betArea, &amount, &ts)
				list = append(list, gin.H{
					"bet_id":   betID,
					"bet_area": betArea,
					"amount":   amount,
					"ts":       ts,
				})
			}
		}
	}
	c.JSON(http.StatusOK, gin.H{"code": 0, "list": list})
}

// BetHandler places a soccer bet. The private server records the wager without
// resolving it (no live matches); stakes are not actually deducted.
func (r *Router) BetHandler(c *gin.Context) {
	uidVal, _ := c.Get("uid")
	uid, _ := uidVal.(int64)

	req, _ := c.Get("body")
	params, _ := req.(map[string]interface{})
	betID := intVal(params, "bet_id")
	betArea := intVal(params, "bet_area")
	amount := int64Val(params, "amount")

	if uid > 0 && betID > 0 {
		r.DB.ExecContext(context.Background(),
			`INSERT INTO user_soccer_bets (uid, bet_id, bet_area, amount) VALUES ($1,$2,$3,$4)`,
			uid, betID, betArea, amount)
	}
	c.JSON(http.StatusOK, gin.H{"code": 0})
}

// BetDetailHandler returns the soccer bet detail. SoccerMain.lua:561 does
// pairs(data.bet_area) — nil crashes, so the array is mandatory.
func (r *Router) BetDetailHandler(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{
		"code":     0,
		"bet_area": []interface{}{},
	})
}

// =============================================================================
// Rankings, quiz, survey
// =============================================================================

// RankingListHandler returns the activity ranking list.
func (r *Router) RankingListHandler(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"code": 0, "list": []interface{}{}})
}

// LikeRankingHandler likes a ranking entry. Stateless success on the private
// server.
func (r *Router) LikeRankingHandler(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"code": 0})
}

// AnswerListHandler returns the retention-quiz question list plus the user's
// recorded answers.
func (r *Router) AnswerListHandler(c *gin.Context) {
	uidVal, _ := c.Get("uid")
	uid, _ := uidVal.(int64)

	answered := []interface{}{}
	if uid > 0 {
		rows, err := r.DB.QueryContext(context.Background(),
			`SELECT group_id, question_id FROM user_question_answers WHERE uid=$1`, uid)
		if err == nil {
			defer rows.Close()
			for rows.Next() {
				var groupID, questionID int
				_ = rows.Scan(&groupID, &questionID)
				answered = append(answered, gin.H{
					"group_id":    groupID,
					"question_id": questionID,
				})
			}
		}
	}
	c.JSON(http.StatusOK, gin.H{"code": 0, "list": []interface{}{}, "answered": answered})
}

// AnswerHandler records a quiz answer.
func (r *Router) AnswerHandler(c *gin.Context) {
	uidVal, _ := c.Get("uid")
	uid, _ := uidVal.(int64)

	req, _ := c.Get("body")
	params, _ := req.(map[string]interface{})
	groupID := intVal(params, "group_id")
	questionID := intVal(params, "question_id")

	if uid > 0 && groupID > 0 && questionID > 0 {
		r.DB.ExecContext(context.Background(),
			`INSERT INTO user_question_answers (uid, group_id, question_id) VALUES ($1,$2,$3)
			 ON CONFLICT (uid, group_id, question_id) DO NOTHING`,
			uid, groupID, questionID)
	}
	c.JSON(http.StatusOK, gin.H{"code": 0})
}

// SurveyHandler returns the retention survey metadata.
func (r *Router) SurveyHandler(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"code": 0, "id": 0, "list": []interface{}{}})
}

// =============================================================================
// Game-side social / share / bind rewards
// =============================================================================

// GetBindRwHandler returns the bind-reward remain count. ActivityModel reads
// data.remain.
func (r *Router) GetBindRwHandler(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"code": 0, "remain": 0})
}

// RecBindRwHandler claims the bind reward.
func (r *Router) RecBindRwHandler(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"code": 0})
}

// GetSharePageHandler returns the share-page list. ShareModel reads data.list.
func (r *Router) GetSharePageHandler(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"code": 0, "list": []interface{}{}})
}

// SharePageHandler records a share action.
func (r *Router) SharePageHandler(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"code": 0})
}

// GetAppCommentHandler returns the app-comment timestamps. SdkHelper reads
// data.list (guarded).
func (r *Router) GetAppCommentHandler(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"code": 0, "list": []interface{}{}})
}

// AppCommentHandler records an app comment / rating action.
func (r *Router) AppCommentHandler(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"code": 0})
}

// GetFollowMediaHandler returns the followable media list.
func (r *Router) GetFollowMediaHandler(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"code": 0, "list": []interface{}{}})
}

// FollowMediaHandler records a follow-media action.
func (r *Router) FollowMediaHandler(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"code": 0})
}

// =============================================================================
// Notices
// =============================================================================

// NoticeListHandler returns the notice/announcement list. NoticeModel:159
// indexes self._notices[v.notice_type] — nil notice_type crashes, so an empty
// array is the safe value when the server has no dynamic notices (the client
// ships static notice config).
func (r *Router) NoticeListHandler(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"code": 0, "list": []interface{}{}})
}

// =============================================================================
// Gacha
// =============================================================================

// cardContentType* mirror CARD_CONTENT_TYPE from GachaModel.lua.
// CHARACTER (1) routes to setCharacterShow() which looks up tpl_character[id].
// DECORATION (2) routes to setItemShow() which looks up tpl_props[id].
const cardContentTypeCharacter = 1
const cardContentTypeDecoration = 2

// DrawCardHandler implements the gacha endpoint. The private server keeps
// resources plentiful (every account already owns everything), so the draw
// always succeeds and just returns a random character as the "new" pull.
// Response shape is dictated by GachaModel.lua: each list item MUST carry
// content_id (used as a Lua table key, nil would throw "table index is nil").
//
// content_type MUST be CHARACTER (1), not DECORATION (2): GachaModel.lua:157-160
// sets major_type=ROLE for CHARACTER, routing to GachaResultShow.setCharacterShow()
// which looks up tpl_character[roleID]. Returning skin IDs with DECORATION would
// route to setItemShow() → tpl_props[skinID] → nil → "attempt to index nil" crash.
func (r *Router) DrawCardHandler(c *gin.Context) {
	uidVal, _ := c.Get("uid")
	uidInt, _ := uidVal.(int64)

	roles := ws.AllRoleIDs()
	if len(roles) == 0 {
		c.JSON(http.StatusOK, gin.H{"code": -1, "reason": "no roles available"})
		return
	}

	req, _ := c.Get("body")
	params, _ := req.(map[string]interface{})
	num := intVal(params, "num")
	if num <= 0 {
		num = 1
	}
	if num > 10 {
		num = 10
	}

	list := make([]gin.H, 0, num)
	for i := 0; i < num; i++ {
		role := roles[int(time.Now().UnixNano()+int64(i))%len(roles)]
		list = append(list, gin.H{
			"content_id":   role,
			"content_type": cardContentTypeCharacter,
			"is_new_role":  false,
		})
	}

	// Best-effort persistence: record the drawn role. The private server already
	// unlocks everything, so this is informational only.
	if uidInt > 0 {
		for _, item := range list {
			roleID := item["content_id"].(int32)
			r.DB.ExecContext(context.Background(),
				`INSERT INTO user_items (uid, item_id, count) VALUES ($1, $2, 1)
				 ON CONFLICT (uid, item_id) DO UPDATE SET count=user_items.count+1`,
				uidInt, roleID)
		}
	}
	c.JSON(http.StatusOK, gin.H{"code": 0, "list": list})
}

// DrawListHandler returns the gacha pool list. The private server unlocks all
// cosmetics, so the pool is a flat list of every skin with equal weight —
// purely for the client's GachaModel UI to render a non-empty pool.
func (r *Router) DrawListHandler(c *gin.Context) {
	skins := ws.AllSkinIDs()
	items := make([]gin.H, 0, len(skins))
	for _, id := range skins {
		items = append(items, gin.H{
			"content_id":   id,
			"content_type": cardContentTypeDecoration,
			"weight":       1,
		})
	}
	c.JSON(http.StatusOK, gin.H{
		"code": 0,
		"list": []gin.H{{
			"pool_id":           10001, // must be a valid tpl_card_pool key (10001-10007, 20002-20016, 30001)
			"pool_name":         "all_skins",
			"sort":              1,
			"character_rate":    10000,
			"cosmetic_rate":     10000,
			"item_rate":         10000,
			"character_skin_id": 0,
			"end_ts":            0,
			"list":              items,
		}},
	})
}

// =============================================================================
// Sign-in helpers
// =============================================================================

// loadSignIn returns the sign-in state for one cycle. Fresh accounts get a
// sensible default (signable now, 7-day cycle).
func (r *Router) loadSignIn(uid int64, signType, activityID int) gin.H {
	var (
		daysRaw     []byte
		nextSignTs  int64
		missSignCnt int
		cycleTs     int64
	)
	if uid > 0 {
		_ = r.DB.QueryRowContext(context.Background(),
			`SELECT days::TEXT, next_sign_ts, miss_sign_cnt, cycle_ts
			 FROM user_signin WHERE uid=$1 AND signin_type=$2 AND activity_id=$3`,
			uid, signType, activityID).Scan(&daysRaw, &nextSignTs, &missSignCnt, &cycleTs)
	}
	if cycleTs == 0 {
		cycleTs = time.Now().Add(7 * 24 * time.Hour).Unix()
	}
	if nextSignTs == 0 {
		nextSignTs = time.Now().Unix()
	}
	return gin.H{
		"days":          jsonRawList(daysRaw),
		"next_sign_ts":  nextSignTs,
		"miss_sign_cnt": missSignCnt,
		"cycle_ts":      cycleTs,
	}
}

// monthlyCardDailyRewards mirrors tpl_constdata.Monthly_Card_Daily_Rewards
// ({10200001,30,10100001,175000}). Delivered via ext_item_list on seven-day
// sign-in for users with an active monthly card. Read from gamedata so any
// config change is picked up without a server rebuild.
func monthlyCardDailyRewards() []gamedata.Reward {
	if d := gamedata.ConstData(); d != nil {
		if arr, ok := d["Monthly_Card_Daily_Rewards"].([]interface{}); ok && len(arr) > 0 {
			if rws := gamedata.ParseFlatRewards(arr); len(rws) > 0 {
				return rws
			}
		}
	}
	// Fallback matching the shipped config ({10200001,30,10100001,175000}).
	return []gamedata.Reward{
		{MajorType: 1, ItemID: 10200001, Num: 30},
		{MajorType: 1, ItemID: 10100001, Num: 175000},
	}
}

// monthlyCardDailyRewardList returns the reward list shaped as item_list for
// HTTP responses (gin.H with major_type/item_id/num keys).
func monthlyCardDailyRewardList() []gin.H {
	rws := monthlyCardDailyRewards()
	out := make([]gin.H, 0, len(rws))
	for _, rw := range rws {
		out = append(out, gin.H{
			"major_type": rw.MajorType,
			"item_id":    rw.ItemID,
			"num":        rw.Num,
		})
	}
	return out
}

// performSignIn records a sign-in: appends the next cycle-day (1..7) to days
// and pushes next_sign_ts forward by 24h. For seven-day sign-in, users with an
// active monthly card also receive the monthly-card daily reward via
// ext_item_list (SignInModel:sendSignIn -> ShopMonthlyCardReward UI).
func (r *Router) performSignIn(c *gin.Context, signType, activityID int) {
	uidVal, _ := c.Get("uid")
	uid, _ := uidVal.(int64)

	now := time.Now().Unix()
	nextSign := now + 24*60*60

	if uid > 0 {
		// day = current signed count + 1, wrapped into the 1..7 cycle window.
		var dayCount int
		r.DB.QueryRowContext(context.Background(),
			`SELECT COALESCE(jsonb_array_length(days), 0) FROM user_signin
		 WHERE uid=$1 AND signin_type=$2 AND activity_id=$3`,
			uid, signType, activityID).Scan(&dayCount)
		day := dayCount%7 + 1

		r.DB.ExecContext(context.Background(),
			`INSERT INTO user_signin (uid, signin_type, activity_id, days, next_sign_ts, cycle_ts, updated_at)
			 VALUES ($1, $2, $3, to_jsonb(ARRAY[$4::int]), $5, $6, NOW())
			 ON CONFLICT (uid, signin_type, activity_id) DO UPDATE
			 SET days = user_signin.days || to_jsonb($4::int),
			     next_sign_ts = $5,
			     updated_at = NOW()`,
			uid, signType, activityID, day, nextSign, now+int64(7*24*3600))
	}

	// Re-read the updated sign-in state for the response.
	var daysRaw []byte
	var missSignCnt int
	if uid > 0 {
		r.DB.QueryRowContext(context.Background(),
			`SELECT days::TEXT, COALESCE(miss_sign_cnt, 0) FROM user_signin
		 WHERE uid=$1 AND signin_type=$2 AND activity_id=$3`,
			uid, signType, activityID).Scan(&daysRaw, &missSignCnt)
	}

	resp := gin.H{
		"code":            0,
		"days":            jsonRawList(daysRaw),
		"next_sign_ts":    nextSign,
		"miss_sign_cnt":   missSignCnt,
		"item_list":       []interface{}{},
		"total_item_list": []interface{}{},
	}

	// Monthly card daily reward: seven-day sign-in for users with an active card.
	if signType == model.SignInTypeSevenDay && uid > 0 {
		var monthlyCardExp int64
		r.DB.QueryRowContext(context.Background(),
			`SELECT COALESCE(monthly_card_exp, 0) FROM users WHERE uid=$1`, uid).Scan(&monthlyCardExp)
		if monthlyCardExp > now {
			resp["ext_item_list"] = monthlyCardDailyRewardList()
			// grantRewards persists gold to users.gold and other items (e.g.
			// diamonds 10200001) to user_items, then schedules pushGoldUpdate.
			r.grantRewards(uid, monthlyCardDailyRewards())
		}
	}

	c.JSON(http.StatusOK, resp)
}
