package ws

import (
	"google.golang.org/protobuf/proto"

	"poker-fate-server/internal/proto/gen"
)

// RegisterPokerHandlers wires up in-game poker (CSHoldem) handlers.
// The private server skips real poker AI; EnterRoom returns -1 so the client
// never enters a real hand. These stubs exist solely to prevent "unhandled
// packet type" logs if the client sends poker packets during edge cases
// (e.g. tutorial transitions).
func (s *Server) RegisterPokerHandlers() {
	s.RegisterHandler("pb.PreActionREQ", s.HandlePreActionREQ)
	s.RegisterHandler("pb.ShowMyCardREQ", s.HandleShowMyCardREQ)
	s.RegisterHandler("pb.CancelWaitBlindREQ", s.HandleCancelWaitBlindREQ)
	s.RegisterHandler("pb.SetWaitBlindTypeREQ", s.HandleSetWaitBlindTypeREQ)
	s.RegisterHandler("pb.RebyREQ", s.HandleRebyREQ)
	s.RegisterHandler("pb.SetRebyREQ", s.HandleSetRebyREQ)
	s.RegisterHandler("pb.ActionREQ", s.HandleActionREQ)
	s.RegisterHandler("pb.GetCardsREQ", s.HandleGetCardsREQ)
	s.RegisterHandler("pb.GetHandsListREQ", s.HandleGetHandsListREQ)
	s.RegisterHandler("pb.RoundStartDisplayFinishREQ", s.HandleRoundStartDisplayFinishREQ)
	s.RegisterHandler("pb.SendEmojiREQ", s.HandleSendEmojiREQ)
}

func (s *Server) HandlePreActionREQ(sess *Session, pkt *Packet) {
	rsp := &gen.PreActionRSP{Code: proto.Int32(-1)}
	sess.SendPacket("pb.PreActionRSP", pkt.RoomID, mustMarshal(rsp))
}

func (s *Server) HandleShowMyCardREQ(sess *Session, pkt *Packet) {
	rsp := &gen.ShowMyCardRSP{Code: proto.Int32(-1)}
	sess.SendPacket("pb.ShowMyCardRSP", pkt.RoomID, mustMarshal(rsp))
}

// HandleCancelWaitBlindREQ: deprecated in proto, no RSP defined. No-op.
func (s *Server) HandleCancelWaitBlindREQ(sess *Session, pkt *Packet) {}

func (s *Server) HandleSetWaitBlindTypeREQ(sess *Session, pkt *Packet) {
	rsp := &gen.SetWaitBlindTypeRSP{Code: proto.Int32(-1)}
	sess.SendPacket("pb.SetWaitBlindTypeRSP", pkt.RoomID, mustMarshal(rsp))
}

// HandleRebyREQ: no RSP defined in proto (only RebyBRC). No-op.
func (s *Server) HandleRebyREQ(sess *Session, pkt *Packet) {}

func (s *Server) HandleSetRebyREQ(sess *Session, pkt *Packet) {
	rsp := &gen.SetRebyRSP{Code: proto.Int32(-1)}
	sess.SendPacket("pb.SetRebyRSP", pkt.RoomID, mustMarshal(rsp))
}

func (s *Server) HandleActionREQ(sess *Session, pkt *Packet) {
	rsp := &gen.ActionRSP{Code: proto.Int32(-1)}
	sess.SendPacket("pb.ActionRSP", pkt.RoomID, mustMarshal(rsp))
}

func (s *Server) HandleGetCardsREQ(sess *Session, pkt *Packet) {
	rsp := &gen.GetCardsRSP{
		BoardCards: []int32{},
	}
	sess.SendPacket("pb.GetCardsRSP", pkt.RoomID, mustMarshal(rsp))
}

func (s *Server) HandleGetHandsListREQ(sess *Session, pkt *Packet) {
	rsp := &gen.GetHandsListRSP{Code: proto.Int32(-1)}
	sess.SendPacket("pb.GetHandsListRSP", pkt.RoomID, mustMarshal(rsp))
}

func (s *Server) HandleRoundStartDisplayFinishREQ(sess *Session, pkt *Packet) {
	rsp := &gen.RoundStartDisplayFinishRSP{Code: proto.Int32(0)}
	sess.SendPacket("pb.RoundStartDisplayFinishRSP", pkt.RoomID, mustMarshal(rsp))
}

func (s *Server) HandleSendEmojiREQ(sess *Session, pkt *Packet) {
	rsp := &gen.SendEmojiRSP{
		Code:           proto.Int32(0),
		EmojiFreeTimes: proto.Int32(0),
	}
	sess.SendPacket("pb.SendEmojiRSP", pkt.RoomID, mustMarshal(rsp))
}
