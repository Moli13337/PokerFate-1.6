package game

import (
	"context"
	"math/rand"
	"time"

	"github.com/google/uuid"
	"google.golang.org/protobuf/proto"

	"poker-fate-server/internal/proto/gen"
)

const (
	actionFold  int32 = 1
	actionCall  int32 = 2
	actionRaise int32 = 3
	actionAllIn int32 = 4

	cashActionTime int32 = 15
)

// startRound begins a new hand: generates gameid, charges blinds/ante, deals
// cards, and broadcasts DealerInfoRSP + AnteBRC + HandCardRSP + RoundStartBRC.
func (m *Manager) startRound(room *Room) {
	room.mu.Lock()
	defer room.mu.Unlock()

	table := room.Table
	table.NewDeck()
	table.GameID = uuid.NewString()
	table.HandID++
	table.SmallBlind = room.Config.Boot / 2
	table.BigBlind = room.Config.Boot
	table.RoundActions = make(map[int32][]*gen.ActionRD)
	table.RoundPots = map[int32]int64{1: 0}
	table.LastRaise = table.BigBlind
	table.Community = nil
	table.Pot = 0

	// Rotate dealer to the next occupied seat from the previous dealer.
	if table.Dealer == 0 {
		// First hand: pick a random seat.
		seats := make([]int, 0, len(room.Players))
		for s := range room.Players {
			seats = append(seats, s)
		}
		table.Dealer = seats[rand.Intn(len(seats))]
	} else {
		table.Dealer = m.nextSeat(room, table.Dealer)
	}
	table.SBSeat = m.nextSeat(room, table.Dealer)
	table.BBSeat = m.nextSeat(room, table.SBSeat)

	// Reset per-hand player state.
	for _, p := range room.Players {
		p.Cards = table.Deal()
		p.Folded = false
		p.AllIn = false
		p.Bet = 0
		p.TotalBet = 0
		p.PreActionType = 0
		p.PreActionChips = 0
		p.ShowCardInfo = make([]int32, len(p.Cards))
		p.BeginChips = p.Gold
		p.LastActionType = 0
	}

	// Charge ante (if configured) and broadcast AnteBRC.
	if room.Config.Ante > 0 {
		var anteInfos []*gen.AnteInfo
		for seatID, p := range room.Players {
			ante := room.Config.Ante
			if ante > p.Gold {
				ante = p.Gold
			}
			p.Gold -= ante
			p.TotalBet += ante
			table.Pot += ante
			anteInfos = append(anteInfos, &gen.AnteInfo{
				Seatid: proto.Int32(int32(seatID)),
				Ante:   proto.Int64(ante),
			})
		}
		m.broadcastAnte(room, anteInfos)
	}

	// Broadcast dealer/blind seats + gameid.
	m.broadcastDealerInfo(room)

	// Send hole cards privately to each player.
	for _, p := range room.Players {
		if p.IsAI {
			continue
		}
		m.broadcastHandCard(room, p)
	}

	// Post small blind and big blind.
	for seatID, p := range room.Players {
		if seatID == table.SBSeat {
			sb := table.SmallBlind
			if sb > p.Gold {
				sb = p.Gold
				p.AllIn = true
			}
			p.Gold -= sb
			p.Bet = sb
			p.TotalBet += sb
			table.Pot += sb
		}
		if seatID == table.BBSeat {
			bb := table.BigBlind
			if bb > p.Gold {
				bb = p.Gold
				p.AllIn = true
			}
			p.Gold -= bb
			p.Bet = bb
			p.TotalBet += bb
			table.Pot += bb
		}
	}

	table.Stage = StagePreflop
	table.LastAggressor = table.BBSeat
	table.ActiveIdx = m.nextSeat(room, table.BBSeat)

	m.broadcastRoundStart(room, StagePreflop)
	m.requestAction(room)
}

