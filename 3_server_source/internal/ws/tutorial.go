package ws

import (
	"context"
	"time"

	"go.uber.org/zap"
	"google.golang.org/protobuf/proto"

	"poker-fate-server/internal/proto/gen"
)

// RegisterTutorialHandlers covers non-game messages that the tutorial still
// routes to the real server. Poker game logic during the tutorial is simulated
// client-side by GameServer.lua (see LobbyByinDialog -> Net:setRecver).
func (s *Server) RegisterTutorialHandlers() {
	s.RegisterHandler("pb.SetClientDefStrREQ", s.HandleSetClientDefStrREQ)
	s.RegisterHandler("pb.GetServerTimeREQ", s.HandleGetServerTimeREQ)
	s.RegisterHandler("pb.SetTableFlagREQ", s.HandleSetTableFlagREQ)
	s.RegisterHandler("pb.TableInfoREQ", s.HandleTableInfoREQ)
	s.RegisterHandler("pb.GetRoomDataREQ", s.HandleGetRoomDataREQ)
	s.RegisterHandler("pb.RoomWinnerRewardsInfoREQ", s.HandleRoomWinnerRewardsInfoREQ)
	s.RegisterHandler("pb.GetUserLevelInfoREQ", s.HandleGetUserLevelInfoREQ)
	s.RegisterHandler("pb.GetUserBondInfoREQ", s.HandleGetUserBondInfoREQ)
	s.RegisterHandler("pb.UnlockNewModuleREQ", s.HandleUnlockNewModuleREQ)
	s.RegisterHandler("pb.GetAllinGameTimeREQ", s.HandleGetAllinGameTimeREQ)
	s.RegisterHandler("pb.QuickStartREQ", s.HandleQuickStartREQ)
	s.RegisterHandler("pb.EnterRoomREQ", s.HandleEnterRoomREQ)
	s.RegisterHandler("pb.LeaveRoomREQ", s.HandleLeaveRoomREQ)
	s.RegisterHandler("pb.SitDownREQ", s.HandleSitDownREQ)
	s.RegisterHandler("pb.StandUpREQ", s.HandleStandUpREQ)
	s.RegisterHandler("pb.SelfAchievementsREQ", s.HandleSelfAchievementsREQ)
	s.RegisterHandler("pb.GetOtherDetailInfoREQ", s.HandleGetOtherDetailInfoREQ)
	s.RegisterHandler("pb.ReserveSeatREQ", s.HandleReserveSeatREQ)
	s.RegisterHandler("pb.ChangeObserveViewREQ", s.HandleChangeObserveViewREQ)
}

func (s *Server) HandleSetClientDefStrREQ(sess *Session, pkt *Packet) {
	req := &gen.SetClientDefStrREQ{}
	if err := proto.Unmarshal(pkt.Body, req); err != nil {
		return
	}
	if sess.UID != 0 && req.GetClientDefStr() != "" {
		if _, err := s.DB.Exec(
			`UPDATE users SET client_def_str=$1 WHERE uid=$2`,
			req.GetClientDefStr(), sess.UID,
		); err != nil {
			s.Logger.Warn("update client_def_str failed", zap.Error(err))
		}
	}
	sess.SendPacket("pb.SetClientDefStrRSP", pkt.RoomID, mustMarshal(&gen.SetClientDefStrRSP{Code: proto.Int32(0)}))
}

func (s *Server) HandleGetServerTimeREQ(sess *Session, pkt *Packet) {
	rsp := &gen.GetServerTimeRSP{ServerTimestamp: proto.Int32(int32(time.Now().Unix()))}
	sess.SendPacket("pb.GetServerTimeRSP", pkt.RoomID, mustMarshal(rsp))
}

func (s *Server) HandleSetTableFlagREQ(sess *Session, pkt *Packet) {
	sess.SendPacket("pb.SetTableFlagRSP", pkt.RoomID, mustMarshal(&gen.SetTableFlagRSP{Code: proto.Int32(0)}))
}

func (s *Server) HandleTableInfoREQ(sess *Session, pkt *Packet) {
	sess.SendPacket("pb.TableInfoRSP", pkt.RoomID, mustMarshal(&gen.TableInfoRSP{}))
}

