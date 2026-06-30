package httpapi

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
)

// Player module handlers. Backed by the users table and its 002 extensions
// (declaration, monthly_card_exp, assoc_pwd, delete_time, stove_guid, ...).
//
// Response shapes mirror what the client Lua models index directly:
//   - /player/payInfo  -> ShopModel:312 reads data.data.monthly_card_exp
//   - /player/gameData -> InformationMainNew:524 reads data.data.game_type
//   - /player/getAssocPwd -> SettingTransferID reads data.assoc_pwd/_expired
// Both payInfo and gameData wrap their payload under data.data; the other
// endpoints put fields at the top level.

// PlayerValidHandler confirms the session is valid. AuthMiddleware already
// verified the token, so a plain success is the official behaviour.
func (r *Router) PlayerValidHandler(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"code": 0})
}

// PlayerReportGuideHandler persists the client's completed guide id so the
// guide manager does not replay tutorials on next login. The client
// (GuideManager.lua:80) sends {guide_id: int} for system guides; legacy
// {guide_list: [...]} / {guide_step: int} are still accepted for compatibility.
func (r *Router) PlayerReportGuideHandler(c *gin.Context) {
	uidVal, _ := c.Get("uid")
	uid, _ := uidVal.(int64)

	req, _ := c.Get("body")
	params, _ := req.(map[string]interface{})

	guideStep := intVal(params, "guide_step")
	guideID := intVal(params, "guide_id")
	guideList, _ := params["guide_list"]

	if uid > 0 {
		if guideID > 0 {
			existing := r.loadUserSetting(uid, "guide_list")
			var list []interface{}
			if arr, ok := existing.([]interface{}); ok {
				list = arr
			}
			found := false
			for _, v := range list {
				if n, ok := v.(float64); ok && int(n) == guideID {
					found = true
					break
				}
			}
			if !found {
				list = append(list, guideID)
				r.saveUserSetting(uid, "guide_list", list)
			}
		} else if guideList != nil {
			r.saveUserSetting(uid, "guide_list", guideList)
		}
		if guideStep > 0 {
			r.DB.ExecContext(context.Background(),
				`UPDATE users SET newer_guide_step=$1 WHERE uid=$2`, guideStep, uid)
		}
	}
	c.JSON(http.StatusOK, gin.H{"code": 0})
}

// PlayerGuideListHandler returns the saved guide-step list. GuideManager
// iterates data.list with `or {}`, so an empty array is safe.
func (r *Router) PlayerGuideListHandler(c *gin.Context) {
	uidVal, _ := c.Get("uid")
	uid, _ := uidVal.(int64)

	list := r.loadUserSetting(uid, "guide_list")
	if list == nil {
		list = []interface{}{}
	}
	c.JSON(http.StatusOK, gin.H{"code": 0, "list": list})
}

// PlayerDeleteAccountHandler marks the account for deletion. The official
// server sets a 7-day grace window; the client reads is_del to switch UI.
func (r *Router) PlayerDeleteAccountHandler(c *gin.Context) {
	uidVal, _ := c.Get("uid")
	uid, _ := uidVal.(int64)

	if uid > 0 {
		r.DB.ExecContext(context.Background(),
			`UPDATE users SET is_deleted=true, delete_time=$1 WHERE uid=$2`,
			time.Now().Add(7*24*time.Hour).Unix(), uid)
	}
	c.JSON(http.StatusOK, gin.H{"code": 0})
}

// PlayerCancelDeleteHandler clears the deletion flag.
func (r *Router) PlayerCancelDeleteHandler(c *gin.Context) {
	uidVal, _ := c.Get("uid")
	uid, _ := uidVal.(int64)

	if uid > 0 {
		r.DB.ExecContext(context.Background(),
			`UPDATE users SET is_deleted=false, delete_time=0 WHERE uid=$1`, uid)
	}
	c.JSON(http.StatusOK, gin.H{"code": 0})
}

// PlayerBindStoveHandler binds a Stove (Longteng) account. The private server
// just records the provided guid; no real third-party validation occurs.
func (r *Router) PlayerBindStoveHandler(c *gin.Context) {
	uidVal, _ := c.Get("uid")
	uid, _ := uidVal.(int64)

	req, _ := c.Get("body")
	params, _ := req.(map[string]interface{})
	stoveGUID := intVal(params, "stove_guid")

	if uid > 0 && stoveGUID > 0 {
		r.DB.ExecContext(context.Background(),
			`UPDATE users SET stove_guid=$1 WHERE uid=$2`, stoveGUID, uid)
	}
	c.JSON(http.StatusOK, gin.H{"code": 0})
}