// requestAction computes bet bounds, handles pre-action, starts the timeout
// timer, and broadcasts ActionNotifyBRC for the current actor.
func (m *Manager) requestAction(room *Room) {
	table := room.Table
	p, ok := room.Players[table.ActiveIdx]
	if !ok || p.Folded || p.AllIn {
		m.advanceAction(room)
		return
	}

	callNeed := m.maxBet(room) - p.Bet
	if callNeed < 0 {
		callNeed = 0
	}

	// Min raise = max(last raise size, big blind). If player can't afford it,
	// minChipin=0 signals the client that raise is unavailable.
	minChipin := table.LastRaise
	if minChipin < table.BigBlind {
		minChipin = table.BigBlind
	}
	if p.Gold < callNeed+minChipin {
		minChipin = 0
	}
	maxChipin := p.Gold - callNeed
	if maxChipin < 0 {
		maxChipin = 0
	}

	// Auto-execute pre-action if applicable.
	if p.PreActionType != 0 {
		executed := m.tryPreAction(room, p, callNeed, minChipin)
		if executed {
			return
		}
	}

	// AI decides asynchronously.
	if p.IsAI {
		go func() {
			time.Sleep(time.Duration(1+rand.Intn(3)) * time.Second)
			room.mu.Lock()
			defer room.mu.Unlock()
			actionType, chips := m.aiDecide(room, p, callNeed, minChipin)
			m.processAction(room, p, actionType, chips)
		}()
		return
	}

	thinkTime := cashActionTime
	m.broadcastActionNotify(room, p, callNeed, minChipin, maxChipin, thinkTime)
	m.startActionTimer(room, p, callNeed)
}

// tryPreAction executes a player's pre-set action if it applies to the current
// situation. Returns true if the action was executed (and the round advanced).
func (m *Manager) tryPreAction(room *Room, p *Player, callNeed, minChipin int64) bool {
	switch p.PreActionType {
	case 1: // check or fold
		if callNeed == 0 {
			p.PreActionType = 0
			m.processAction(room, p, actionCall, 0)
			return true
		}
		p.PreActionType = 0
		m.processAction(room, p, actionFold, 0)
		return true
	case 2: // call current
		if callNeed > 0 && callNeed == p.PreActionChips {
			p.PreActionType = 0
			m.processAction(room, p, actionCall, 0)
			return true
		}
	case 3: // call any
		if callNeed > 0 {
			p.PreActionType = 0
			m.processAction(room, p, actionCall, 0)
			return true
		}
		// call any with no bet → check
		p.PreActionType = 0
		m.processAction(room, p, actionCall, 0)
		return true
	}
	return false
}

// startActionTimer auto-folds/checks the player after the action time expires.
func (m *Manager) startActionTimer(room *Room, p *Player, callNeed int64) {
	table := room.Table
	if table.ActionTimer != nil {
		table.ActionTimer.Stop()
	}
	table.ActionTimer = time.AfterFunc(time.Duration(cashActionTime)*time.Second, func() {
		room.mu.Lock()
		defer room.mu.Unlock()
		// Verify it's still this player's turn.
		if table.ActiveIdx != p.SeatID || p.Folded || p.AllIn {
			return
		}
		if callNeed == 0 {
			m.processAction(room, p, actionCall, 0) // check
		} else {
			m.processAction(room, p, actionFold, 0)
		}
	})
}

// processAction applies a player's action, records it, and advances the round.
func (m *Manager) processAction(room *Room, player *Player, actionType int32, chips int64) {
	table := room.Table
	if table.ActionTimer != nil {
		table.ActionTimer.Stop()
		table.ActionTimer = nil
	}

	callNeed := m.maxBet(room) - player.Bet
	if callNeed < 0 {
		callNeed = 0
	}

	switch actionType {
	case actionFold:
		player.Folded = true
	case actionCall:
		callAmt := callNeed
		if callAmt > player.Gold {
			callAmt = player.Gold
			player.AllIn = true
		}
		player.Gold -= callAmt
		player.Bet += callAmt
		player.TotalBet += callAmt
		table.Pot += callAmt
	case actionRaise:
		// chips is the raise increment above the call. Validate min-raise.
		if chips < table.LastRaise && chips < player.Gold-callNeed {
			chips = table.LastRaise
		}
		total := callNeed + chips
		if total >= player.Gold {
			total = player.Gold
			player.AllIn = true
			chips = total - callNeed
		}
		player.Gold -= total
		player.Bet += total
		player.TotalBet += total
		table.Pot += total
		// New raise size sets the min for future raises.
		if chips > table.LastRaise {
			table.LastRaise = chips
		}
		table.LastAggressor = player.SeatID
		// Invalidate other players' pre-actions.
		m.invalidatePreActions(room, player.SeatID)
	case actionAllIn:
		allin := player.Gold
		player.Gold = 0
		player.Bet += allin
		player.TotalBet += allin
		table.Pot += allin
		player.AllIn = true
		raisePart := player.Bet - m.maxBet(room) + allin
		if raisePart > table.LastRaise {
			table.LastRaise = raisePart
		}
		if player.Bet > m.maxBet(room)-allin+player.Bet {
			table.LastAggressor = player.SeatID
			m.invalidatePreActions(room, player.SeatID)
		}
	}

	player.LastActionType = actionType
	m.recordAction(room, player, actionType, chips)
	m.broadcastAction(room, player, actionType, chips)
	m.advanceAction(room)
}

