package httpapi

import (
	"context"
	"net/http"

	"github.com/gin-gonic/gin"
)

// Collection-card module handlers. Replays are persisted in
// user_collected_cards (migration 002). Each row stores both the summary fields
// the list endpoints return and the full replay payload that /collCard/detail
// hands to PKRecordData:create.
//
// Summary entry shape consumed by PlayerModel / IngameReplay:
//   {id, gameid, game_type, profit, hand_type, cards, small_blind, big_blind,
//    ante, tour_name, game_start_time, collected}

// CollCardListHandler returns the player's collected-replay list.
func (r *Router) CollCardListHandler(c *gin.Context) {
	uidVal, _ := c.Get("uid")
	uid, _ := uidVal.(int64)

	list := r.loadCollCards(uid, 0)
	c.JSON(http.StatusOK, gin.H{"code": 0, "list": list})
}

// CollCardRecentlyListHandler returns the most recently collected replays.
func (r *Router) CollCardRecentlyListHandler(c *gin.Context) {
	uidVal, _ := c.Get("uid")
	uid, _ := uidVal.(int64)

	list := r.loadCollCards(uid, 20)
	c.JSON(http.StatusOK, gin.H{"code": 0, "list": list})
}

// CollCardNumHandler returns the current collection count and the storage cap.
// PlayerModel reads data.coll_num and data.limit_num directly.
func (r *Router) CollCardNumHandler(c *gin.Context) {
	uidVal, _ := c.Get("uid")
	uid, _ := uidVal.(int64)

	var collNum int
	if uid > 0 {
		r.DB.QueryRowContext(context.Background(),
			`SELECT COUNT(*) FROM user_collected_cards WHERE uid=$1`, uid).Scan(&collNum)
	}
	c.JSON(http.StatusOK, gin.H{
		"code":      0,
		"coll_num":  collNum,
		"limit_num": 50,
	})
}

// CollCardDetailHandler returns the full replay payload for one card.
// IngameReplay passes d.data straight to PKRecordData:create, so an empty
// object is the safe fallback when the id is unknown.
func (r *Router) CollCardDetailHandler(c *gin.Context) {
	uidVal, _ := c.Get("uid")
	uid, _ := uidVal.(int64)

	req, _ := c.Get("body")
	params, _ := req.(map[string]interface{})
	gameID := strVal(params, "gameid")
	if gameID == "" {
		gameID = strVal(params, "id")
	}

	data := gin.H{}
	if uid > 0 && gameID != "" {
		var raw []byte
		err := r.DB.QueryRowContext(context.Background(),
			`SELECT replay_data::TEXT FROM user_collected_cards WHERE uid=$1 AND gameid=$2`,
			uid, gameID).Scan(&raw)
		if err == nil && len(raw) > 0 {
			data = jsonRawMap(raw)
		}
	}
	c.JSON(http.StatusOK, gin.H{"code": 0, "data": data})
}

// CollCardUpdateHandler upserts a replay into the collection. The client sends
// the full summary + replay payload after a game ends; the server stores it and
// honours the `collected` flag (toggle save/forget).
func (r *Router) CollCardUpdateHandler(c *gin.Context) {
	uidVal, _ := c.Get("uid")
	uid, _ := uidVal.(int64)

	req, _ := c.Get("body")
	params, _ := req.(map[string]interface{})

	gameID := strVal(params, "gameid")
	if gameID == "" {
		gameID = strVal(params, "id")
	}
	if uid == 0 || gameID == "" {
		c.JSON(http.StatusOK, gin.H{"code": 0})
		return
	}

	gameType := intVal(params, "game_type")
	profit := int64Val(params, "profit")
	handType := intVal(params, "hand_type")
	smallBlind := intVal(params, "small_blind")
	bigBlind := intVal(params, "big_blind")
	ante := intVal(params, "ante")
	tourName := strVal(params, "tour_name")
	gameStartTime := int64Val(params, "game_start_time")
	collected := true
	if v, ok := params["collected"]; ok {
		if b, isBool := v.(bool); isBool {
			collected = b
		}
	}

	cardsRaw, _ := jsonMarshal(params["cards"])
	replayRaw, _ := jsonMarshal(params["replay_data"])
	if replayRaw == nil {
		replayRaw = []byte("{}")
	}
	if cardsRaw == nil {
		cardsRaw = []byte("[]")
	}

	r.DB.ExecContext(context.Background(),
		`INSERT INTO user_collected_cards
		   (uid, gameid, game_type, profit, hand_type, cards, small_blind,
		    big_blind, ante, tour_name, game_start_time, replay_data, collected)
		 VALUES ($1,$2,$3,$4,$5,$6::jsonb,$7,$8,$9,$10,$11,$12::jsonb,$13)
		 ON CONFLICT (uid, gameid) DO UPDATE SET
		   game_type=EXCLUDED.game_type, profit=EXCLUDED.profit,
		   hand_type=EXCLUDED.hand_type, cards=EXCLUDED.cards,
		   small_blind=EXCLUDED.small_blind, big_blind=EXCLUDED.big_blind,
		   ante=EXCLUDED.ante, tour_name=EXCLUDED.tour_name,
		   game_start_time=EXCLUDED.game_start_time,
		   replay_data=EXCLUDED.replay_data, collected=EXCLUDED.collected`,
		uid, gameID, gameType, profit, handType, string(cardsRaw),
		smallBlind, bigBlind, ante, tourName, gameStartTime, string(replayRaw), collected)

	c.JSON(http.StatusOK, gin.H{"code": 0})
}

// --- helpers ---

// loadCollCards returns the summary list, optionally limited to the most recent
// `limit` entries. Zero limit means all.
func (r *Router) loadCollCards(uid int64, limit int) []gin.H {
	if uid == 0 {
		return []gin.H{}
	}
	q := `SELECT id, gameid, game_type, profit, hand_type, cards::TEXT,
	         small_blind, big_blind, ante, tour_name, game_start_time, collected
	      FROM user_collected_cards WHERE uid=$1 ORDER BY created_at DESC`
	args := []interface{}{uid}
	if limit > 0 {
		q += " LIMIT $2"
		args = append(args, limit)
	}

	rows, err := r.DB.QueryContext(context.Background(), q, args...)
	if err != nil {
		return []gin.H{}
	}
	defer rows.Close()

	list := make([]gin.H, 0)
	for rows.Next() {
		var id int64
		var profit, gameStartTime int64
		var gameType, handType, smallBlind, bigBlind, ante int
		var gameID, tourName, cardsRaw string
		var collected bool
		_ = rows.Scan(&id, &gameID, &gameType, &profit, &handType, &cardsRaw,
			&smallBlind, &bigBlind, &ante, &tourName, &gameStartTime, &collected)

		collectedVal := 0
		if collected {
			collectedVal = 1
		}
		list = append(list, gin.H{
			"id":              id,
			"gameid":          gameID,
			"game_type":       gameType,
			"profit":          profit,
			"hand_type":       handType,
			"cards":           jsonRawList([]byte(cardsRaw)),
			"small_blind":     smallBlind,
			"big_blind":       bigBlind,
			"ante":            ante,
			"tour_name":       tourName,
			"game_start_time": gameStartTime,
			"collected":       collectedVal,
		})
	}
	return list
}

// int64Val reads a numeric field that may arrive as float64 (JSON) or int.
func int64Val(m map[string]interface{}, key string) int64 {
	switch v := m[key].(type) {
	case float64:
		return int64(v)
	case int:
		return int64(v)
	case int64:
		return v
	default:
		return 0
	}
}
