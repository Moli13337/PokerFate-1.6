package game

import (
	"google.golang.org/protobuf/proto"

	"poker-fate-server/internal/proto/gen"
)

// broadcast sends a packet to every player (and spectator) in the room.
func (m *Manager) broadcast(room *Room, packType string, body []byte) {
	var uids []int64
	room.mu.Lock()
	for _, p := range room.Players {
		uids = append(uids, p.UID)
	}
	for uid := range room.Spectators {
		uids = append(uids, uid)
	}
	room.mu.Unlock()
	m.wsSrv.BroadcastToUIDs(uids, packType, room.ID, body)
}

// broadcastTo sends a packet to a specific uid list within the room.
func (m *Manager) broadcastTo(room *Room, uids []int64, packType string, body []byte) {
	m.wsSrv.BroadcastToUIDs(uids, packType, room.ID, body)
}

// communityToInt32 converts the byte-card community slice to proto int32 form.
func (m *Manager) communityToInt32(cards []byte) []int32 {
	result := make([]int32, len(cards))
	for i, c := range cards {
		result[i] = int32(c)
	}
	return result
}

func (m *Manager) sendPacket(room *Room, packType string, msg proto.Message) {
	body, _ := proto.Marshal(msg)
	m.broadcast(room, packType, body)
}

// broadcastSitDown notifies the room that a player sat down.
func (m *Manager) broadcastSitDown(room *Room, player *Player) {
	brc := &gen.SitDownBRC{
		Status: &gen.SeatStatus{
			Seatid:    proto.Int32(int32(player.SeatID)),
			Player:    buildUserBrief(player),
			HandChips: proto.Int64(player.Gold),
		},
	}
	m.sendPacket(room, "pb.SitDownBRC", brc)
}

// buildUserBrief constructs the public profile for a player.
func buildUserBrief(p *Player) *gen.UserBrief {
	return &gen.UserBrief{
		Uid:    proto.Int64(p.UID),
		Name:   proto.String(p.Name),
		Avatar: proto.Int32(int32(p.Avatar)),
		RoleId: proto.Int32(int32(p.RoleID)),
		SkinId: proto.Int32(int32(p.SkinID)),
	}
}

// broadcastDealerInfo sends DealerInfoRSP at the start of each hand.
func (m *Manager) broadcastDealerInfo(room *Room) {
	table := room.Table
	var startInfo []*gen.StartInfo
	for seatID, p := range room.Players {
		startInfo = append(startInfo, &gen.StartInfo{
			Seatid: proto.Int32(int32(seatID)),
			Chips:  proto.Int64(p.Gold),
		})
	}
	rsp := &gen.DealerInfoRSP{
		Dealer:     proto.Int32(int32(table.Dealer)),
		SmallBlind: proto.Int32(int32(table.SBSeat)),
		BigBlind:   proto.Int32(int32(table.BBSeat)),
		StartInfo:  startInfo,
		Gameid:     proto.String(table.GameID),
	}
	m.sendPacket(room, "pb.DealerInfoRSP", rsp)
}

// broadcastAnte sends AnteBRC when ante is charged.
func (m *Manager) broadcastAnte(room *Room, infos []*gen.AnteInfo) {
	m.sendPacket(room, "pb.AnteBRC", &gen.AnteBRC{Info: infos})
}

// broadcastHandCard sends a player's hole cards privately.
func (m *Manager) broadcastHandCard(room *Room, p *Player) {
	rsp := &gen.HandCardRSP{
		Card1: proto.Int32(int32(p.Cards[0])),
		Card2: proto.Int32(int32(p.Cards[1])),
	}
	body, _ := proto.Marshal(rsp)
	m.wsSrv.BroadcastToUIDs([]int64{p.UID}, "pb.HandCardRSP", room.ID, body)
}

// broadcastRoundStart sends RoundStartBRC for a new stage.
func (m *Manager) broadcastRoundStart(room *Room, stage Stage) {
	m.sendPacket(room, "pb.RoundStartBRC", &gen.RoundStartBRC{
		Stage: proto.Int32(stageProto(stage)),
		Board: m.communityToInt32(room.Table.Community),
	})
}

// broadcastActionNotify tells the room whose turn it is and the bet bounds.
func (m *Manager) broadcastActionNotify(room *Room, p *Player, callNeed, minChipin, maxChipin int64, thinkTime int32) {
	rsp := &gen.ActionNotifyBRC{
		Seatid:        proto.Int32(int32(p.SeatID)),
		CallNeedChips: proto.Int64(callNeed),
		MinChipin:     proto.Int64(minChipin),
		MaxChipin:     proto.Int64(maxChipin),
		ThinkTime:     proto.Int32(thinkTime),
	}
	m.sendPacket(room, "pb.ActionNotifyBRC", rsp)
}

