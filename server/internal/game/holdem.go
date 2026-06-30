package game

import (
	"math/rand"
	"time"

	"poker-fate-server/internal/proto/gen"
)

type Stage int

const (
	StageNone Stage = iota
	StageDealer
	StagePreflop
	StageFlop
	StageTurn
	StageRiver
	StageShow
	StageWin
	StageRoundOver
)

// HoldemTable holds the per-hand state of a poker table.
type HoldemTable struct {
	RoomID    uint32
	Stage     Stage
	Boot      int64
	Dealer    int
	ActiveIdx int
	Community []byte
	Pot       int64
	Deck      []byte
	DeckIdx   int
	Round     int

	// Per-hand identity & blind seats.
	GameID     string
	HandID     int
	SBSeat     int
	BBSeat     int
	SmallBlind int64
	BigBlind   int64

	// Action timer for the current actor. Stopped in processAction.
	ActionTimer *time.Timer

	// Per-stage action history (for disconnect/reconnect his_detail).
	// Keyed by proto stage value: 1=preflop 2=flop 3=turn 4=river.
	RoundActions map[int32][]*gen.ActionRD
	// Pot snapshot at the start of each stage (same key convention).
	RoundPots map[int32]int64
	// Last raise size in the current stage (for min-raise validation).
	LastRaise int64
	// LastAggressor is the seat of the last raiser in the current stage.
	// A betting round ends when action returns to this seat. In preflop,
	// this is the BB (big blind gets the option to check or raise).
	LastAggressor int
}

func NewHoldemTable(roomID uint32, boot int64) *HoldemTable {
	return &HoldemTable{
		RoomID:       roomID,
		Boot:         boot,
		Stage:        StageNone,
		SBSeat:       -1,
		BBSeat:       -1,
		RoundActions: make(map[int32][]*gen.ActionRD),
		RoundPots:    make(map[int32]int64),
	}
}

func (t *HoldemTable) NewDeck() {
	t.Deck = make([]byte, 52)
	for i := 0; i < 52; i++ {
		t.Deck[i] = byte(i)
	}
	rand.Shuffle(len(t.Deck), func(i, j int) {
		t.Deck[i], t.Deck[j] = t.Deck[j], t.Deck[i]
	})
	t.DeckIdx = 0
}

func (t *HoldemTable) Deal() []byte {
	cards := make([]byte, 2)
	cards[0] = t.Deck[t.DeckIdx]
	cards[1] = t.Deck[t.DeckIdx+1]
	t.DeckIdx += 2
	return cards
}

func (t *HoldemTable) DealCommunity(n int) []byte {
	cards := make([]byte, n)
	for i := 0; i < n; i++ {
		cards[i] = t.Deck[t.DeckIdx]
		t.DeckIdx++
	}
	return cards
}

// resetBets zeroes per-stage Bet for every player (called on stage advance).
func (t *HoldemTable) resetBets(room *Room) {
	for _, p := range room.Players {
		p.Bet = 0
	}
}

// stageProto maps an internal Stage to the proto stage value used by
// RoundStartBRC / RoundRD / ActionNotifyBRC: 1=preflop 2=flop 3=turn 4=river.
func stageProto(s Stage) int32 {
	switch s {
	case StagePreflop:
		return 1
	case StageFlop:
		return 2
	case StageTurn:
		return 3
	case StageRiver:
		return 4
	}
	return 0
}
