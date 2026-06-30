package ws

import (
	"google.golang.org/protobuf/proto"

	"poker-fate-server/internal/proto/gen"
)

// RegisterTournamentHandlers wires up SNG/MTT tournament handlers.
// The private server hosts no live tournaments; all handlers return empty
// lists or code=-1 so the client UI renders an empty tournament lobby
// instead of hanging on a missing response.
func (s *Server) RegisterTournamentHandlers() {
	s.RegisterHandler("pb.TourListREQ", s.HandleTourListREQ)
	s.RegisterHandler("pb.TourHistoryListREQ", s.HandleTourHistoryListREQ)
	s.RegisterHandler("pb.TourDetailInfoREQ", s.HandleTourDetailInfoREQ)
	s.RegisterHandler("pb.SngSignREQ", s.HandleSngSignREQ)
	s.RegisterHandler("pb.TourRoomDetailREQ", s.HandleTourRoomDetailREQ)
	s.RegisterHandler("pb.MttRankREQ", s.HandleMttRankREQ)
	s.RegisterHandler("pb.MttRankRewardREQ", s.HandleMttRankRewardREQ)
	s.RegisterHandler("pb.MttPotRewardStructureREQ", s.HandleMttPotRewardStructureREQ)
	s.RegisterHandler("pb.MttSignREQ", s.HandleMttSignREQ)
	s.RegisterHandler("pb.MttCancelSignREQ", s.HandleMttCancelSignREQ)
	s.RegisterHandler("pb.MttRoomDetailREQ", s.HandleMttRoomDetailREQ)
	s.RegisterHandler("pb.GetMttRoomREQ", s.HandleGetMttRoomREQ)
	s.RegisterHandler("pb.GetMttTourBriefInfoREQ", s.HandleGetMttTourBriefInfoREQ)
	s.RegisterHandler("pb.SngCancelSignREQ", s.HandleSngCancelSignREQ)
}

func (s *Server) HandleTourListREQ(sess *Session, pkt *Packet) {
	req := &gen.TourListREQ{}
	if err := proto.Unmarshal(pkt.Body, req); err != nil {
		return
	}
	rsp := &gen.TourListRSP{
		Code:    proto.Int32(0),
		ReqType: proto.Int32(req.GetReqType()),
		SngList: []*gen.SngListItem{},
		MttList: []*gen.MttListItem{},
	}
	sess.SendPacket("pb.TourListRSP", pkt.RoomID, mustMarshal(rsp))
}

func (s *Server) HandleTourHistoryListREQ(sess *Session, pkt *Packet) {
	req := &gen.TourHistoryListREQ{}
	if err := proto.Unmarshal(pkt.Body, req); err != nil {
		return
	}
	rsp := &gen.TourHistoryListRSP{
		Code:           proto.Int32(0),
		ReqType:        proto.Int32(req.GetReqType()),
		HistorySngList: []*gen.SngListItem{},
		HistoryMttList: []*gen.MttListItem{},
	}
	sess.SendPacket("pb.TourHistoryListRSP", pkt.RoomID, mustMarshal(rsp))
}

func (s *Server) HandleTourDetailInfoREQ(sess *Session, pkt *Packet) {
	rsp := &gen.TourDetailInfoRSP{Code: proto.Int32(0)}
	sess.SendPacket("pb.TourDetailInfoRSP", pkt.RoomID, mustMarshal(rsp))
}

func (s *Server) HandleSngSignREQ(sess *Session, pkt *Packet) {
	req := &gen.SngSignREQ{}
	if err := proto.Unmarshal(pkt.Body, req); err != nil {
		return
	}
	rsp := &gen.SngSignRSP{
		Code:   proto.Int32(-1),
		TourId: proto.Int32(req.GetTourId()),
	}
	sess.SendPacket("pb.SngSignRSP", pkt.RoomID, mustMarshal(rsp))
}

