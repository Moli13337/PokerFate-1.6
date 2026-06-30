package game

import (
	"context"

	"google.golang.org/protobuf/proto"

	"poker-fate-server/internal/proto/gen"
	"poker-fate-server/internal/ws"
)

// Gold model (private server): table chips are a copy of user.gold at sit-down.
// No deduction on sit, no refund on leave. finishRound credits winners to
// users.gold via persistWinToDB; losses are free. Rebuys add to table chips
// without deducting from the bank. This keeps the bank monotonic and avoids
// double-counting with round.go's persistWinToDB.

// respond marshals msg and sends it back to the requesting session.
func (m *Manager) respond(sess *ws.Session, pkt *ws.Packet, packType string, msg proto.Message) {
	body, _ := proto.Marshal(msg)
	m.wsSrv.BroadcastToUIDs([]int64{sess.UID}, packType, pkt.RoomID, body)
}

// playerInRoom returns the room containing uid and the seated player (or nil).
// The returned room has no lock held; callers re-lock for mutations.
func (m *Manager) playerInRoom(uid int64) (*Room, *Player) {
	room := m.getPlayerRoom(uid)
	if room == nil {
		return nil, nil
	}
	room.mu.Lock()
	defer room.mu.Unlock()
	for _, p := range room.Players {
		if p.UID == uid {
			return room, p
		}
	}
	return room, nil
}

// --- Core room/action handlers ---

// HandleQuickStartREQ finds/creates a room at the requested boot and seats the
// player, then returns QuickStartRSP with the room id.
func (m *Manager) HandleQuickStartREQ(sess *ws.Session, pkt *ws.Packet) {
	req := &gen.QuickStartREQ{}
	if err := proto.Unmarshal(pkt.Body, req); err != nil {
		return
	}
	boot := req.GetBoot()
	if boot <= 0 {
		boot = 1000
	}

	// Prevent multi-room/multi-seat conflicts: if the player is already in a
	// room (e.g. from a previous QuickStart that hasn't finished), remove them
	// first. Without this, rapid double-clicks put the same uid in two seats
	// or two rooms, and the client crashes from conflicting BRC packets.
	m.cleanupPlayerRooms(sess.UID)

	room := m.findOrCreateRoom(boot)
	user, err := m.getUser(sess.UID)
	if err != nil {
		m.respond(sess, pkt, "pb.QuickStartRSP", &gen.QuickStartRSP{Code: proto.Int32(-1)})
		return
	}

	room.mu.Lock()
	seatID := room.findEmptySeat()
	if seatID < 0 {
		room.mu.Unlock()
		m.respond(sess, pkt, "pb.QuickStartRSP", &gen.QuickStartRSP{Code: proto.Int32(-1)})
		return
	}
	player := &Player{
		UID:    user.UID,
		Name:   user.Name,
		Avatar: user.Avatar,
		Gold:   user.Gold,
		RoleID: user.UsingRoleID,
		SkinID: user.UsingSkinID,
		SeatID: seatID,
	}
	room.Players[seatID] = player
	room.Spectators[sess.UID] = true
	startNeeded := len(room.Players) >= 2 && room.Table.Stage == StageNone
	room.mu.Unlock()

	m.broadcastSitDown(room, player)
	if startNeeded {
		go m.startRound(room)
	}

	m.respond(sess, pkt, "pb.QuickStartRSP", &gen.QuickStartRSP{
		Code:      proto.Int32(0),
		GameType:  proto.Int32(req.GetGameType()),
		Boot:      proto.Int64(boot),
		LobbyCoin: proto.Int32(req.GetLobbyCoin()),
	})
}

