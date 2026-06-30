package ws

import (
	"context"
	"strconv"

	"go.uber.org/zap"
	"google.golang.org/protobuf/proto"

	"poker-fate-server/internal/model"
	"poker-fate-server/internal/proto/gen"
)

// RegisterItemHandlers wires up item-list and item-operation WS handlers.
func (s *Server) RegisterItemHandlers() {
	s.RegisterHandler("pb.ItemListREQ", s.HandleItemListREQ)
	s.RegisterHandler("pb.RefreshItemREQ", s.HandleRefreshItemREQ)
	s.RegisterHandler("pb.RecycleItemREQ", s.HandleRecycleItemREQ)
	s.RegisterHandler("pb.UseTreasureBoxREQ", s.HandleUseTreasureBoxREQ)
	s.RegisterHandler("pb.UseExpItemREQ", s.HandleUseExpItemREQ)
	s.RegisterHandler("pb.ChangeAvatarREQ", s.HandleChangeAvatarREQ)
	s.RegisterHandler("pb.ChangeFrameREQ", s.HandleChangeFrameREQ)
	s.RegisterHandler("pb.ChangeTitleREQ", s.HandleChangeTitleREQ)
	s.RegisterHandler("pb.ChangeOutFitsREQ", s.HandleChangeOutFitsREQ)
}

func (s *Server) HandleItemListREQ(sess *Session, pkt *Packet) {
	if sess.UID == 0 {
		return
	}

	items, err := s.getItemsByUID(sess.UID)
	if err != nil {
		s.Logger.Warn("get items failed", zap.Int64("uid", sess.UID), zap.Error(err))
		return
	}

	var itemList []*gen.ItemInfo
	for _, item := range items {
		info := &gen.ItemInfo{
			ItemId:     proto.Int32(int32(item.ItemID)),
			ItemUniqId: proto.String(strconv.FormatInt(item.ID, 10)),
			Num:        proto.Int64(int64(item.Count)),
		}
		itemList = append(itemList, info)
	}

	rsp := &gen.ItemListRSP{
		ItemList:        itemList,
		OwnedItemIdList: allItemIDs(),
		Code:            proto.Int32(0),
	}
	body, _ := proto.Marshal(rsp)
	sess.SendPacket("pb.ItemListRSP", pkt.RoomID, body)
}

func (s *Server) getItemsByUID(uid int64) ([]model.Item, error) {
	rows, err := s.DB.QueryContext(context.Background(),
		`SELECT id, item_id, count FROM user_items WHERE uid=$1 AND count > 0 ORDER BY id`, uid)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var items []model.Item
	for rows.Next() {
		var it model.Item
		if err := rows.Scan(&it.ID, &it.ItemID, &it.Count); err != nil {
			continue
		}
		it.UID = uid
		items = append(items, it)
	}
	return items, nil
}

// --- Item operations ---

func (s *Server) HandleRefreshItemREQ(sess *Session, pkt *Packet) {
	rsp := &gen.RefreshItemRSP{Code: proto.Int32(0)}
	sess.SendPacket("pb.RefreshItemRSP", pkt.RoomID, mustMarshal(rsp))
}

func (s *Server) HandleRecycleItemREQ(sess *Session, pkt *Packet) {
	rsp := &gen.RecycleItemRSP{Code: proto.Int32(0), RewardList: []*gen.RewardItemInfo{}}
	sess.SendPacket("pb.RecycleItemRSP", pkt.RoomID, mustMarshal(rsp))
}

func (s *Server) HandleUseTreasureBoxREQ(sess *Session, pkt *Packet) {
	rsp := &gen.UseTreasureBoxRSP{
		Code:           proto.Int32(0),
		RewardPropList: []*gen.RewardItemInfo{},
		RewardSkinList: []*gen.RewardItemInfo{},
		RewardRoleList: []*gen.RewardItemInfo{},
	}
	sess.SendPacket("pb.UseTreasureBoxRSP", pkt.RoomID, mustMarshal(rsp))
}

func (s *Server) HandleUseExpItemREQ(sess *Session, pkt *Packet) {
	rsp := &gen.UseExpItemRSP{Code: proto.Int32(0), ExpInc: proto.Int32(1000)}
	sess.SendPacket("pb.UseExpItemRSP", pkt.RoomID, mustMarshal(rsp))
}

func (s *Server) HandleChangeAvatarREQ(sess *Session, pkt *Packet) {
	req := &gen.ChangeAvatarREQ{}
	if err := proto.Unmarshal(pkt.Body, req); err != nil {
		return
	}
	if sess.UID != 0 {
		s.DB.ExecContext(context.Background(),
			`UPDATE users SET avatar=$1 WHERE uid=$2`, req.GetItemId(), sess.UID)
	}
	rsp := &gen.ChangeAvatarRSP{Code: proto.Int32(0), ItemId: req.ItemId}
	sess.SendPacket("pb.ChangeAvatarRSP", pkt.RoomID, mustMarshal(rsp))
}

func (s *Server) HandleChangeFrameREQ(sess *Session, pkt *Packet) {
	req := &gen.ChangeFrameREQ{}
	if err := proto.Unmarshal(pkt.Body, req); err != nil {
		return
	}
	if sess.UID != 0 {
		s.DB.ExecContext(context.Background(),
			`UPDATE users SET frame=$1 WHERE uid=$2`, req.GetItemId(), sess.UID)
	}
	rsp := &gen.ChangeFrameRSP{Code: proto.Int32(0), ItemId: req.ItemId}
	sess.SendPacket("pb.ChangeFrameRSP", pkt.RoomID, mustMarshal(rsp))
}

func (s *Server) HandleChangeTitleREQ(sess *Session, pkt *Packet) {
	req := &gen.ChangeTitleREQ{}
	if err := proto.Unmarshal(pkt.Body, req); err != nil {
		return
	}
	if sess.UID != 0 {
		s.DB.ExecContext(context.Background(),
			`UPDATE users SET title=$1 WHERE uid=$2`, req.GetItemId(), sess.UID)
	}
	rsp := &gen.ChangeTitleRSP{Code: proto.Int32(0), ItemId: req.ItemId}
	sess.SendPacket("pb.ChangeTitleRSP", pkt.RoomID, mustMarshal(rsp))
}

func (s *Server) HandleChangeOutFitsREQ(sess *Session, pkt *Packet) {
	req := &gen.ChangeOutFitsREQ{}
	if err := proto.Unmarshal(pkt.Body, req); err != nil {
		return
	}
	rsp := &gen.ChangeOutFitsRSP{Code: proto.Int32(0), ItemType: req.ItemType, ItemId: req.ItemId}
	sess.SendPacket("pb.ChangeOutFitsRSP", pkt.RoomID, mustMarshal(rsp))
}
