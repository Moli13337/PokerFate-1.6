package ws

import (
	"google.golang.org/protobuf/proto"

	"poker-fate-server/internal/proto/gen"
)

// RegisterLobbyHandlers wires up lobby-level handlers (jackpot UI).
func (s *Server) RegisterLobbyHandlers() {
	s.RegisterHandler("pb.LobJackPotREQ", s.HandleLobJackPotREQ)
	s.RegisterHandler("pb.LobJackPotConfigREQ", s.HandleLobJackPotConfigREQ)
}

// HandleLobJackPotREQ returns the current lobby jackpot pool. Private server
// runs no real jackpot, so the pool is always 0.
func (s *Server) HandleLobJackPotREQ(sess *Session, pkt *Packet) {
	req := &gen.LobJackPotREQ{}
	if err := proto.Unmarshal(pkt.Body, req); err != nil {
		return
	}
	rsp := &gen.LobJackPotRSP{
		Blind:        proto.Int64(req.GetBlind()),
		JackpotChips: proto.Int64(0),
		GameType:     proto.Int32(req.GetGameType()),
	}
	sess.SendPacket("pb.LobJackPotRSP", pkt.RoomID, mustMarshal(rsp))
}

// HandleLobJackPotConfigREQ returns the jackpot reward rate config. Empty list
// means no jackpot reward tiers are configured.
func (s *Server) HandleLobJackPotConfigREQ(sess *Session, pkt *Packet) {
	req := &gen.LobJackPotConfigREQ{}
	if err := proto.Unmarshal(pkt.Body, req); err != nil {
		return
	}
	rsp := &gen.LobJackPotConfigRSP{
		Blind:    proto.Int64(req.GetBlind()),
		Rate:     []int32{},
		GameType: proto.Int32(req.GetGameType()),
	}
	sess.SendPacket("pb.LobJackPotConfigRSP", pkt.RoomID, mustMarshal(rsp))
}