// GetRoomDataREQ during tutorial returns code=-1 so the client falls back to lobby.
func (s *Server) HandleGetRoomDataREQ(sess *Session, pkt *Packet) {
	rsp := &gen.GetRoomDataRSP{Code: proto.Int32(-1), Roomid: proto.Int32(0)}
	sess.SendPacket("pb.GetRoomDataRSP", pkt.RoomID, mustMarshal(rsp))
}

func (s *Server) HandleRoomWinnerRewardsInfoREQ(sess *Session, pkt *Packet) {
	req := &gen.RoomWinnerRewardsInfoREQ{}
	if err := proto.Unmarshal(pkt.Body, req); err != nil {
		return
	}
	rsp := &gen.RoomWinnerRewardsInfoRSP{
		Code:       proto.Int32(0),
		GameType:   req.GameType,
		RewardList: []*gen.RoomWinnerRewardsItem{},
	}
	sess.SendPacket("pb.RoomWinnerRewardsInfoRSP", pkt.RoomID, mustMarshal(rsp))
}

func (s *Server) HandleGetUserLevelInfoREQ(sess *Session, pkt *Packet) {
	rsp := &gen.GetUserLevelInfoRSP{
		Code:      proto.Int32(0),
		Hands:     proto.Int32(0),
		ResetTime: proto.Int32(int32(time.Now().Unix() + 86400)),
		SngHands:  proto.Int32(0),
		MttHands:  proto.Int32(0),
	}
	sess.SendPacket("pb.GetUserLevelInfoRSP", pkt.RoomID, mustMarshal(rsp))
}

func (s *Server) HandleGetUserBondInfoREQ(sess *Session, pkt *Packet) {
	rsp := &gen.GetUserBondInfoRSP{
		Code:     proto.Int32(0),
		Hands:    proto.Int32(0),
		SngHands: proto.Int32(0),
		MttHands: proto.Int32(0),
	}
	sess.SendPacket("pb.GetUserBondInfoRSP", pkt.RoomID, mustMarshal(rsp))
}

func (s *Server) HandleUnlockNewModuleREQ(sess *Session, pkt *Packet) {
	req := &gen.UnlockNewModuleREQ{}
	if err := proto.Unmarshal(pkt.Body, req); err != nil {
		return
	}
	rsp := &gen.UnlockNewModuleRSP{
		SuccessIds: req.GetIds(),
		FailIds:    []int32{},
	}
	sess.SendPacket("pb.UnlockNewModuleRSP", pkt.RoomID, mustMarshal(rsp))
}

func (s *Server) HandleGetAllinGameTimeREQ(sess *Session, pkt *Packet) {
	now := int32(time.Now().Unix())
	rsp := &gen.GetAllinGameTimeRSP{
		StartTime: proto.Int32(now),
		EndTime:   proto.Int32(now + 86400*365),
	}
	sess.SendPacket("pb.GetAllinGameTimeRSP", pkt.RoomID, mustMarshal(rsp))
}

// QuickStartREQ is intercepted by GameServer.lua during the tutorial.
// This handler only serves the post-tutorial flow and returns code=-1
// to redirect the client back to lobby until real rooms are implemented.
func (s *Server) HandleQuickStartREQ(sess *Session, pkt *Packet) {
	req := &gen.QuickStartREQ{}
	if err := proto.Unmarshal(pkt.Body, req); err != nil {
		return
	}
	s.Logger.Info("QuickStartREQ",
		zap.Int64("uid", sess.UID),
		zap.Int32("game_type", req.GetGameType()),
		zap.Int64("boot", req.GetBoot()),
	)
	rsp := &gen.QuickStartRSP{
		Code:      proto.Int32(-1),
		GameType:  req.GameType,
		Boot:      req.Boot,
		LobbyCoin: req.LobbyCoin,
	}
	sess.SendPacket("pb.QuickStartRSP", pkt.RoomID, mustMarshal(rsp))
}

