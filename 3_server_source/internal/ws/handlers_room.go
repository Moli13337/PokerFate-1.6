package ws

import (
	"google.golang.org/protobuf/proto"

	"poker-fate-server/internal/proto/gen"
)

// RegisterRoomHandlers wires up in-room chat, friend-room, and leave handlers.
// The private server runs no real multiplayer rooms; chat/friend-room stubs
// return success or empty lists so the client UI doesn't hang.
func (s *Server) RegisterRoomHandlers() {
	s.RegisterHandler("pb.DelayLeaveRoomREQ", s.HandleDelayLeaveRoomREQ)
	s.RegisterHandler("pb.FaceREQ", s.HandleFaceREQ)
	s.RegisterHandler("pb.TextREQ", s.HandleTextREQ)
	s.RegisterHandler("pb.CustomTextREQ", s.HandleCustomTextREQ)
	s.RegisterHandler("pb.SetBlockChatREQ", s.HandleSetBlockChatREQ)
	s.RegisterHandler("pb.CreateFriendRoomREQ", s.HandleCreateFriendRoomREQ)
	s.RegisterHandler("pb.JoinFriendRoomREQ", s.HandleJoinFriendRoomREQ)
	s.RegisterHandler("pb.FriendRoomListREQ", s.HandleFriendRoomListREQ)
	s.RegisterHandler("pb.InvitePlayREQ", s.HandleInvitePlayREQ)
	s.RegisterHandler("pb.GetFriendHisRecordREQ", s.HandleGetFriendHisRecordREQ)
	s.RegisterHandler("pb.GetScoreboardREQ", s.HandleGetScoreboardREQ)
}

func (s *Server) HandleDelayLeaveRoomREQ(sess *Session, pkt *Packet) {
	req := &gen.DelayLeaveRoomREQ{}
	if err := proto.Unmarshal(pkt.Body, req); err != nil {
		return
	}
	rsp := &gen.DelayLeaveRoomRSP{
		Code: proto.Int32(0),
		Flag: proto.Bool(req.GetFlag()),
	}
	sess.SendPacket("pb.DelayLeaveRoomRSP", pkt.RoomID, mustMarshal(rsp))
}

// HandleFaceREQ: FaceREQ has no RSP, only a FaceBRC broadcast. With no real
// table to broadcast to, this is a no-op.
func (s *Server) HandleFaceREQ(sess *Session, pkt *Packet) {}

func (s *Server) HandleTextREQ(sess *Session, pkt *Packet) {
	rsp := &gen.TextRSP{Code: proto.Int32(0)}
	sess.SendPacket("pb.TextRSP", pkt.RoomID, mustMarshal(rsp))
}

// HandleCustomTextREQ: CustomTextREQ has no RSP, only a CustomTextBRC broadcast.
func (s *Server) HandleCustomTextREQ(sess *Session, pkt *Packet) {}

func (s *Server) HandleSetBlockChatREQ(sess *Session, pkt *Packet) {
	req := &gen.SetBlockChatREQ{}
	if err := proto.Unmarshal(pkt.Body, req); err != nil {
		return
	}
	rsp := &gen.SetBlockChatRSP{
		Code:    proto.Int32(0),
		IsBlock: proto.Bool(req.GetIsBlock()),
		TheUid:  proto.Int64(req.GetTheUid()),
	}
	sess.SendPacket("pb.SetBlockChatRSP", pkt.RoomID, mustMarshal(rsp))
}

func (s *Server) HandleCreateFriendRoomREQ(sess *Session, pkt *Packet) {
	rsp := &gen.CreateFriendRoomRSP{Code: proto.Int32(-1)}
	sess.SendPacket("pb.CreateFriendRoomRSP", pkt.RoomID, mustMarshal(rsp))
}

// HandleJoinFriendRoomREQ: per proto comment, response is EnterRoomRSP.
func (s *Server) HandleJoinFriendRoomREQ(sess *Session, pkt *Packet) {
	rsp := &gen.EnterRoomRSP{
		Code:      proto.Int32(-1),
		Roomid:    proto.Int32(0),
		OldRoomId: proto.Int32(0),
		GameType:  proto.Int32(0),
	}
	sess.SendPacket("pb.EnterRoomRSP", pkt.RoomID, mustMarshal(rsp))
}

func (s *Server) HandleFriendRoomListREQ(sess *Session, pkt *Packet) {
	rsp := &gen.FriendRoomListRSP{
		RoomList: []*gen.FriendRoomItem{},
	}
	sess.SendPacket("pb.FriendRoomListRSP", pkt.RoomID, mustMarshal(rsp))
}

func (s *Server) HandleInvitePlayREQ(sess *Session, pkt *Packet) {
	req := &gen.InvitePlayREQ{}
	if err := proto.Unmarshal(pkt.Body, req); err != nil {
		return
	}
	rsp := &gen.InvitePlayRSP{
		Code:  proto.Int32(-1),
		ToUid: proto.Int64(req.GetToUid()),
	}
	sess.SendPacket("pb.InvitePlayRSP", pkt.RoomID, mustMarshal(rsp))
}

func (s *Server) HandleGetFriendHisRecordREQ(sess *Session, pkt *Packet) {
	rsp := &gen.GetFriendHisRecordRSP{
		Code: proto.Int32(0),
		List: []int32{},
	}
	sess.SendPacket("pb.GetFriendHisRecordRSP", pkt.RoomID, mustMarshal(rsp))
}

func (s *Server) HandleGetScoreboardREQ(sess *Session, pkt *Packet) {
	rsp := &gen.GetScoreboardRSP{
		Code: proto.Int32(0),
		List: []int32{},
	}
	sess.SendPacket("pb.GetScoreboardRSP", pkt.RoomID, mustMarshal(rsp))
}
