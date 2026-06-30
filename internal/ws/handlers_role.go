package ws

import (
	"context"
	"database/sql"
	"encoding/json"

	"go.uber.org/zap"
	"google.golang.org/protobuf/proto"

	"poker-fate-server/internal/gamedata"
	"poker-fate-server/internal/proto/gen"
)

// RegisterRoleHandlers wires up role-list and role-operation WS handlers.
func (s *Server) RegisterRoleHandlers() {
	s.RegisterHandler("pb.RoleListREQ", s.HandleRoleListREQ)
	s.RegisterHandler("pb.SwitchRoleREQ", s.HandleSwitchRoleREQ)
	s.RegisterHandler("pb.SwitchRoleSkinREQ", s.HandleSwitchRoleSkinREQ)
	s.RegisterHandler("pb.RoleSetStarREQ", s.HandleRoleSetStarREQ)
	s.RegisterHandler("pb.RoleGiftREQ", s.HandleRoleGiftREQ)
	s.RegisterHandler("pb.RoleAwakenREQ", s.HandleRoleAwakenREQ)
	s.RegisterHandler("pb.EditFavoriteRoleREQ", s.HandleEditFavoriteRoleREQ)
	s.RegisterHandler("pb.GetSkinGameDetailREQ", s.HandleGetSkinGameDetailREQ)
}

// HandleRoleListREQ returns the full character roster with every skin owned
// and maxed level/bond. This is the private-server "all characters unlocked"
// policy and also fixes the tutorial crash where an empty role list caused
// CharacterModel:getUsingRole() to return nil.
func (s *Server) HandleRoleListREQ(sess *Session, pkt *Packet) {
	if sess.UID == 0 {
		return
	}

	user, _ := s.getUserByUID(sess.UID)
	usingRoleID := defaultRoleID()
	if user != nil && user.UsingRoleID != 0 {
		usingRoleID = int32(user.UsingRoleID)
	}

	roles := allRolesWithSkins()
	roleList := make([]*gen.RoleInfo, 0, len(roles))
	ownedSkinIds := make([]int32, 0)
	for _, r := range roles {
		ownedSkins := append([]int32(nil), r.Skins...)
		ownedSkinIds = append(ownedSkinIds, ownedSkins...)
		info := &gen.RoleInfo{
			RoleId: proto.Int32(r.RoleID),
			IsStar: proto.Bool(true),
			SkinInfo: &gen.RoleSkinInfo{
				SkinId:       proto.Int32(r.Skins[0]),
				OwnedSkinIds: ownedSkins,
			},
			LevelInfo: &gen.RoleLevelInfo{
				Level:   proto.Int32(maxRoleLevel),
				BondExp: proto.Int32(maxBondExp),
			},
		}
		roleList = append(roleList, info)
	}

	rsp := &gen.RoleListRSP{
		RoleList:     roleList,
		UsingRoleId:  proto.Int32(usingRoleID),
		OwnedSkinIds: ownedSkinIds,
	}
	body, _ := proto.Marshal(rsp)
	sess.SendPacket("pb.RoleListRSP", pkt.RoomID, body)
}

func (s *Server) HandleSwitchRoleREQ(sess *Session, pkt *Packet) {
	req := &gen.SwitchRoleREQ{}
	if err := proto.Unmarshal(pkt.Body, req); err != nil {
		return
	}
	if sess.UID != 0 && req.GetNewRoleId() != 0 {
		s.DB.ExecContext(context.Background(),
			`UPDATE users SET using_role_id=$1 WHERE uid=$2`, req.GetNewRoleId(), sess.UID)
	}
	rsp := &gen.SwitchRoleRSP{Code: proto.Int32(0), NewRoleId: req.NewRoleId}
	sess.SendPacket("pb.SwitchRoleRSP", pkt.RoomID, mustMarshal(rsp))
}

func (s *Server) HandleSwitchRoleSkinREQ(sess *Session, pkt *Packet) {
	req := &gen.SwitchRoleSkinREQ{}
	if err := proto.Unmarshal(pkt.Body, req); err != nil {
		return
	}
	if sess.UID != 0 && req.GetNewSkinId() != 0 {
		s.DB.ExecContext(context.Background(),
			`UPDATE users SET using_skin_id=$1 WHERE uid=$2`, req.GetNewSkinId(), sess.UID)
	}
	rsp := &gen.SwitchRoleSkinRSP{Code: proto.Int32(0), RoleId: req.RoleId, NewSkinId: req.NewSkinId}
	sess.SendPacket("pb.SwitchRoleSkinRSP", pkt.RoomID, mustMarshal(rsp))
}