func (s *Server) HandleEnterRoomREQ(sess *Session, pkt *Packet) {
	req := &gen.EnterRoomREQ{}
	if err := proto.Unmarshal(pkt.Body, req); err != nil {
		return
	}
	s.Logger.Info("EnterRoomREQ",
		zap.Int64("uid", sess.UID),
		zap.Int32("roomid", req.GetRoomid()),
	)
	rsp := &gen.EnterRoomRSP{
		Code:      proto.Int32(-1),
		Roomid:    proto.Int32(0),
		OldRoomId: proto.Int32(0),
		GameType:  proto.Int32(0),
	}
	sess.SendPacket("pb.EnterRoomRSP", pkt.RoomID, mustMarshal(rsp))
}

func (s *Server) HandleLeaveRoomREQ(sess *Session, pkt *Packet) {
	rsp := &gen.LeaveRoomRSP{Code: proto.Int32(0), Roomid: proto.Int32(0)}
	sess.SendPacket("pb.LeaveRoomRSP", pkt.RoomID, mustMarshal(rsp))
}

func (s *Server) HandleSitDownREQ(sess *Session, pkt *Packet) {
	sess.SendPacket("pb.SitDownRSP", pkt.RoomID, mustMarshal(&gen.SitDownRSP{Code: proto.Int32(-1)}))
}

func (s *Server) HandleStandUpREQ(sess *Session, pkt *Packet) {
	sess.SendPacket("pb.StandUpRSP", pkt.RoomID, mustMarshal(&gen.StandUpRSP{Code: proto.Int32(0)}))
}

func (s *Server) HandleSelfAchievementsREQ(sess *Session, pkt *Packet) {
	sess.SendPacket("pb.SelfAchievementsRSP", pkt.RoomID, mustMarshal(&gen.SelfAchievementsRSP{}))
}

func (s *Server) HandleGetOtherDetailInfoREQ(sess *Session, pkt *Packet) {
	req := &gen.GetOtherDetailInfoREQ{}
	if err := proto.Unmarshal(pkt.Body, req); err != nil {
		return
	}
	targetUID := req.GetTheUid()
	if targetUID == 0 {
		sess.SendPacket("pb.GetOtherDetailInfoRSP", pkt.RoomID, mustMarshal(&gen.GetOtherDetailInfoRSP{Code: proto.Int32(0)}))
		return
	}

	var name, authCertURL string
	var registerTime, authCertTime int64
	_ = s.DB.QueryRowContext(context.Background(),
		`SELECT name, EXTRACT(EPOCH FROM register_time)::BIGINT, auth_cert_url, auth_cert_time
		 FROM users WHERE uid=$1 AND is_deleted=false`, targetUID,
	).Scan(&name, &registerTime, &authCertURL, &authCertTime)

	rsp := &gen.GetOtherDetailInfoRSP{
		Code:          proto.Int32(0),
		Brief:         &gen.UserBrief{Uid: proto.Int64(targetUID), Name: proto.String(name)},
		AuthCertUrl:   proto.String(authCertURL),
		RegisterTime:  proto.Int64(registerTime),
		AuthCertTime:  proto.Int64(authCertTime),
		FavoriteRoles: s.loadFavoriteRoles(targetUID),
	}
	sess.SendPacket("pb.GetOtherDetailInfoRSP", pkt.RoomID, mustMarshal(rsp))
}

func (s *Server) HandleReserveSeatREQ(sess *Session, pkt *Packet) {
	req := &gen.ReserveSeatREQ{}
	if err := proto.Unmarshal(pkt.Body, req); err != nil {
		return
	}
	rsp := &gen.ReserveSeatRSP{Code: proto.Int32(0), Reserve: req.Reserve}
	sess.SendPacket("pb.ReserveSeatRSP", pkt.RoomID, mustMarshal(rsp))
}

func (s *Server) HandleChangeObserveViewREQ(sess *Session, pkt *Packet) {
	sess.SendPacket("pb.ChangeObserveViewRSP", pkt.RoomID, mustMarshal(&gen.ChangeObserveViewRSP{Code: proto.Int32(-1)}))
}

func mustMarshal(m proto.Message) []byte {
	b, err := proto.Marshal(m)
	if err != nil {
		return nil
	}
	return b
}