// invalidatePreActions clears pre-actions and notifies affected players.
func (m *Manager) invalidatePreActions(room *Room, exceptSeat int) {
	var resetUIDs []int64
	for _, p := range room.Players {
		if p.SeatID == exceptSeat || p.Folded || p.AllIn {
			continue
		}
		if p.PreActionType != 0 {
			p.PreActionType = 0
			if !p.IsAI {
				resetUIDs = append(resetUIDs, p.UID)
			}
		}
	}
	m.broadcastPreActionReset(room, resetUIDs)
}

// recordAction appends an ActionRD to the current stage's history.
func (m *Manager) recordAction(room *Room, p *Player, actionType int32, chips int64) {
	table := room.Table
	st := stageProto(table.Stage)
	rd := &gen.ActionRD{
		Uid:           proto.Int64(p.UID),
		Seatid:        proto.Int32(int32(p.SeatID)),
		ActionType:    proto.Int32(actionType),
		ActionChips:   proto.Int64(chips),
		IsAllin:       proto.Bool(p.AllIn),
		LeftHandChips: proto.Int64(p.Gold),
		Name:          proto.String(p.Name),
	}
	table.RoundActions[st] = append(table.RoundActions[st], rd)
}

// advanceAction moves to the next actor or advances the stage / finishes.
func (m *Manager) advanceAction(room *Room) {
	table := room.Table

	// Check if only one non-folded player remains.
	activeCount := 0
	for _, p := range room.Players {
		if !p.Folded {
			activeCount++
		}
	}
	if activeCount <= 1 {
		m.finishRound(room)
		return
	}

	// Check if all non-folded non-allin players have matched the max bet
	// AND action has returned to the last aggressor.
	maxBet := m.maxBet(room)
	allMatched := true
	canAct := 0
	for _, p := range room.Players {
		if p.Folded || p.AllIn {
			continue
		}
		canAct++
		if p.Bet != maxBet {
			allMatched = false
		}
	}

	// If everyone is all-in or folded, run out the board.
	if canAct <= 1 && allMatched {
		m.runOutBoard(room)
		return
	}

	if allMatched && table.ActiveIdx == table.LastAggressor {
		// Betting round complete — advance stage.
		m.advanceStage(room)
		return
	}

	// Move to next acting seat.
	table.ActiveIdx = m.nextActingSeat(room, table.ActiveIdx)
	if table.ActiveIdx < 0 {
		m.finishRound(room)
		return
	}
	m.requestAction(room)
}

// advanceStage moves to the next community-card stage.
func (m *Manager) advanceStage(room *Room) {
	table := room.Table
	table.RoundPots[stageProto(table.Stage)] = table.Pot

	switch table.Stage {
	case StagePreflop:
		table.Stage = StageFlop
		table.Community = append(table.Community, table.DealCommunity(3)...)
	case StageFlop:
		table.Stage = StageTurn
		table.Community = append(table.Community, table.DealCommunity(1)...)
	case StageTurn:
		table.Stage = StageRiver
		table.Community = append(table.Community, table.DealCommunity(1)...)
	case StageRiver:
		m.finishRound(room)
		return
	}

	table.resetBets(room)
	table.LastRaise = table.BigBlind
	table.RoundPots[stageProto(table.Stage)] = table.Pot

	m.broadcastRoundStart(room, table.Stage)

	first := m.nextActingSeat(room, table.Dealer)
	if first < 0 {
		m.finishRound(room)
		return
	}
	table.ActiveIdx = first
	table.LastAggressor = first
	m.requestAction(room)
}

// runOutBoard deals remaining community cards without further action when all
// remaining players are all-in.
func (m *Manager) runOutBoard(room *Room) {
	table := room.Table
	for table.Stage < StageRiver {
		table.RoundPots[stageProto(table.Stage)] = table.Pot
		switch table.Stage {
		case StagePreflop:
			table.Stage = StageFlop
			table.Community = append(table.Community, table.DealCommunity(3)...)
		case StageFlop:
			table.Stage = StageTurn
			table.Community = append(table.Community, table.DealCommunity(1)...)
		case StageTurn:
			table.Stage = StageRiver
			table.Community = append(table.Community, table.DealCommunity(1)...)
		}
		table.resetBets(room)
		m.broadcastRoundStart(room, table.Stage)
	}
	m.finishRound(room)
}

