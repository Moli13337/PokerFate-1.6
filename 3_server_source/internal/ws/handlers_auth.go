package ws

import (
	"context"
	"fmt"
	"time"

	"go.uber.org/zap"
	"google.golang.org/protobuf/proto"

	"poker-fate-server/internal/proto/gen"
)

// RegisterAuthHandlers wires up login / logout / login-queue WS handlers.
func (s *Server) RegisterAuthHandlers() {
	s.RegisterHandler("pb.UserLoginREQ", s.HandleUserLoginREQ)
	s.RegisterHandler("pb.UserLogoutREQ", s.HandleUserLogoutREQ)
	s.RegisterHandler("pb.CancelLoginQueueREQ", s.HandleCancelLoginQueueREQ)
}

func (s *Server) HandleUserLoginREQ(sess *Session, pkt *Packet) {
	req := &gen.UserLoginREQ{}
	if err := proto.Unmarshal(pkt.Body, req); err != nil {
		s.Logger.Warn("unmarshal UserLoginREQ failed", zap.Error(err))
		return
	}

	uid := req.GetUid()
	key := req.GetKey()

	rdkey, err := s.Redis.Get(context.Background(), fmt.Sprintf("rdkey:%d", uid)).Result()
	if err != nil || rdkey != key {
		rsp := &gen.UserLoginRSP{Code: proto.Int32(-1), Reason: proto.String("invalid key")}
		body, _ := proto.Marshal(rsp)
		sess.SendPacket("pb.UserLoginRSP", pkt.RoomID, body)
		return
	}

	sess.UID = uid
	s.AddSession(uid, sess)

	s.Redis.Del(context.Background(), fmt.Sprintf("rdkey:%d", uid))

	rsp := &gen.UserLoginRSP{
		Code:            proto.Int32(0),
		ServerTimestamp: proto.Int32(int32(time.Now().Unix())),
	}
	body, _ := proto.Marshal(rsp)
	sess.SendPacket("pb.UserLoginRSP", pkt.RoomID, body)
}

func (s *Server) HandleUserLogoutREQ(sess *Session, pkt *Packet) {
	rsp := &gen.UserLogoutRSP{Code: proto.Int32(0)}
	body, _ := proto.Marshal(rsp)
	sess.SendPacket("pb.UserLogoutRSP", pkt.RoomID, body)
	sess.Close()
}

func (s *Server) HandleCancelLoginQueueREQ(sess *Session, pkt *Packet) {
	rsp := &gen.CancelLoginQueueRSP{}
	body, _ := proto.Marshal(rsp)
	sess.SendPacket("pb.CancelLoginQueueRSP", pkt.RoomID, body)
}