// HandleSitDownREQ seats the player at an explicit seat in their current room.
// SitDownREQ carries only Seatid/Chips/WaitBlind — the room is resolved from
// the player's spectator membership (they must have EnterRoom'd first).
func (m *Manager) HandleSitDownREQ(sess *ws.Session, pkt *ws.Packet) {
	req := &gen.SitDownREQ{}
	if err := proto.Unmarshal(pkt.Body, req); err != nil {
		return
	}

	// If the player is already seated somewhere, remove them first to prevent
	// the same multi-seat crash as QuickStart.
	if existing := m.getPlayerRoom(sess.UID); existing != nil {
		m.removePlayerFromRoom(existing, sess.UID)
	}

	room := m.getSpectatorRoom(sess.UID)
	if room == nil {
		m.respond(sess, pkt, "pb.SitDownRSP", &gen.SitDownRSP{Code: proto.Int32(-1)})
		return
	}
	user, err := m.getUser(sess.UID)
	if err != nil {
		m.respond(sess, pkt, "pb.SitDownRSP", &gen.SitDownRSP{Code: proto.Int32(-1)})
		return
	}

	room.mu.Lock()
	seatID := int(req.GetSeatid())
	if seatID <= 0 {
		seatID = room.findEmptySeat()
	}
	if seatID < 0 || room.Players[seatID] != nil {
		room.mu.Unlock()
		m.respond(sess, pkt, "pb.SitDownRSP", &gen.SitDownRSP{Code: proto.Int32(-1)})
		return
	}
	player := &Player{
		UID:    user.UID,
		Name:   user.Name,
		Avatar: user.Avatar,
		Gold:   user.Gold,
		RoleID: user.UsingRoleID,
		SkinID: user.UsingSkinID,
		SeatID: seatID,
	}
	room.Players[seatID] = player
	room.Spectators[sess.UID] = true
	startNeeded := len(room.Players) >= 2 && room.Table.Stage == StageNone
	room.mu.Unlock()

	m.broadcastSitDown(room, player)
	if startNeeded {
		go m.startRound(room)
	}

	m.respond(sess, pkt, "pb.SitDownRSP", &gen.SitDownRSP{
		Code:   proto.Int32(0),
		Seatid: proto.Int32(int32(seatID)),
		Chips:  proto.Int64(user.Gold),
	})
}

// HandleActionREQ applies the player's fold/call/raise/allin action.
func (m *Manager) HandleActionREQ(sess *ws.Session, pkt *ws.Packet) {
	req := &gen.ActionREQ{}
	if err := proto.Unmarshal(pkt.Body, req); err != nil {
		return
	}
	room := m.getPlayerRoom(sess.UID)
	if room == nil {
		m.respond(sess, pkt, "pb.ActionRSP", &gen.ActionRSP{Code: proto.Int32(-1)})
		return
	}
	room.mu.Lock()
	var player *Player
	for _, p := range room.Players {
		if p.UID == sess.UID {
			player = p
			break
		}
	}
	if player == nil || room.Table.ActiveIdx != player.SeatID {
		room.mu.Unlock()
		m.respond(sess, pkt, "pb.ActionRSP", &gen.ActionRSP{Code: proto.Int32(-1)})
		return
	}
	m.processAction(room, player, req.GetActionType(), req.GetChips())
	room.mu.Unlock()

	m.respond(sess, pkt, "pb.ActionRSP", &gen.ActionRSP{Code: proto.Int32(0)})
}

// HandleStandUpREQ removes the player from their seat.
func (m *Manager) HandleStandUpREQ(sess *ws.Session, pkt *ws.Packet) {
	room := m.getPlayerRoom(sess.UID)
	if room == nil {
		m.respond(sess, pkt, "pb.StandUpRSP", &gen.StandUpRSP{Code: proto.Int32(0)})
		return
	}
	room.mu.Lock()
	seatID := -1
	for s, p := range room.Players {
		if p.UID == sess.UID {
			seatID = s
			break
		}
	}
	if seatID > 0 {
		delete(room.Players, seatID)
	}
	delete(room.Spectators, sess.UID)
	room.mu.Unlock()

	if seatID > 0 {
		m.broadcastStandUp(room, seatID)
	}
	m.respond(sess, pkt, "pb.StandUpRSP", &gen.StandUpRSP{Code: proto.Int32(0)})
}