// finishRound resolves the pot: shows hands, computes side pots, awards
// winners, broadcasts results, persists history, and schedules the next hand.
func (m *Manager) finishRound(room *Room) {
	table := room.Table

	// Collect non-folded players.
	var activePlayers []*Player
	for _, p := range room.Players {
		if !p.Folded {
			activePlayers = append(activePlayers, p)
		}
	}

	var winners []*gen.WinningInfo
	var profits []*gen.WinningProfit
	var pools []int64
	var poolRds []*gen.PoolRD

	if len(activePlayers) == 1 {
		// Single winner takes the whole pot (no showdown).
		w := activePlayers[0]
		w.Gold += table.Pot
		pools = []int64{table.Pot}
		poolRds = m.buildPoolRds(room, table.Pot, nil)
		winners = append(winners, &gen.WinningInfo{
			Seatid: proto.Int32(int32(w.SeatID)),
			Poolid: proto.Int32(0),
			Chips:  proto.Int64(table.Pot),
			Type:   proto.Int32(1),
			Uid:    proto.Int64(w.UID),
		})
		profits = append(profits, &gen.WinningProfit{
			Seatid: proto.Int32(int32(w.SeatID)),
			Chips:  proto.Int64(table.Pot - w.BeginChips + w.Gold - table.Pot),
			Uid:    proto.Int64(w.UID),
		})
		m.persistWinToDB(w, table.Pot)
	} else if len(activePlayers) > 1 {
		// Showdown: reveal hands.
		table.Stage = StageShow
		m.broadcastShowHand(room, activePlayers)

		// Compute and award side pots.
		pots := computeSidePots(allPlayersSlice(room))
		results := awardSidePots(pots, table.Community)

		for _, res := range results {
			pools = append(pools, res.Pot)
			share := res.Pot / int64(len(res.Winners))
			for _, w := range res.Winners {
				w.Gold += share
				winners = append(winners, &gen.WinningInfo{
					Seatid: proto.Int32(int32(w.SeatID)),
					Poolid: proto.Int32(res.PoolID),
					Chips:  proto.Int64(share),
					Type:   proto.Int32(1),
					Uid:    proto.Int64(w.UID),
				})
				profits = append(profits, &gen.WinningProfit{
					Seatid: proto.Int32(int32(w.SeatID)),
					Chips:  proto.Int64(share),
					Uid:    proto.Int64(w.UID),
				})
				m.persistWinToDB(w, share)
			}
		}
		poolRds = m.buildPoolRds(room, table.Pot, pots)
	}

	// Broadcast round-over with pot details, then winners, then chips-back.
	m.broadcastRoundOver(room, pools, poolRds)
	m.broadcastWinner(room, winners, profits)
	for _, p := range room.Players {
		m.broadcastChipsBack(room, p.SeatID, p.Gold)
	}

	// Persist hand history for human players.
	m.persistHandHistory(room, handHistoryEntry{
		GameID: table.GameID,
		HandID: table.HandID,
		Hands:  m.buildPlayerHands(room, activePlayers),
		Board:  uint32Board(table.Community),
	})

	// Update game stats for human players.
	m.updateGameStats(room, activePlayers, winners)

	// Clean up the table.
	table.Stage = StageNone
	table.Community = nil
	table.Pot = 0
	if table.ActionTimer != nil {
		table.ActionTimer.Stop()
		table.ActionTimer = nil
	}

	// Remove broke AI players.
	for seatID, p := range room.Players {
		if p.IsAI && p.Gold < room.Config.Boot {
			delete(room.Players, seatID)
		}
	}

	// Schedule the next hand if enough players remain.
	remaining := 0
	for _, p := range room.Players {
		if !p.IsAI || p.Gold >= room.Config.Boot {
			remaining++
		}
	}
	if remaining >= 2 {
		go func() {
			time.Sleep(3 * time.Second)
			m.startRound(room)
		}()
	}
}

// persistWinToDB credits a winner's share to their persistent gold balance.
func (m *Manager) persistWinToDB(p *Player, amount int64) {
	if m.db == nil || p.IsAI {
		return
	}
	m.db.ExecContext(context.Background(),
		`UPDATE users SET gold = gold + $1 WHERE uid = $2`, amount, p.UID)
}