// broadcastAction sends ActionBRC after a player acts.
func (m *Manager) broadcastAction(room *Room, p *Player, actionType int32, chips int64) {
	rsp := &gen.ActionBRC{
		Seatid:     proto.Int32(int32(p.SeatID)),
		ActionType: proto.Int32(actionType),
		Chips:      proto.Int64(chips),
		HandChips:  proto.Int64(p.Gold),
	}
	m.sendPacket(room, "pb.ActionBRC", rsp)
}

// broadcastShowHand sends ShowHandRSP with all contested players' hole cards.
func (m *Manager) broadcastShowHand(room *Room, players []*Player) {
	var info []*gen.ShowHandInfo
	for _, p := range players {
		si := &gen.ShowHandInfo{Seatid: proto.Int32(int32(p.SeatID))}
		if len(p.Cards) >= 1 {
			si.Card1 = proto.Int32(int32(p.Cards[0]))
		}
		if len(p.Cards) >= 2 {
			si.Card2 = proto.Int32(int32(p.Cards[1]))
		}
		info = append(info, si)
	}
	m.sendPacket(room, "pb.ShowHandRSP", &gen.ShowHandRSP{Info: info})
}

// broadcastRoundOver sends RoundOverBRC with side-pot details.
func (m *Manager) broadcastRoundOver(room *Room, pools []int64, poolRds []*gen.PoolRD) {
	m.sendPacket(room, "pb.RoundOverBRC", &gen.RoundOverBRC{Pool: pools, PoolRd: poolRds})
}

// broadcastWinner sends WinnerRSP with per-pool winners and profit info.
func (m *Manager) broadcastWinner(room *Room, winners []*gen.WinningInfo, profits []*gen.WinningProfit) {
	m.sendPacket(room, "pb.WinnerRSP", &gen.WinnerRSP{Winner: winners, Profit: profits})
}

// broadcastChipsBack sends ChipsBackBRC to return chips to a player's stack.
func (m *Manager) broadcastChipsBack(room *Room, seatID int, chips int64) {
	m.sendPacket(room, "pb.ChipsBackBRC", &gen.ChipsBackBRC{
		Seatid: proto.Int32(int32(seatID)),
		Chips:  proto.Int64(chips),
	})
}

// broadcastStandUp sends StandUpBRC.
func (m *Manager) broadcastStandUp(room *Room, seatID int) {
	m.sendPacket(room, "pb.StandUpBRC", &gen.StandUpBRC{Seatid: proto.Int32(int32(seatID))})
}

// broadcastPreActionReset notifies players their pre-action was invalidated.
func (m *Manager) broadcastPreActionReset(room *Room, uids []int64) {
	if len(uids) == 0 {
		return
	}
	body, _ := proto.Marshal(&gen.PreActionResetRSP{})
	m.wsSrv.BroadcastToUIDs(uids, "pb.PreActionResetRSP", room.ID, body)
}

// broadcastReby sends RebyBRC when a player rebuys.
func (m *Manager) broadcastReby(room *Room, seatID int, chips int64, rebyType string) {
	m.sendPacket(room, "pb.RebyBRC", &gen.RebyBRC{
		Code:   proto.Int32(0),
		Seatid: proto.Int32(int32(seatID)),
		Chips:  proto.Int64(chips),
		Type:   proto.String(rebyType),
	})
}

// broadcastShowMyCard sends ShowMyCardBRC.
func (m *Manager) broadcastShowMyCard(room *Room, seatID int, showCardInfo []int32) {
	m.sendPacket(room, "pb.ShowMyCardBRC", &gen.ShowMyCardBRC{
		Gameid: proto.String(room.Table.GameID),
		Info: []*gen.ShowMyCardInfo{
			{
				Seatid:    proto.Int32(int32(seatID)),
				HandCards: showCardInfo,
			},
		},
	})
}

// broadcastEmoji sends EmojiBRC to the room.
func (m *Manager) broadcastEmoji(room *Room, fromUID, toUID int64, emojiID int32) {
	m.sendPacket(room, "pb.EmojiBRC", &gen.EmojiBRC{
		Uid:   proto.Int64(fromUID),
		ToUid: proto.Int64(toUID),
		Id:    proto.Int32(emojiID),
	})
}

// sendToUID marshals and sends a packet to a single uid in the room.
func (m *Manager) sendToUID(room *Room, uid int64, packType string, msg proto.Message) {
	body, _ := proto.Marshal(msg)
	m.wsSrv.BroadcastToUIDs([]int64{uid}, packType, room.ID, body)
}
