package ws

import (
	"context"
	"database/sql"
	"encoding/json"

	"github.com/lib/pq"
	"go.uber.org/zap"
	"google.golang.org/protobuf/proto"

	"poker-fate-server/internal/gamedata"
	"poker-fate-server/internal/model"
	"poker-fate-server/internal/proto/gen"
)

// RegisterUserHandlers wires up user-info / user-value / guide-step / SNG match WS handlers.
func (s *Server) RegisterUserHandlers() {
	s.RegisterHandler("pb.SelfUserInfoREQ", s.HandleSelfUserInfoREQ)
	s.RegisterHandler("pb.UserValueREQ", s.HandleUserValueREQ)
	s.RegisterHandler("pb.SngMatchPlayerNumREQ", s.HandleSngMatchPlayerNumREQ)
	s.RegisterHandler("pb.SetNewerGuideStepREQ", s.HandleSetNewerGuideStepREQ)
}

func (s *Server) HandleSelfUserInfoREQ(sess *Session, pkt *Packet) {
	if sess.UID == 0 {
		return
	}

	user, err := s.getUserByUID(sess.UID)
	if err != nil {
		s.Logger.Warn("get user failed", zap.Int64("uid", sess.UID), zap.Error(err))
		return
	}

	roleID := int32(user.UsingRoleID)
	if roleID == 0 {
		roleID = defaultRoleID()
	}
	skinID := int32(user.UsingSkinID)
	if skinID == 0 || skinID == 1 {
		skinID = defaultSkinID()
	}

	rsp := &gen.SelfUserInfoRSP{
		Brief: &gen.UserBrief{
			Uid:    proto.Int64(user.UID),
			Name:   proto.String(user.Name),
			Avatar: proto.Int32(int32(user.Avatar)),
			Frame:  proto.Int32(int32(user.Frame)),
			Title:  proto.Int32(int32(user.Title)),
			Level:  proto.Int32(int32(user.Level)),
			RoleId: proto.Int32(roleID),
			SkinId: proto.Int32(skinID),
		},
		Exp:            proto.Int32(int32(user.Exp)),
		VipLevel:       proto.Int32(int32(user.VipLevel)),
		NewerGuideStep: proto.Int32(int32(user.NewerGuideStep)),
		ClientDefStr:   proto.String(user.ClientDefStr),
		RegisterTime:   proto.Int64(user.RegisterTime.Unix()),
		UnlockModules:  []int32{1, 2, 3, 4, 5, 6, 7, 8, 9},
		AuthCertUrl:    proto.String(user.AuthCertURL),
		AuthCertTime:   proto.Int64(user.AuthCertTime),
		MonthlyCardExp: proto.Int32(int32(user.MonthlyCardExp)),
		FavoriteRoles:  s.loadFavoriteRoles(user.UID),
	}
	body, _ := proto.Marshal(rsp)
	sess.SendPacket("pb.SelfUserInfoRSP", pkt.RoomID, body)
}

func (s *Server) HandleUserValueREQ(sess *Session, pkt *Packet) {
	if sess.UID == 0 {
		return
	}

	req := &gen.UserValueREQ{}
	if err := proto.Unmarshal(pkt.Body, req); err != nil {
		return
	}

	user, err := s.getUserByUID(sess.UID)
	if err != nil {
		return
	}

	rsp := &gen.UserValueRSP{
		Type:  req.Type,
		Value: proto.Int64(user.Gold),
	}
	body, _ := proto.Marshal(rsp)
	sess.SendPacket("pb.UserValueRSP", pkt.RoomID, body)
}

func (s *Server) HandleSngMatchPlayerNumREQ(sess *Session, pkt *Packet) {
	rsp := &gen.SngMatchPlayerNumRSP{}
	body, _ := proto.Marshal(rsp)
	sess.SendPacket("pb.SngMatchPlayerNumRSP", pkt.RoomID, body)
}