// updateGameStats increments play/win counters for human participants.
func (m *Manager) updateGameStats(room *Room, activePlayers []*Player, winners []*gen.WinningInfo) {
	if m.db == nil {
		return
	}
	winnerUIDs := make(map[int64]bool)
	for _, w := range winners {
		winnerUIDs[w.GetUid()] = true
	}
	for _, p := range room.Players {
		if p.IsAI {
			continue
		}
		won := 0
		if winnerUIDs[p.UID] {
			won = 1
		}
		profit := p.Gold - p.BeginChips
		m.db.ExecContext(context.Background(),
			`INSERT INTO user_game_stats (uid, game_type, play_times, win_play_times, profit)
			 VALUES ($1, 1, 1, $2, $3)
			 ON CONFLICT (uid, game_type) DO UPDATE SET
			   play_times = user_game_stats.play_times + 1,
			   win_play_times = user_game_stats.win_play_times + $2,
			   profit = user_game_stats.profit + $3,
			   updated_at = NOW()`,
			p.UID, won, profit)
	}
}

// buildPoolRds constructs the PoolRD list for RoundOverBRC.
func (m *Manager) buildPoolRds(room *Room, totalPot int64, pots []sidePot) []*gen.PoolRD {
	if len(pots) == 0 {
		// Single pot — list all contributors.
		var users []*gen.PoolUser
		for _, p := range room.Players {
			if p.TotalBet > 0 {
				users = append(users, &gen.PoolUser{
					Uid:    proto.Int64(p.UID),
					Chips:  proto.Int64(p.TotalBet),
					Name:   proto.String(p.Name),
					Seatid: proto.Int32(int32(p.SeatID)),
				})
			}
		}
		return []*gen.PoolRD{{Poolid: proto.Int32(0), Pot: proto.Int64(totalPot), Users: users}}
	}
	var rds []*gen.PoolRD
	for i, pot := range pots {
		var users []*gen.PoolUser
		for _, p := range room.Players {
			if p.TotalBet > 0 && !p.Folded {
				users = append(users, &gen.PoolUser{
					Uid:    proto.Int64(p.UID),
					Chips:  proto.Int64(p.TotalBet),
					Name:   proto.String(p.Name),
					Seatid: proto.Int32(int32(p.SeatID)),
				})
			}
		}
		rds = append(rds, &gen.PoolRD{
			Poolid: proto.Int32(int32(i)),
			Pot:    proto.Int64(pot.Pot),
			Users:  users,
		})
	}
	return rds
}

// buildPlayerHands constructs PlayerHands for history persistence.
func (m *Manager) buildPlayerHands(room *Room, activePlayers []*Player) []*gen.PlayerHands {
	var hands []*gen.PlayerHands
	for _, p := range room.Players {
		cards := make([]uint32, len(p.Cards))
		for i, c := range p.Cards {
			cards[i] = uint32(c)
		}
		profit := p.Gold - p.BeginChips
		handType := int32(0)
		if !p.Folded {
			allCards := append(append([]byte{}, p.Cards...), room.Table.Community...)
			handType = int32(EvaluateHand(allCards).Rank)
		}
		hands = append(hands, &gen.PlayerHands{
			Brief:        buildUserBrief(p),
			Cards:        cards,
			Profit:       proto.Int64(profit),
			HandType:     proto.Int32(handType),
			IsFold:       proto.Bool(p.Folded),
			ShowCardInfo: p.ShowCardInfo,
			Seatid:       proto.Int32(int32(p.SeatID)),
		})
	}
	return hands
}

// maxBet returns the highest current-stage bet among all players.
func (m *Manager) maxBet(room *Room) int64 {
	var max int64
	for _, p := range room.Players {
		if p.Bet > max {
			max = p.Bet
		}
	}
	return max
}

// nextSeat returns the next occupied seat after `current`.
func (m *Manager) nextSeat(room *Room, current int) int {
	for i := 1; i <= room.Config.MaxPlayers; i++ {
		seat := (current-1+i)%room.Config.MaxPlayers + 1
		if _, ok := room.Players[seat]; ok {
			return seat
		}
	}
	return current
}

// nextActingSeat returns the next seat that can still act (not folded/allin).
func (m *Manager) nextActingSeat(room *Room, afterSeat int) int {
	for i := 1; i <= room.Config.MaxPlayers; i++ {
		seat := (afterSeat-1+i)%room.Config.MaxPlayers + 1
		p, ok := room.Players[seat]
		if ok && !p.Folded && !p.AllIn {
			return seat
		}
	}
	return -1
}

// allPlayersSlice returns all seated players as a slice.
func allPlayersSlice(room *Room) []*Player {
	out := make([]*Player, 0, len(room.Players))
	for _, p := range room.Players {
		out = append(out, p)
	}
	return out
}

// uint32Board converts byte community cards to uint32 (proto board format).
func uint32Board(cards []byte) []uint32 {
	out := make([]uint32, len(cards))
	for i, c := range cards {
		out[i] = uint32(c)
	}
	return out
}
