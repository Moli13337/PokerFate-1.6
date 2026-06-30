package game

import (
	"context"
	"database/sql"
	"fmt"
	"math/rand"
	"sync"
	"sync/atomic"

	"github.com/redis/go-redis/v9"
	"go.uber.org/zap"

	"poker-fate-server/internal/ws"
)

// Player represents a seated participant in a poker room.
type Player struct {
	UID      int64
	Name     string
	Avatar   int
	Gold     int64
	RoleID   int
	SkinID   int
	SeatID   int
	Cards    []byte
	Folded   bool
	AllIn    bool
	Bet      int64 // current-stage bet
	TotalBet int64 // whole-hand contribution (for side pots)
	IsAI     bool

	// Pre-action (check/fold, call current, call any).
	PreActionType  int32
	PreActionChips int64

	// Wait-blind preference: 0=pay bb now, 1=wait for bb, 2=no wait.
	WaitBlindType int32

	// Auto-reby threshold (0 = disabled).
	RebyChips int64

	// Per-card show flag (0=hidden, 1=shown). Length matches Cards.
	ShowCardInfo []int32

	// Time bank remaining (SNG/MTT). Cash tables use action_time only.
	ThinkTime int32

	// Chips at hand start (for SeatStatus.begin_chips / profit calc).
	BeginChips int64

	// Last action type in the current stage (for reconnect display).
	LastActionType int32
}

// RoomConfig holds the stakes and capacity of a cash table.
type RoomConfig struct {
	Boot       int64
	MinBuyIn   int64
	MaxBuyIn   int64
	MaxPlayers int
	Ante       int64
}

// Room is a single poker table instance.
type Room struct {
	ID         uint32
	Table      *HoldemTable
	Players    map[int]*Player
	Spectators map[int64]bool
	Config     RoomConfig
	mu         sync.Mutex
}

// Manager owns all rooms and registers poker WS handlers.
type Manager struct {
	rooms  map[uint32]*Room
	nextID atomic.Uint32
	mu     sync.RWMutex
	db     *sql.DB
	rdb    *redis.Client
	wsSrv  *ws.Server
	logger *zap.Logger
}

// NewManager creates the manager and registers all poker handlers,
// overriding the stubs registered by ws.RegisterAllHandlers.
func NewManager(db *sql.DB, rdb *redis.Client, wsSrv *ws.Server, logger *zap.Logger) *Manager {
	m := &Manager{
		rooms:  make(map[uint32]*Room),
		db:     db,
		rdb:    rdb,
		wsSrv:  wsSrv,
		logger: logger,
	}
	m.nextID.Store(1000)

	// Core room/action handlers.
	wsSrv.RegisterHandler("pb.QuickStartREQ", m.HandleQuickStartREQ)
	wsSrv.RegisterHandler("pb.SitDownREQ", m.HandleSitDownREQ)
	wsSrv.RegisterHandler("pb.ActionREQ", m.HandleActionREQ)
	wsSrv.RegisterHandler("pb.StandUpREQ", m.HandleStandUpREQ)
	wsSrv.RegisterHandler("pb.LeaveRoomREQ", m.HandleLeaveRoomREQ)
	wsSrv.RegisterHandler("pb.EnterRoomREQ", m.HandleEnterRoomREQ)

	// In-game poker handlers (override ws stubs).
	wsSrv.RegisterHandler("pb.PreActionREQ", m.HandlePreActionREQ)
	wsSrv.RegisterHandler("pb.ShowMyCardREQ", m.HandleShowMyCardREQ)
	wsSrv.RegisterHandler("pb.SetWaitBlindTypeREQ", m.HandleSetWaitBlindTypeREQ)
	wsSrv.RegisterHandler("pb.RebyREQ", m.HandleRebyREQ)
	wsSrv.RegisterHandler("pb.SetRebyREQ", m.HandleSetRebyREQ)
	wsSrv.RegisterHandler("pb.GetCardsREQ", m.HandleGetCardsREQ)
	wsSrv.RegisterHandler("pb.GetHandsListREQ", m.HandleGetHandsListREQ)
	wsSrv.RegisterHandler("pb.RoundStartDisplayFinishREQ", m.HandleRoundStartDisplayFinishREQ)
	wsSrv.RegisterHandler("pb.SendEmojiREQ", m.HandleSendEmojiREQ)

	return m
}

