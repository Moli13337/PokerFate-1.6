package ws

import (
	"google.golang.org/protobuf/proto"

	"poker-fate-server/internal/proto/gen"
)

// RegisterSideGameHandlers wires up the side-game (push gift / color game /
// pinball) WS handlers.
func (s *Server) RegisterSideGameHandlers() {
	s.RegisterHandler("pb.PushGiftREQ", s.HandlePushGiftREQ)
	s.RegisterHandler("pb.GetSideGameConfREQ", s.HandleGetSideGameConfREQ)
	s.RegisterHandler("pb.GetSideGameHisRecordREQ", s.HandleGetSideGameHisRecordREQ)
	s.RegisterHandler("pb.ColorGameActionREQ", s.HandleColorGameActionREQ)
	s.RegisterHandler("pb.GetColorGameConfREQ", s.HandleGetColorGameConfREQ)
	s.RegisterHandler("pb.PinballActionREQ", s.HandlePinballActionREQ)
}

// --- Side games ---

// HandlePushGiftREQ returns code=0 with no active gift so the client does not
// push a gift popup during normal play.
func (s *Server) HandlePushGiftREQ(sess *Session, pkt *Packet) {
	req := &gen.PushGiftREQ{}
	if err := proto.Unmarshal(pkt.Body, req); err != nil {
		return
	}
	rsp := &gen.PushGiftRSP{Code: proto.Int32(0), GameType: req.GameType}
	sess.SendPacket("pb.PushGiftRSP", pkt.RoomID, mustMarshal(rsp))
}

func (s *Server) HandleGetSideGameConfREQ(sess *Session, pkt *Packet) {
	req := &gen.GetSideGameConfREQ{}
	if err := proto.Unmarshal(pkt.Body, req); err != nil {
		return
	}
	rsp := &gen.GetSideGameConfRSP{Code: proto.Int32(0), GameType: req.GameType, Conf: proto.String("{}")}
	sess.SendPacket("pb.GetSideGameConfRSP", pkt.RoomID, mustMarshal(rsp))
}

func (s *Server) HandleGetSideGameHisRecordREQ(sess *Session, pkt *Packet) {
	req := &gen.GetSideGameHisRecordREQ{}
	if err := proto.Unmarshal(pkt.Body, req); err != nil {
		return
	}
	rsp := &gen.GetSideGameHisRecordRSP{Code: proto.Int32(0), GameType: req.GameType}
	sess.SendPacket("pb.GetSideGameHisRecordRSP", pkt.RoomID, mustMarshal(rsp))
}

func (s *Server) HandleColorGameActionREQ(sess *Session, pkt *Packet) {
	req := &gen.ColorGameActionREQ{}
	if err := proto.Unmarshal(pkt.Body, req); err != nil {
		return
	}
	rsp := &gen.ColorGameActionRSP{Code: proto.Int32(-1)}
	sess.SendPacket("pb.ColorGameActionRSP", pkt.RoomID, mustMarshal(rsp))
}

func (s *Server) HandleGetColorGameConfREQ(sess *Session, pkt *Packet) {
	rsp := &gen.GetColorGameConfRSP{Code: proto.Int32(0), Conf: proto.String("{}")}
	sess.SendPacket("pb.GetColorGameConfRSP", pkt.RoomID, mustMarshal(rsp))
}

func (s *Server) HandlePinballActionREQ(sess *Session, pkt *Packet) {
	rsp := &gen.PinballActionRSP{Code: proto.Int32(-1)}
	sess.SendPacket("pb.PinballActionRSP", pkt.RoomID, mustMarshal(rsp))
}