func (s *Server) HandleRoleSetStarREQ(sess *Session, pkt *Packet) {
	req := &gen.RoleSetStarREQ{}
	if err := proto.Unmarshal(pkt.Body, req); err != nil {
		return
	}
	rsp := &gen.RoleSetStarRSP{Code: proto.Int32(0), RoleId: req.RoleId, IsStar: req.IsStar}
	sess.SendPacket("pb.RoleSetStarRSP", pkt.RoomID, mustMarshal(rsp))
}

func (s *Server) HandleRoleGiftREQ(sess *Session, pkt *Packet) {
	req := &gen.RoleGiftREQ{}
	if err := proto.Unmarshal(pkt.Body, req); err != nil {
		return
	}
	rsp := &gen.RoleGiftRSP{
		Code:        proto.Int32(0),
		SendGiftCnt: proto.Int32(0),
		RoleId:      req.RoleId,
		ItemUniqId:  req.ItemUniqId,
	}
	sess.SendPacket("pb.RoleGiftRSP", pkt.RoomID, mustMarshal(rsp))
}

func (s *Server) HandleRoleAwakenREQ(sess *Session, pkt *Packet) {
	req := &gen.RoleAwakenREQ{}
	if err := proto.Unmarshal(pkt.Body, req); err != nil {
		return
	}
	rsp := &gen.RoleAwakenRSP{Code: proto.Int32(0), RoleId: req.RoleId}
	sess.SendPacket("pb.RoleAwakenRSP", pkt.RoomID, mustMarshal(rsp))
}

// HandleEditFavoriteRoleREQ persists the player's favorite role skin selection
// into users.favorite_roles (JSONB array of skin_ids). When adding a favorite,
// any existing skin belonging to the same character is removed first, matching
// the client-side logic in net_role.lua:EditFavoriteRoleRSP where each role can
// only have one favorite skin.
func (s *Server) HandleEditFavoriteRoleREQ(sess *Session, pkt *Packet) {
	req := &gen.EditFavoriteRoleREQ{}
	if err := proto.Unmarshal(pkt.Body, req); err != nil {
		return
	}
	skinID := req.GetSkinId()
	isFavorite := req.GetIsFavorite()
	uid := sess.UID

	if uid != 0 && skinID != 0 {
		ctx := context.Background()
		var raw []byte
		err := s.DB.QueryRowContext(ctx,
			`SELECT favorite_roles::TEXT FROM users WHERE uid=$1`, uid,
		).Scan(&raw)
		if err != nil && err != sql.ErrNoRows {
			s.Logger.Warn("load favorite_roles failed", zap.Int64("uid", uid), zap.Error(err))
		}

		var skins []int32
		if len(raw) > 0 {
			_ = json.Unmarshal(raw, &skins)
		}
		if skins == nil {
			skins = []int32{}
		}

		if isFavorite {
			// Remove any existing favorite skin belonging to the same character.
			targetRole := gamedata.RoleBySkinID(skinID)
			filtered := skins[:0]
			for _, s := range skins {
				if gamedata.RoleBySkinID(s) == targetRole && targetRole != 0 {
					continue
				}
				filtered = append(filtered, s)
			}
			skins = append(filtered, skinID)
		} else {
			filtered := skins[:0]
			for _, s := range skins {
				if s == skinID {
					continue
				}
				filtered = append(filtered, s)
			}
			skins = filtered
		}

		payload, _ := json.Marshal(skins)
		if _, err := s.DB.ExecContext(ctx,
			`UPDATE users SET favorite_roles=$1, updated_at=NOW() WHERE uid=$2`,
			payload, uid,
		); err != nil {
			s.Logger.Warn("save favorite_roles failed", zap.Int64("uid", uid), zap.Error(err))
		}
	}

	rsp := &gen.EditFavoriteRoleRSP{Code: proto.Int32(0), SkinId: req.SkinId, IsFavorite: req.IsFavorite}
	sess.SendPacket("pb.EditFavoriteRoleRSP", pkt.RoomID, mustMarshal(rsp))
}

func (s *Server) HandleGetSkinGameDetailREQ(sess *Session, pkt *Packet) {
	rsp := &gen.GetSkinGameDetailRSP{Code: proto.Int32(0), SkinGameDetails: []*gen.SkinGameDetailItem{}}
	sess.SendPacket("pb.GetSkinGameDetailRSP", pkt.RoomID, mustMarshal(rsp))
}
