package ws

import (
	"google.golang.org/protobuf/proto"

	"poker-fate-server/internal/proto/gen"
)

// RegisterBustProtectHandlers wires up the bankruptcy-protection WS handlers.
// (formerly RegisterMiscHandlers in handlers_misc.go)
func (s *Server) RegisterBustProtectHandlers() {
	s.RegisterHandler("pb.BustProtectInfoREQ", s.HandleBustProtectInfoREQ)
	s.RegisterHandler("pb.BustProtectRewardREQ", s.HandleBustProtectRewardREQ)
}

// HandleBustProtectInfoREQ returns the bankruptcy-protection status. The
// private server grants unlimited protection with no claimable reward so the
// IngameBankrupt popup never appears.
func (s *Server) HandleBustProtectInfoREQ(sess *Session, pkt *Packet) {
	rsp := &gen.BustProtectInfoRSP{
		Code:        proto.Int32(0),
		LeftTimes:   proto.Int32(999),
		RewardChips: proto.Int64(0),
	}
	sess.SendPacket("pb.BustProtectInfoRSP", pkt.RoomID, mustMarshal(rsp))
}

// HandleBustProtectRewardREQ acknowledges a reward claim with zero payout.
func (s *Server) HandleBustProtectRewardREQ(sess *Session, pkt *Packet) {
	rsp := &gen.BustProtectRewardRSP{Code: proto.Int32(0), RewardChips: proto.Int64(0)}
	sess.SendPacket("pb.BustProtectRewardRSP", pkt.RoomID, mustMarshal(rsp))
}
