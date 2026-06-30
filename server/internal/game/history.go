package game

import (
	"context"
	"encoding/json"

	"google.golang.org/protobuf/proto"

	"poker-fate-server/internal/proto/gen"
)

// handHistoryEntry is the per-hand snapshot persisted to user_hand_history.
type handHistoryEntry struct {
	GameID string
	HandID int
	Hands  []*gen.PlayerHands
	Board  []uint32
}

// persistHandHistory writes one row per participant into user_hand_history.
// Called from finishRound after the pot is awarded.
func (m *Manager) persistHandHistory(room *Room, entry handHistoryEntry) {
	if m.db == nil || entry.GameID == "" {
		return
	}
	handsJSON, _ := json.Marshal(entry.Hands)
	boardJSON, _ := json.Marshal(entry.Board)
	data := map[string]interface{}{
		"hands": json.RawMessage(handsJSON),
		"board": json.RawMessage(boardJSON),
	}
	dataJSON, _ := json.Marshal(data)

	for _, p := range room.Players {
		if p.IsAI {
			continue
		}
		_, _ = m.db.ExecContext(context.Background(),
			`INSERT INTO user_hand_history (uid, gameid, hand_id, game_type, room_id, hands_data)
			 VALUES ($1,$2,$3,$4,$5,$6::jsonb)
			 ON CONFLICT (uid, gameid) DO UPDATE SET hands_data=EXCLUDED.hands_data`,
			p.UID, entry.GameID, entry.HandID, 1, int(room.ID), string(dataJSON))
	}
}

// loadHandHistory loads a single hand by (uid, gameid) for GetHandsListREQ.
func (m *Manager) loadHandHistory(uid int64, gameID string) *gen.HandsInfo {
	if m.db == nil || uid == 0 || gameID == "" {
		return nil
	}
	var handID int
	var dataRaw []byte
	err := m.db.QueryRowContext(context.Background(),
		`SELECT hand_id, hands_data::TEXT FROM user_hand_history WHERE uid=$1 AND gameid=$2`,
		uid, gameID).Scan(&handID, &dataRaw)
	if err != nil {
		return nil
	}

	var data struct {
		Hands []*gen.PlayerHands `json:"hands"`
		Board []uint32           `json:"board"`
	}
	_ = json.Unmarshal(dataRaw, &data)
	return &gen.HandsInfo{
		Id:     proto.Int32(int32(handID)),
		Hands:  data.Hands,
		Board:  data.Board,
		Gameid: proto.String(gameID),
	}
}

// loadLatestHandHistory returns the most recent hand for a uid (id=0 case).
func (m *Manager) loadLatestHandHistory(uid int64) *gen.HandsInfo {
	if m.db == nil || uid == 0 {
		return nil
	}
	var gameID string
	err := m.db.QueryRowContext(context.Background(),
		`SELECT gameid FROM user_hand_history WHERE uid=$1 ORDER BY created_at DESC LIMIT 1`,
		uid).Scan(&gameID)
	if err != nil {
		return nil
	}
	return m.loadHandHistory(uid, gameID)
}

// buildHisDetail constructs TableHisDetail for disconnect/reconnect.
// Only populated when a hand is in progress (Stage >= Preflop).
func (m *Manager) buildHisDetail(room *Room) *gen.TableHisDetail {
	table := room.Table
	if table.Stage < StagePreflop {
		return nil
	}

	// pool_rd: current contributions per player.
	var poolUsers []*gen.PoolUser
	for _, p := range room.Players {
		if p.TotalBet > 0 {
			poolUsers = append(poolUsers, &gen.PoolUser{
				Uid:    proto.Int64(p.UID),
				Chips:  proto.Int64(p.TotalBet),
				Name:   proto.String(p.Name),
				Seatid: proto.Int32(int32(p.SeatID)),
			})
		}
	}
	var poolRds []*gen.PoolRD
	if len(poolUsers) > 0 {
		poolRds = append(poolRds, &gen.PoolRD{
			Poolid: proto.Int32(0),
			Pot:    proto.Int64(table.Pot),
			Users:  poolUsers,
		})
	}

	// round_rd: per-stage action history.
	var roundRds []*gen.RoundRD
	for st := int32(1); st <= 4; st++ {
		actions := table.RoundActions[st]
		if len(actions) == 0 && table.RoundPots[st] == 0 {
			continue
		}
		roundRds = append(roundRds, &gen.RoundRD{
			Stage:  proto.Int32(st),
			Action: actions,
			Pot:    proto.Int64(table.RoundPots[st]),
		})
	}

	detail := &gen.TableHisDetail{
		PoolRd:  poolRds,
		RoundRd: roundRds,
	}
	if table.Stage == StageShow {
		detail.ShowRoundRd = m.buildShowRoundRD(room)
	}
	return detail
}

// buildShowRoundRD constructs the show-stage detail for his_detail.
func (m *Manager) buildShowRoundRD(room *Room) *gen.ShowRoundRD {
	var items []*gen.ShowRoundItem
	for _, p := range room.Players {
		if p.Folded {
			continue
		}
		cards := make([]int32, len(p.Cards))
		for i, c := range p.Cards {
			cards[i] = int32(c)
		}
		showType := make([]int32, len(p.Cards))
		for i := range showType {
			showType[i] = 1 // contested show
		}
		items = append(items, &gen.ShowRoundItem{
			Seatid:       proto.Int32(int32(p.SeatID)),
			Uid:          proto.Int64(p.UID),
			HandCards:    cards,
			ShowTypeList: showType,
			Name:         proto.String(p.Name),
		})
	}
	return &gen.ShowRoundRD{Pot: proto.Int64(room.Table.Pot), Items: items}
}