// HandleLeaveRoomREQ leaves the room (standing up first if seated).
func (m *Manager) HandleLeaveRoomREQ(sess *ws.Session, pkt *ws.Packet) {
	room := m.getPlayerRoom(sess.UID)
	if room == nil {
		m.respond(sess, pkt, "pb.LeaveRoomRSP", &gen.LeaveRoomRSP{Code: proto.Int32(0)})
		return
	}
	room.mu.Lock()
	seatID := -1
	for s, p := range room.Players {
		if p.UID == sess.UID {
			seatID = s
			break
		}
	}
	if seatID > 0 {
		delete(room.Players, seatID)
	}
	delete(room.Spectators, sess.UID)
	room.mu.Unlock()

	if seatID > 0 {
		m.broadcastStandUp(room, seatID)
	}
	m.respond(sess, pkt, "pb.LeaveRoomRSP", &gen.LeaveRoomRSP{Code: proto.Int32(0)})
}

// HandleEnterRoomREQ joins the room as a spectator and returns full table state.
func (m *Manager) HandleEnterRoomREQ(sess *ws.Session, pkt *ws.Packet) {
	req := &gen.EnterRoomREQ{}
	if err := proto.Unmarshal(pkt.Body, req); err != nil {
		return
	}
	m.mu.RLock()
	room := m.rooms[uint32(req.GetRoomid())]
	m.mu.RUnlock()
	if room == nil {
		m.respond(sess, pkt, "pb.EnterRoomRSP", &gen.EnterRoomRSP{Code: proto.Int32(-1)})
		return
	}
	room.mu.Lock()
	room.Spectators[sess.UID] = true
	var player *Player
	for _, p := range room.Players {
		if p.UID == sess.UID {
			player = p
			break
		}
	}
	rsp := m.buildEnterRoomRSP(room, player)
	room.mu.Unlock()

	m.respond(sess, pkt, "pb.EnterRoomRSP", rsp)
}

// --- In-game poker sub-handlers ---

// HandlePreActionREQ stores the player's pre-set action (check/fold, call
// current, call any) for auto-execution when it becomes their turn.
func (m *Manager) HandlePreActionREQ(sess *ws.Session, pkt *ws.Packet) {
	req := &gen.PreActionREQ{}
	if err := proto.Unmarshal(pkt.Body, req); err != nil {
		return
	}
	room, player := m.playerInRoom(sess.UID)
	if player == nil {
		m.respond(sess, pkt, "pb.PreActionRSP", &gen.PreActionRSP{Code: proto.Int32(-1)})
		return
	}
	room.mu.Lock()
	player.PreActionType = req.GetType()
	player.PreActionChips = req.GetChips()
	room.mu.Unlock()

	m.respond(sess, pkt, "pb.PreActionRSP", &gen.PreActionRSP{
		Code:  proto.Int32(0),
		Type:  proto.Int32(req.GetType()),
		Chips: proto.Int64(req.GetChips()),
	})
}

// HandleShowMyCardREQ toggles the per-card show flag and broadcasts it.
func (m *Manager) HandleShowMyCardREQ(sess *ws.Session, pkt *ws.Packet) {
	req := &gen.ShowMyCardREQ{}
	if err := proto.Unmarshal(pkt.Body, req); err != nil {
		return
	}
	room, player := m.playerInRoom(sess.UID)
	if player == nil {
		m.respond(sess, pkt, "pb.ShowMyCardRSP", &gen.ShowMyCardRSP{Code: proto.Int32(-1)})
		return
	}
	pos := int(req.GetPos())
	room.mu.Lock()
	if pos >= 1 && pos <= len(player.ShowCardInfo) {
		player.ShowCardInfo[pos-1] = req.GetFlag()
	}
	seatID := player.SeatID
	showInfo := append([]int32(nil), player.ShowCardInfo...)
	room.mu.Unlock()

	m.respond(sess, pkt, "pb.ShowMyCardRSP", &gen.ShowMyCardRSP{
		Code:   proto.Int32(0),
		Gameid: proto.String(req.GetGameid()),
		Pos:    proto.Int32(req.GetPos()),
		Flag:   proto.Int32(req.GetFlag()),
	})
	m.broadcastShowMyCard(room, seatID, showInfo)
}

