package ws

import (
	"google.golang.org/protobuf/proto"

	"poker-fate-server/internal/proto/gen"
)

// RegisterSocialHandlers wires up social-related WS handlers (block-chat list, etc.).
func (s *Server) RegisterSocialHandlers() {
	s.RegisterHandler("pb.GetBlockChatListREQ", s.HandleGetBlockChatListREQ)
}

func (s *Server) HandleGetBlockChatListREQ(sess *Session, pkt *Packet) {
	rsp := &gen.GetBlockChatListRSP{
		List: []*gen.BlockChatItem{},
	}
	body, _ := proto.Marshal(rsp)
	sess.SendPacket("pb.GetBlockChatListRSP", pkt.RoomID, body)
}