// findOrCreateRoom returns a joinable room at the given boot, or creates one.
func (m *Manager) findOrCreateRoom(boot int64) *Room {
	m.mu.RLock()
	for _, room := range m.rooms {
		if room.Config.Boot == boot && len(room.Players) < room.Config.MaxPlayers && room.Table.Stage < StagePreflop {
			m.mu.RUnlock()
			return room
		}
	}
	m.mu.RUnlock()

	roomID := m.nextID.Add(1)
	cfg := RoomConfig{
		Boot:       boot,
		MinBuyIn:   boot * 40,
		MaxBuyIn:   boot * 200,
		MaxPlayers: 6,
		Ante:       boot / 10,
	}

	room := &Room{
		ID:         roomID,
		Players:    make(map[int]*Player),
		Spectators: make(map[int64]bool),
		Config:     cfg,
		Table:      NewHoldemTable(roomID, boot),
	}

	m.mu.Lock()
	m.rooms[roomID] = room
	m.mu.Unlock()

	if m.wsSrv.Config.Game.AIOpponentEnabled {
		m.addAIRandom(room)
	}
	return room
}

// addAIRandom fills the room with 1-2 AI players so a hand can start.
func (m *Manager) addAIRandom(room *Room) {
	aiCount := 1 + rand.Intn(2)
	for i := 0; i < aiCount; i++ {
		seatID := room.findEmptySeat()
		if seatID < 0 {
			break
		}
		aiID := int64(900000 + i + int(room.ID)*10)
		ai := &Player{
			UID:    aiID,
			Name:   fmt.Sprintf("AI_%d", i+1),
			Avatar: rand.Intn(10),
			Gold:   room.Config.Boot * 100,
			RoleID: 1001,
			SkinID: 1,
			SeatID: seatID,
			IsAI:   true,
		}
		room.Players[seatID] = ai
	}
}

// findEmptySeat returns the lowest free seat id, or -1 if full.
func (r *Room) findEmptySeat() int {
	for i := 1; i <= r.Config.MaxPlayers; i++ {
		if _, ok := r.Players[i]; !ok {
			return i
		}
	}
	return -1
}

// getPlayerRoom returns the room containing the uid, or nil.
func (m *Manager) getPlayerRoom(uid int64) *Room {
	m.mu.RLock()
	defer m.mu.RUnlock()
	for _, room := range m.rooms {
		room.mu.Lock()
		for _, p := range room.Players {
			if p.UID == uid {
				room.mu.Unlock()
				return room
			}
		}
		room.mu.Unlock()
	}
	return nil
}

// getSpectatorRoom returns the room where uid is a spectator, or nil.
func (m *Manager) getSpectatorRoom(uid int64) *Room {
	m.mu.RLock()
	defer m.mu.RUnlock()
	for _, room := range m.rooms {
		room.mu.Lock()
		if room.Spectators[uid] {
			room.mu.Unlock()
			return room
		}
		room.mu.Unlock()
	}
	return nil
}

// removePlayerFromRoom removes uid from all seats and the spectator set of the
// given room. Called before seating a player in a new room to prevent the
// multi-room/multi-seat conflicts that crash the client when it receives
// conflicting BRC packets from two rooms at once.
func (m *Manager) removePlayerFromRoom(room *Room, uid int64) {
	room.mu.Lock()
	for seat, p := range room.Players {
		if p.UID == uid {
			delete(room.Players, seat)
		}
	}
	delete(room.Spectators, uid)
	isEmpty := len(room.Players) == 0
	room.mu.Unlock()

	if isEmpty {
		m.mu.Lock()
		delete(m.rooms, room.ID)
		m.mu.Unlock()
	}
}

// cleanupPlayerRooms removes the player from any room they're in (seated or
// spectating). Returns true if a room was found and cleaned up.
func (m *Manager) cleanupPlayerRooms(uid int64) bool {
	if room := m.getPlayerRoom(uid); room != nil {
		m.removePlayerFromRoom(room, uid)
		return true
	}
	if room := m.getSpectatorRoom(uid); room != nil {
		m.removePlayerFromRoom(room, uid)
		return true
	}
	return false
}

// getUser loads the lightweight profile needed to seat a player.
func (m *Manager) getUser(uid int64) (*UserLite, error) {
	var u UserLite
	err := m.db.QueryRowContext(context.Background(),
		`SELECT uid, name, gold, avatar, using_role_id, using_skin_id FROM users WHERE uid=$1`, uid).
		Scan(&u.UID, &u.Name, &u.Gold, &u.Avatar, &u.UsingRoleID, &u.UsingSkinID)
	if err != nil {
		return nil, err
	}
	return &u, nil
}

// UserLite is the minimal user fields needed to seat a player.
type UserLite struct {
	UID         int64
	Name        string
	Gold        int64
	Avatar      int
	UsingRoleID int
	UsingSkinID int
}