func (s *Server) HandleTourRoomDetailREQ(sess *Session, pkt *Packet) {
	rsp := &gen.TourRoomDetailRSP{
		Code:     proto.Int32(-1),
		RankList: []*gen.TourRankItem{},
	}
	sess.SendPacket("pb.TourRoomDetailRSP", pkt.RoomID, mustMarshal(rsp))
}

func (s *Server) HandleMttRankREQ(sess *Session, pkt *Packet) {
	rsp := &gen.MttRankRSP{
		Code:     proto.Int32(0),
		RankList: []*gen.TourRankItem{},
	}
	sess.SendPacket("pb.MttRankRSP", pkt.RoomID, mustMarshal(rsp))
}

func (s *Server) HandleMttRankRewardREQ(sess *Session, pkt *Packet) {
	rsp := &gen.MttRankRewardRSP{
		Code:           proto.Int32(0),
		RankRewardList: []*gen.MttRankRewardItem{},
	}
	sess.SendPacket("pb.MttRankRewardRSP", pkt.RoomID, mustMarshal(rsp))
}

func (s *Server) HandleMttPotRewardStructureREQ(sess *Session, pkt *Packet) {
	rsp := &gen.MttPotRewardStructureRSP{
		Code:            proto.Int32(0),
		RewardStructure: []*gen.MttPotRewardStructureItem{},
	}
	sess.SendPacket("pb.MttPotRewardStructureRSP", pkt.RoomID, mustMarshal(rsp))
}

func (s *Server) HandleMttSignREQ(sess *Session, pkt *Packet) {
	req := &gen.MttSignREQ{}
	if err := proto.Unmarshal(pkt.Body, req); err != nil {
		return
	}
	rsp := &gen.MttSignRSP{
		Code:   proto.Int32(-1),
		TourId: proto.Int32(req.GetTourId()),
	}
	sess.SendPacket("pb.MttSignRSP", pkt.RoomID, mustMarshal(rsp))
}

func (s *Server) HandleMttCancelSignREQ(sess *Session, pkt *Packet) {
	req := &gen.MttCancelSignREQ{}
	if err := proto.Unmarshal(pkt.Body, req); err != nil {
		return
	}
	rsp := &gen.MttCancelSignRSP{
		Code:   proto.Int32(0),
		TourId: proto.Int32(req.GetTourId()),
	}
	sess.SendPacket("pb.MttCancelSignRSP", pkt.RoomID, mustMarshal(rsp))
}

func (s *Server) HandleMttRoomDetailREQ(sess *Session, pkt *Packet) {
	rsp := &gen.MttRoomDetailRSP{
		Code:      proto.Int32(-1),
		TableList: []*gen.MttTableSheet{},
	}
	sess.SendPacket("pb.MttRoomDetailRSP", pkt.RoomID, mustMarshal(rsp))
}

func (s *Server) HandleGetMttRoomREQ(sess *Session, pkt *Packet) {
	rsp := &gen.GetMttRoomRSP{
		MttRoomList: []*gen.MttRoomItem{},
	}
	sess.SendPacket("pb.GetMttRoomRSP", pkt.RoomID, mustMarshal(rsp))
}

func (s *Server) HandleGetMttTourBriefInfoREQ(sess *Session, pkt *Packet) {
	rsp := &gen.GetMttTourBriefInfoRSP{
		Code:   proto.Int32(0),
		Briefs: []*gen.MttTourBrief{},
	}
	sess.SendPacket("pb.GetMttTourBriefInfoRSP", pkt.RoomID, mustMarshal(rsp))
}

func (s *Server) HandleSngCancelSignREQ(sess *Session, pkt *Packet) {
	req := &gen.SngCancelSignREQ{}
	if err := proto.Unmarshal(pkt.Body, req); err != nil {
		return
	}
	rsp := &gen.SngCancelSignRSP{
		Code:   proto.Int32(0),
		TourId: proto.Int32(req.GetTourId()),
	}
	sess.SendPacket("pb.SngCancelSignRSP", pkt.RoomID, mustMarshal(rsp))
}