// HandleSetWaitBlindTypeREQ sets whether the player waits for the big blind.
func (m *Manager) HandleSetWaitBlindTypeREQ(sess *ws.Session, pkt *ws.Packet) {
	req := &gen.SetWaitBlindTypeREQ{}
	if err := proto.Unmarshal(pkt.Body, req); err != nil {
		return
	}
	room, player := m.playerInRoom(sess.UID)
	if player == nil {
		m.respond(sess, pkt, "pb.SetWaitBlindTypeRSP", &gen.SetWaitBlindTypeRSP{Code: proto.Int32(-1)})
		return
	}
	var wbt int32
	if req.GetWaitBlind() {
		wbt = 1
	}
	room.mu.Lock()
	player.WaitBlindType = wbt
	room.mu.Unlock()

	m.respond(sess, pkt, "pb.SetWaitBlindTypeRSP", &gen.SetWaitBlindTypeRSP{
		Code:          proto.Int32(0),
		WaitBlindType: proto.Int32(wbt),
	})
}

// HandleRebyREQ rebuys chips for the player (no RSP, only RebyBRC broadcast).
func (m *Manager) HandleRebyREQ(sess *ws.Session, pkt *ws.Packet) {
	req := &gen.RebyREQ{}
	if err := proto.Unmarshal(pkt.Body, req); err != nil {
		return
	}
	if !req.GetIsReby() {
		return
	}
	room, player := m.playerInRoom(sess.UID)
	if player == nil {
		return
	}
	room.mu.Lock()
	player.Gold += req.GetChips()
	seatID := player.SeatID
	room.mu.Unlock()

	m.broadcastReby(room, seatID, req.GetChips(), "reby")
}

// HandleSetRebyREQ stores the auto-rebuy threshold.
func (m *Manager) HandleSetRebyREQ(sess *ws.Session, pkt *ws.Packet) {
	req := &gen.SetRebyREQ{}
	if err := proto.Unmarshal(pkt.Body, req); err != nil {
		return
	}
	room, player := m.playerInRoom(sess.UID)
	if player == nil {
		m.respond(sess, pkt, "pb.SetRebyRSP", &gen.SetRebyRSP{Code: proto.Int32(-1)})
		return
	}
	room.mu.Lock()
	player.RebyChips = req.GetRebyChips()
	room.mu.Unlock()

	m.respond(sess, pkt, "pb.SetRebyRSP", &gen.SetRebyRSP{
		Code:      proto.Int32(0),
		RebyChips: proto.Int64(req.GetRebyChips()),
	})
}

// HandleGetCardsREQ returns the current board and the player's hole cards
// (used by the client on disconnect/reconnect).
func (m *Manager) HandleGetCardsREQ(sess *ws.Session, pkt *ws.Packet) {
	room, player := m.playerInRoom(sess.UID)
	rsp := &gen.GetCardsRSP{BoardCards: []int32{}}
	if room == nil || player == nil {
		m.respond(sess, pkt, "pb.GetCardsRSP", rsp)
		return
	}
	room.mu.Lock()
	rsp.BoardCards = m.communityToInt32(room.Table.Community)
	if len(player.Cards) >= 1 {
		rsp.Card1 = proto.Int32(int32(player.Cards[0]))
	}
	if len(player.Cards) >= 2 {
		rsp.Card2 = proto.Int32(int32(player.Cards[1]))
	}
	room.mu.Unlock()

	m.respond(sess, pkt, "pb.GetCardsRSP", rsp)
}