// PlayerBindEmailHandler binds an email to the current account.
func (r *Router) PlayerBindEmailHandler(c *gin.Context) {
	uidVal, _ := c.Get("uid")
	uid, _ := uidVal.(int64)

	req, _ := c.Get("body")
	params, _ := req.(map[string]interface{})
	email := strVal(params, "email")

	if uid > 0 && email != "" {
		r.DB.ExecContext(context.Background(),
			`UPDATE users SET email=$1 WHERE uid=$2`, email, uid)
	}
	c.JSON(http.StatusOK, gin.H{"code": 0})
}

// PlayerPayInfoHandler returns the monthly-card expiry. ShopModel:312 reads
// data.data.monthly_card_exp without a guard, so the nested object is mandatory.
// The expiry is persisted in users.monthly_card_exp on monthly-card purchase.
func (r *Router) PlayerPayInfoHandler(c *gin.Context) {
	uidVal, _ := c.Get("uid")
	uid, _ := uidVal.(int64)

	var monthlyCardExp int64
	if uid > 0 {
		r.DB.QueryRowContext(context.Background(),
			`SELECT COALESCE(monthly_card_exp, 0) FROM users WHERE uid=$1`, uid).Scan(&monthlyCardExp)
	}
	c.JSON(http.StatusOK, gin.H{
		"code": 0,
		"data": gin.H{
			"monthly_card_exp": monthlyCardExp,
		},
	})
}

// PlayerGameDataHandler returns per-game-type statistics. InformationMainNew:524
// reads data.data.game_type directly, so the payload is nested under data.data.
func (r *Router) PlayerGameDataHandler(c *gin.Context) {
	uidVal, _ := c.Get("uid")
	uid, _ := uidVal.(int64)

	req, _ := c.Get("body")
	params, _ := req.(map[string]interface{})
	gameType := intVal(params, "game_type")
	if gameType <= 0 {
		gameType = 1
	}

	data := r.loadGameStat(uid, gameType)
	c.JSON(http.StatusOK, gin.H{
		"code": 0,
		"data": data,
	})
}

// PlayerTourRecordHandler returns the tournament placement history.
// InformationMainNew iterates resp.list directly.
func (r *Router) PlayerTourRecordHandler(c *gin.Context) {
	uidVal, _ := c.Get("uid")
	uid, _ := uidVal.(int64)

	rows, err := r.DB.QueryContext(context.Background(),
		`SELECT id, game_type, placement, EXTRACT(EPOCH FROM created_at)::BIGINT
		 FROM user_tour_records WHERE uid=$1 ORDER BY created_at DESC LIMIT 50`, uid)
	if err != nil {
		c.JSON(http.StatusOK, gin.H{"code": 0, "list": []interface{}{}})
		return
	}
	defer rows.Close()

	list := make([]gin.H, 0)
	for rows.Next() {
		var id int64
		var gt, placement int
		var ts int64
		_ = rows.Scan(&id, &gt, &placement, &ts)
		list = append(list, gin.H{
			"id":        id,
			"game_type": gt,
			"placement": placement,
			"create_ts": ts,
			"tour_name": "",
		})
	}
	c.JSON(http.StatusOK, gin.H{"code": 0, "list": list})
}

// PlayerUpdateNicknameHandler updates the player display name. Accepts both
// `nickname` and `name` keys (the client uses either depending on flow).
func (r *Router) PlayerUpdateNicknameHandler(c *gin.Context) {
	uidVal, _ := c.Get("uid")
	uid, _ := uidVal.(int64)

	req, _ := c.Get("body")
	params, _ := req.(map[string]interface{})
	nickname := strVal(params, "nickname")
	if nickname == "" {
		nickname = strVal(params, "name")
	}
	if uid > 0 && nickname != "" {
		r.DB.ExecContext(context.Background(),
			`UPDATE users SET name=$1 WHERE uid=$2`, nickname, uid)
	}
	c.JSON(http.StatusOK, gin.H{"code": 0})
}

// PlayerUpdateDeclarationHandler updates the player's signature/declaration.
func (r *Router) PlayerUpdateDeclarationHandler(c *gin.Context) {
	uidVal, _ := c.Get("uid")
	uid, _ := uidVal.(int64)

	req, _ := c.Get("body")
	params, _ := req.(map[string]interface{})
	declaration := strVal(params, "declaration")

	if uid > 0 {
		r.DB.ExecContext(context.Background(),
			`UPDATE users SET declaration=$1 WHERE uid=$2`, declaration, uid)
	}
	c.JSON(http.StatusOK, gin.H{"code": 0})
}

