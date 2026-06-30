package ws

import (
	"time"

	"poker-fate-server/internal/proto/gen"

	"google.golang.org/protobuf/proto"
)

// RegisterHeartBeatHandlers wires up the heartbeat handler.
func (s *Server) RegisterHeartBeatHandlers() {
	s.RegisterHandler("pb.HeartBeatREQ", s.HandleHeartBeatREQ)
}

func (s *Server) HandleHeartBeatREQ(sess *Session, pkt *Packet) {
	rsp := &gen.HeartBeatRSP{
		ServerTimestamp: proto.Int32(int32(time.Now().Unix())),
	}
	sess.SendPacket("pb.HeartBeatRSP", pkt.RoomID, mustMarshal(rsp))
}