// HandleGetHandsListREQ returns a persisted hand by id (0 = latest).
func (m *Manager) HandleGetHandsListREQ(sess *ws.Session, pkt *ws.Packet) {
	req := &gen.GetHandsListREQ{}
	if err := proto.Unmarshal(pkt.Body, req); err != nil {
		return
	}
	var info *gen.HandsInfo
	if req.GetId() == 0 {
		info = m.loadLatestHandHistory(sess.UID)
	} else if m.db != nil {
		var gameID string
		_ = m.db.QueryRowContext(context.Background(),
			`SELECT gameid FROM user_hand_history WHERE uid=$1 AND hand_id=$2 ORDER BY created_at DESC LIMIT 1`,
			sess.UID, req.GetId()).Scan(&gameID)
		if gameID != "" {
			info = m.loadHandHistory(sess.UID, gameID)
		}
	}
	m.respond(sess, pkt, "pb.GetHandsListRSP", &gen.GetHandsListRSP{
		Code: proto.Int32(0),
		Info: info,
	})
}

// HandleRoundStartDisplayFinishREQ is a client ack that round-start rendering
// is done; it does not block the round flow.
func (m *Manager) HandleRoundStartDisplayFinishREQ(sess *ws.Session, pkt *ws.Packet) {
	m.respond(sess, pkt, "pb.RoundStartDisplayFinishRSP", &gen.RoundStartDisplayFinishRSP{Code: proto.Int32(0)})
}

// HandleSendEmojiREQ broadcasts an emoji to the room.
func (m *Manager) HandleSendEmojiREQ(sess *ws.Session, pkt *ws.Packet) {
	req := &gen.SendEmojiREQ{}
	if err := proto.Unmarshal(pkt.Body, req); err != nil {
		return
	}
	room, _ := m.playerInRoom(sess.UID)
	m.respond(sess, pkt, "pb.SendEmojiRSP", &gen.SendEmojiRSP{
		Code:           proto.Int32(0),
		EmojiFreeTimes: proto.Int32(0),
	})
	if room != nil {
		m.broadcastEmoji(room, sess.UID, req.GetToUid(), req.GetId())
	}
}

// --- EnterRoomRSP builders (all assume room.mu is held by the caller) ---

func (m *Manager) buildEnterRoomRSP(room *Room, player *Player) *gen.EnterRoomRSP {
	return &gen.EnterRoomRSP{
		Code:          proto.Int32(0),
		Roomid:        proto.Int32(int32(room.ID)),
		UserNum:       proto.Int32(int32(len(room.Players))),
		GameType:      proto.Int32(1),
		TableStatus:   m.buildTableStatus(room),
		RoomStatus:    &gen.RoomStatus{Profit: []*gen.ProfitInfo{}, IsJackpotOpen: proto.Bool(false)},
		RoomInfo:      m.buildRoomInfo(room),
		PlayingStatus: m.buildPlayingStatus(room, player),
		Scheme:        m.buildScheme(player),
	}
}

func (m *Manager) buildTableStatus(room *Room) *gen.TableStatus {
	t := room.Table
	return &gen.TableStatus{
		IsPlaying:  proto.Bool(t.Stage >= StagePreflop),
		ActionIdx:  proto.Int32(int32(t.ActiveIdx)),
		DIdx:       proto.Int32(int32(t.Dealer)),
		SbIdx:      proto.Int32(int32(t.SBSeat)),
		BbIdx:      proto.Int32(int32(t.BBSeat)),
		Seat:       m.buildSeatStatuses(room, t.Stage == StageShow),
		Pool:       []int64{t.Pot},
		Stage:      proto.Int32(stageProto(t.Stage)),
		Board:      m.communityToInt32(t.Community),
		HandId:     proto.String(t.GameID),
		Gameid:     proto.String(t.GameID),
		IsShowCard: proto.Bool(t.Stage == StageShow),
		HisDetail:  m.buildHisDetail(room),
	}
}