// PlayerGenAssocPwdHandler generates a transfer/association password. The
// password is valid for 10 minutes on the official server.
func (r *Router) PlayerGenAssocPwdHandler(c *gin.Context) {
	uidVal, _ := c.Get("uid")
	uid, _ := uidVal.(int64)

	pwd := genAssocPwd()
	expired := time.Now().Add(10 * time.Minute).Unix()
	if uid > 0 {
		r.DB.ExecContext(context.Background(),
			`UPDATE users SET assoc_pwd=$1, assoc_pwd_expired=$2, assoc_err_num=0 WHERE uid=$3`,
			pwd, expired, uid)
	}
	c.JSON(http.StatusOK, gin.H{
		"code":              0,
		"assoc_pwd":         pwd,
		"assoc_pwd_expired": expired,
	})
}

// PlayerGetAssocPwdHandler returns the current association password and its
// expiry. SettingTransferID reads data.assoc_pwd / data.assoc_pwd_expired.
func (r *Router) PlayerGetAssocPwdHandler(c *gin.Context) {
	uidVal, _ := c.Get("uid")
	uid, _ := uidVal.(int64)

	var pwd string
	var expired int64
	if uid > 0 {
		r.DB.QueryRowContext(context.Background(),
			`SELECT assoc_pwd, assoc_pwd_expired FROM users WHERE uid=$1`, uid).Scan(&pwd, &expired)
	}
	c.JSON(http.StatusOK, gin.H{
		"code":              0,
		"assoc_pwd":         pwd,
		"assoc_pwd_expired": expired,
	})
}

// PlayerAssocUserHandler associates the current account with another via the
// transfer password. The private server accepts any well-formed request.
func (r *Router) PlayerAssocUserHandler(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"code": 0})
}

// NewbieNicknameHandler returns the empty nickname during the newbie naming
// flow so the client shows the input dialog.
func (r *Router) NewbieNicknameHandler(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"code": 0, "nickname": ""})
}

// --- helpers ---

// loadGameStat builds the gameData payload for one game type. Missing rows
// yield a zeroed stats object, which the client renders as "no games played".
func (r *Router) loadGameStat(uid int64, gameType int) gin.H {
	var (
		playTimes, winPlayTimes, firePower int
		poolEntry, addBefore, threeBet     int
		showHand, active, cBete            int
		profit                             int64
		maxProfitCards, bestCards          []byte
	)
	if uid > 0 {
		_ = r.DB.QueryRowContext(context.Background(),
			`SELECT play_times, win_play_times, profit, fire_power,
			        pool_entry_rate, add_before_flipping_rate, three_bet_rate,
			        show_hand_rate, active_rate, c_bete_rate,
			        max_profit_cards::TEXT, best_cards::TEXT
			 FROM user_game_stats WHERE uid=$1 AND game_type=$2`, uid, gameType).
			Scan(&playTimes, &winPlayTimes, &profit, &firePower,
				&poolEntry, &addBefore, &threeBet,
				&showHand, &active, &cBete,
				&maxProfitCards, &bestCards)
	}
	return gin.H{
		"game_type":                gameType,
		"play_times":               playTimes,
		"win_play_times":           winPlayTimes,
		"profit":                   profit,
		"fire_power":               firePower,
		"max_profit_cards":         jsonRawList(maxProfitCards),
		"best_cards":               jsonRawList(bestCards),
		"pool_entry_rate":          poolEntry,
		"add_before_flipping_rate": addBefore,
		"three_bet_rate":           threeBet,
		"show_hand_rate":           showHand,
		"active_rate":              active,
		"c_bete_rate":              cBete,
	}
}

// saveUserSetting upserts a JSON-serialisable value into user_settings.
func (r *Router) saveUserSetting(uid int64, key string, value interface{}) {
	raw, err := jsonMarshal(value)
	if err != nil {
		return
	}
	r.DB.ExecContext(context.Background(),
		`INSERT INTO user_settings (uid, cfg_key, cfg_value, updated_at)
		 VALUES ($1, $2, $3::jsonb, NOW())
		 ON CONFLICT (uid, cfg_key) DO UPDATE SET cfg_value=$3::jsonb, updated_at=NOW()`,
		uid, key, raw)
}

// loadUserSetting returns the stored JSON value for a key, or nil if absent.
func (r *Router) loadUserSetting(uid int64, key string) interface{} {
	var raw []byte
	if uid == 0 {
		return nil
	}
	err := r.DB.QueryRowContext(context.Background(),
		`SELECT cfg_value::TEXT FROM user_settings WHERE uid=$1 AND cfg_key=$2`, uid, key).Scan(&raw)
	if err != nil || len(raw) == 0 {
		return nil
	}
	var v interface{}
	_ = jsonUnmarshal(raw, &v)
	return v
}

// genAssocPwd returns an 8-char hex password.
func genAssocPwd() string {
	b := make([]byte, 4)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}