func (s *Server) HandleSetNewerGuideStepREQ(sess *Session, pkt *Packet) {
	req := &gen.SetNewerGuideStepREQ{}
	if err := proto.Unmarshal(pkt.Body, req); err != nil {
		return
	}
	// Persist the tutorial progress so the client does not replay it.
	if sess.UID != 0 && req.Step != nil {
		s.DB.ExecContext(context.Background(),
			`UPDATE users SET newer_guide_step=$1 WHERE uid=$2`, *req.Step, sess.UID)
	}
	rsp := &gen.SetNewerGuideStepRSP{Code: proto.Int32(0), Step: req.Step}
	body, _ := proto.Marshal(rsp)
	sess.SendPacket("pb.SetNewerGuideStepRSP", pkt.RoomID, body)
}

func (s *Server) getUserByUID(uid int64) (*model.User, error) {
	var u model.User
	err := s.DB.QueryRowContext(context.Background(),
		`SELECT uid, name, login_type, gold, avatar, frame, title, level, exp, vip_level, using_role_id, using_skin_id, newer_guide_step, client_def_str, register_time, auth_cert_url, auth_cert_time, monthly_card_exp, favorite_roles FROM users WHERE uid=$1 AND is_deleted=false`,
		uid,
	).Scan(&u.UID, &u.Name, &u.LoginType, &u.Gold, &u.Avatar, &u.Frame, &u.Title, &u.Level, &u.Exp, &u.VipLevel, &u.UsingRoleID, &u.UsingSkinID, &u.NewerGuideStep, &u.ClientDefStr, &u.RegisterTime, &u.AuthCertURL, &u.AuthCertTime, &u.MonthlyCardExp, &u.FavoriteRoles)
	if err != nil {
		return nil, err
	}
	return &u, nil
}

// loadFavoriteRoles parses the favorite_roles JSONB (array of skin_ids) and
// enriches each entry with the bond_level looked up from user_roles.bond via
// the role_id mapped from skin_id through tpl_character_skin.
func (s *Server) loadFavoriteRoles(uid int64) []*gen.UserFavoriteRole {
	var raw []byte
	err := s.DB.QueryRowContext(context.Background(),
		`SELECT favorite_roles::TEXT FROM users WHERE uid=$1`, uid,
	).Scan(&raw)
	if err != nil && err != sql.ErrNoRows {
		s.Logger.Warn("load favorite_roles failed", zap.Int64("uid", uid), zap.Error(err))
		return nil
	}
	if len(raw) == 0 {
		return nil
	}

	var skins []int32
	if err := json.Unmarshal(raw, &skins); err != nil {
		return nil
	}
	if len(skins) == 0 {
		return nil
	}

	// Batch-load bond levels for all relevant role_ids to avoid N+1 queries.
	roleIDs := make(map[int32]bool)
	skinToRole := make(map[int32]int32, len(skins))
	for _, skinID := range skins {
		roleID := gamedata.RoleBySkinID(skinID)
		skinToRole[skinID] = roleID
		if roleID != 0 {
			roleIDs[roleID] = true
		}
	}

	bondByRole := make(map[int32]int32)
	if len(roleIDs) > 0 {
		ids := make([]int32, 0, len(roleIDs))
		for id := range roleIDs {
			ids = append(ids, id)
		}
		rows, err := s.DB.QueryContext(context.Background(),
			`SELECT role_id, bond FROM user_roles WHERE uid=$1 AND role_id = ANY($2)`,
			uid, pq.Array(ids))
		if err == nil {
			for rows.Next() {
				var roleID, bond int32
				if rows.Scan(&roleID, &bond) == nil {
					bondByRole[roleID] = bond
				}
			}
			rows.Close()
		}
	}

	out := make([]*gen.UserFavoriteRole, 0, len(skins))
	for _, skinID := range skins {
		roleID := skinToRole[skinID]
		bond := bondByRole[roleID]
		out = append(out, &gen.UserFavoriteRole{
			SkinId:    proto.Int32(skinID),
			BondLevel: proto.Int32(bond),
		})
	}
	return out
}