func (m *Manager) buildSeatStatuses(room *Room, showCards bool) []*gen.SeatStatus {
	seats := make([]*gen.SeatStatus, 0, room.Config.MaxPlayers)
	for i := 1; i <= room.Config.MaxPlayers; i++ {
		p, ok := room.Players[i]
		if !ok {
			seats = append(seats, &gen.SeatStatus{
				Seatid:      proto.Int32(int32(i)),
				HasCard:     proto.Bool(false),
				SeatReserve: proto.Bool(false),
				SeatIndex:   proto.Int32(int32(i)),
			})
			continue
		}
		ss := &gen.SeatStatus{
			Seatid:         proto.Int32(int32(i)),
			Player:         buildUserBrief(p),
			HandChips:      proto.Int64(p.Gold),
			DestopChips:    proto.Int64(p.Gold),
			HasCard:        proto.Bool(!p.Folded && len(p.Cards) > 0),
			SeatReserve:    proto.Bool(false),
			BeginChips:     proto.Int64(p.BeginChips),
			LastActionType: proto.Int32(p.LastActionType),
			WaitBlindType:  proto.Int32(p.WaitBlindType),
			IsAllin:        proto.Bool(p.AllIn),
			SeatIndex:      proto.Int32(int32(i)),
		}
		if showCards && !p.Folded && len(p.Cards) >= 2 {
			ss.Card1 = proto.Int32(int32(p.Cards[0]))
			ss.Card2 = proto.Int32(int32(p.Cards[1]))
		}
		seats = append(seats, ss)
	}
	return seats
}

func (m *Manager) buildPlayingStatus(room *Room, player *Player) *gen.PlayingStatus {
	ps := &gen.PlayingStatus{
		ActionSeatid: proto.Int32(int32(room.Table.ActiveIdx)),
		ActionTime:   proto.Int32(cashActionTime),
	}
	if player == nil || len(player.Cards) < 2 {
		return ps
	}
	ps.Card1 = proto.Int32(int32(player.Cards[0]))
	ps.Card2 = proto.Int32(int32(player.Cards[1]))
	ps.PreActionType = proto.Int32(player.PreActionType)
	ps.PreActionChips = proto.Int64(player.PreActionChips)
	ps.ShowCardInfo = player.ShowCardInfo
	ps.ThinkTime = proto.Int32(player.ThinkTime)

	// Fill bet bounds only when it is this player's turn.
	if room.Table.ActiveIdx == player.SeatID {
		callNeed := m.maxBet(room) - player.Bet
		if callNeed < 0 {
			callNeed = 0
		}
		minChipin := room.Table.LastRaise
		if minChipin < room.Table.BigBlind {
			minChipin = room.Table.BigBlind
		}
		if player.Gold < callNeed+minChipin {
			minChipin = 0
		}
		maxChipin := player.Gold - callNeed
		if maxChipin < 0 {
			maxChipin = 0
		}
		ps.CallNeedChips = proto.Int64(callNeed)
		ps.MinChipin = proto.Int64(minChipin)
		ps.MaxChipin = proto.Int64(maxChipin)
	}
	return ps
}

func (m *Manager) buildRoomInfo(room *Room) *gen.RoomInfo {
	return &gen.RoomInfo{
		Roomid:     proto.Int32(int32(room.ID)),
		Sb:         proto.Int64(room.Config.Boot / 2),
		Bb:         proto.Int64(room.Config.Boot),
		Ante:       proto.Int64(room.Config.Ante),
		MinByin:    proto.Int64(room.Config.MinBuyIn),
		MaxByin:    proto.Int64(room.Config.MaxBuyIn),
		ActionTime: proto.Int32(cashActionTime),
		SeatNum:    proto.Int32(int32(room.Config.MaxPlayers)),
		Feetype:    proto.Int32(1),
		Feepoint:   proto.Int32(5),
		GameType:   proto.Int32(1),
	}
}

// buildScheme returns a default decoration scheme. The private server unlocks
// all cosmetics, so this is a fixed default rather than a DB lookup.
func (m *Manager) buildScheme(player *Player) *gen.DCSchemeRoomItem {
	scheme := &gen.DCSchemeRoomItem{
		Table:     proto.Int32(1),
		CardBack:  proto.Int32(1),
		CardFront: proto.Int32(1),
	}
	if player != nil {
		scheme.RoleId = proto.Int32(int32(player.RoleID))
		scheme.SkinId = proto.Int32(int32(player.SkinID))
	}
	return scheme
}
