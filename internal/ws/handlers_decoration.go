package ws

import (
	"context"
	"database/sql"
	"time"

	"github.com/google/uuid"
	"github.com/lib/pq"
	"go.uber.org/zap"
	"google.golang.org/protobuf/proto"

	"poker-fate-server/internal/proto/gen"
)

// RegisterDecorationHandlers wires up the decoration-scheme WS handlers.
func (s *Server) RegisterDecorationHandlers() {
	s.RegisterHandler("pb.GetDCSchemeREQ", s.HandleGetDCSchemeREQ)
	s.RegisterHandler("pb.SaveDCSchemeREQ", s.HandleSaveDCSchemeREQ)
	s.RegisterHandler("pb.DeleteDCSchemeREQ", s.HandleDeleteDCSchemeREQ)
	s.RegisterHandler("pb.ChangeUsingDCSchemeREQ", s.HandleChangeUsingDCSchemeREQ)
	s.RegisterHandler("pb.UpdateDCSchemeRandFlagREQ", s.HandleUpdateDCSchemeRandFlagREQ)
	s.RegisterHandler("pb.SetSchemeNameREQ", s.HandleSetSchemeNameREQ)
	s.RegisterHandler("pb.SaveRoomDCSchemeREQ", s.HandleSaveRoomDCSchemeREQ)
	s.RegisterHandler("pb.SetSchemeInfoREQ", s.HandleSetSchemeInfoREQ)
	s.RegisterHandler("pb.ChangeAnimationREQ", s.HandleChangeAnimationREQ)
}

// --- Decoration schemes ---
//
// Persistence backed by three tables (migrations/003_decoration_schemes.sql):
//   user_dc_schemes        - lobby decoration schemes (DCSchemeItem)
//   user_room_dc_schemes   - in-game/room decoration schemes (RoomDCScheme)
//   user_dc_scheme_state   - per-user selection state (specify / rand list / using)

// dcSchemeState mirrors user_dc_scheme_state.
type dcSchemeState struct {
	lobbySpecifyScheme string
	lobbyRandSchemes   []string
	lobbyUsingScheme   string
	roomSpecifyScheme  string
	roomRandSchemes    []string
	roomMode           int32
	randDCSchemeFlag   int32
}

// loadDCSchemeState returns the user's scheme state, seeding a default row on first access.
func (s *Server) loadDCSchemeState(ctx context.Context, uid int64) (*dcSchemeState, error) {
	st := &dcSchemeState{
		lobbyRandSchemes: []string{},
		roomRandSchemes:  []string{},
	}
	err := s.DB.QueryRowContext(ctx,
		`SELECT lobby_specify_scheme, lobby_rand_schemes, lobby_using_scheme,
		        room_specify_scheme, room_rand_schemes, room_mode, rand_dc_scheme_flag
		 FROM user_dc_scheme_state WHERE uid=$1`, uid,
	).Scan(&st.lobbySpecifyScheme, pq.Array(&st.lobbyRandSchemes), &st.lobbyUsingScheme,
		&st.roomSpecifyScheme, pq.Array(&st.roomRandSchemes), &st.roomMode, &st.randDCSchemeFlag)
	if err == sql.ErrNoRows {
		// Insert default row; concurrent inserts are safe via ON CONFLICT.
		if _, e := s.DB.ExecContext(ctx,
			`INSERT INTO user_dc_scheme_state(uid) VALUES($1) ON CONFLICT DO NOTHING`, uid); e != nil {
			return nil, e
		}
		return st, nil
	}
	return st, err
}

// saveDCSchemeState upserts the user's scheme state.
func (s *Server) saveDCSchemeState(ctx context.Context, uid int64, st *dcSchemeState) error {
	// Coalesce nil slices to empty so pq.Array never emits SQL NULL,
	// which would violate the NOT NULL columns lobby_rand_schemes / room_rand_schemes.
	lobbyRand := st.lobbyRandSchemes
	if lobbyRand == nil {
		lobbyRand = []string{}
	}
	roomRand := st.roomRandSchemes
	if roomRand == nil {
		roomRand = []string{}
	}
	_, err := s.DB.ExecContext(ctx,
		`INSERT INTO user_dc_scheme_state(uid, lobby_specify_scheme, lobby_rand_schemes,
		        lobby_using_scheme, room_specify_scheme, room_rand_schemes, room_mode,
		        rand_dc_scheme_flag, updated_at)
		 VALUES($1,$2,$3,$4,$5,$6,$7,$8,NOW())
		 ON CONFLICT(uid) DO UPDATE SET
		    lobby_specify_scheme=EXCLUDED.lobby_specify_scheme,
		    lobby_rand_schemes=EXCLUDED.lobby_rand_schemes,
		    lobby_using_scheme=EXCLUDED.lobby_using_scheme,
		    room_specify_scheme=EXCLUDED.room_specify_scheme,
		    room_rand_schemes=EXCLUDED.room_rand_schemes,
		    room_mode=EXCLUDED.room_mode,
		    rand_dc_scheme_flag=EXCLUDED.rand_dc_scheme_flag,
		    updated_at=NOW()`,
		uid, st.lobbySpecifyScheme, pq.Array(lobbyRand), st.lobbyUsingScheme,
		st.roomSpecifyScheme, pq.Array(roomRand), st.roomMode, st.randDCSchemeFlag)
	return err
}

// loadLobbyDCSchemes loads all lobby decoration schemes for a user, ordered by create time.
func (s *Server) loadLobbyDCSchemes(ctx context.Context, uid int64) ([]*gen.DCSchemeItem, error) {
	rows, err := s.DB.QueryContext(ctx,
		`SELECT scheme_id, skin_id, lobby_scene_id, lobby_bgm_list, create_at,
		        property_list, title, frame, avatar
		 FROM user_dc_schemes WHERE uid=$1 ORDER BY create_at`, uid)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	items := make([]*gen.DCSchemeItem, 0)
	for rows.Next() {
		var (
			schemeID     string
			skinID       int32
			lobbySceneID int32
			lobbyBgmList pq.Int32Array
			createAt     int64
			propertyList pq.Int32Array
			title        int32
			frame        int32
			avatar       int32
		)
		if err := rows.Scan(&schemeID, &skinID, &lobbySceneID, &lobbyBgmList,
			&createAt, &propertyList, &title, &frame, &avatar); err != nil {
			return nil, err
		}
		items = append(items, &gen.DCSchemeItem{
			SchemeId:     proto.String(schemeID),
			SkinId:       proto.Int32(skinID),
			LobbySceneId: proto.Int32(lobbySceneID),
			LobbyBgmList: []int32(lobbyBgmList),
			CreateAt:     proto.Int32(int32(createAt)),
			PropertyList: []int32(propertyList),
			Title:        proto.Int32(title),
			Frame:        proto.Int32(frame),
			Avatar:       proto.Int32(avatar),
		})
	}
	return items, rows.Err()
}

// loadRoomDCSchemes loads all room decoration schemes for a user, ordered by create time.
func (s *Server) loadRoomDCSchemes(ctx context.Context, uid int64) ([]*gen.RoomDCScheme, error) {
	rows, err := s.DB.QueryContext(ctx,
		`SELECT scheme_id, name, create_at, rand, emojis, title, "table",
		        card_back, card_front, all_in_animation, bgm_list, skin_id,
		        show_card, open_card
		 FROM user_room_dc_schemes WHERE uid=$1 ORDER BY create_at`, uid)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	items := make([]*gen.RoomDCScheme, 0)
	for rows.Next() {
		var (
			schemeID  string
			name      string
			createAt  int64
			randVal   int32
			emojis    pq.Int32Array
			title     pq.Int32Array
			table     pq.Int32Array
			cardBack  pq.Int32Array
			cardFront pq.Int32Array
			allInAnim pq.Int32Array
			bgmList   pq.Int32Array
			skinID    pq.Int32Array
			showCard  pq.Int32Array
			openCard  pq.Int32Array
		)
		if err := rows.Scan(&schemeID, &name, &createAt, &randVal, &emojis, &title,
			&table, &cardBack, &cardFront, &allInAnim, &bgmList, &skinID, &showCard, &openCard); err != nil {
			return nil, err
		}
		items = append(items, &gen.RoomDCScheme{
			SchemeId:       proto.String(schemeID),
			Name:           proto.String(name),
			CreateAt:       proto.Int32(int32(createAt)),
			Rand:           proto.Int32(randVal),
			Emojis:         []int32(emojis),
			Title:          []int32(title),
			Table:          []int32(table),
			CardBack:       []int32(cardBack),
			CardFront:      []int32(cardFront),
			AllInAnimation: []int32(allInAnim),
			BgmList:        []int32(bgmList),
			SkinId:         []int32(skinID),
			ShowCard:       []int32(showCard),
			OpenCard:       []int32(openCard),
		})
	}
	return items, rows.Err()
}

// seedDefaultLobbyScheme creates a default lobby decoration scheme for first-time
// users and marks it as the using + specify scheme. Without this, the client's
// changeCurScheme() returns a scheme with nil scheme_id, causing SaveDCSchemeREQ
// (operation_type=1, scheme_id="") to UPDATE 0 rows silently.
func (s *Server) seedDefaultLobbyScheme(ctx context.Context, uid int64) (*gen.DCSchemeItem, error) {
	schemeID := uuid.NewString()
	now := time.Now().Unix()
	// property_list defaults: [posX, posY, scale, musicTag=1(order), angle]
	defaultProps := pq.Int32Array{0, 0, 0, 1, 0}
	// Field defaults must match client DecorationModel:getDefaultLobbyScheme()
	// (lobby_scene_id=20900001, lobby_bgm_list={20700001}, avatar=20110101).
	// lobby_scene_id=0 would crash tpl_props[0] in getCurLobbyScene().
	if _, err := s.DB.ExecContext(ctx,
		`INSERT INTO user_dc_schemes(uid, scheme_id, skin_id, lobby_scene_id,
		        lobby_bgm_list, create_at, property_list, title, frame, avatar)
		 VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9,$10)`,
		uid, schemeID, defaultSkinID(), defaultLobbySceneID,
		pq.Array([]int32{defaultLobbyBgmID}), now, pq.Array(defaultProps), 0, 0, defaultAvatarID); err != nil {
		return nil, err
	}
	return &gen.DCSchemeItem{
		SchemeId:     proto.String(schemeID),
		SkinId:       proto.Int32(defaultSkinID()),
		LobbySceneId: proto.Int32(defaultLobbySceneID),
		LobbyBgmList: []int32{defaultLobbyBgmID},
		CreateAt:     proto.Int32(int32(now)),
		PropertyList: []int32(defaultProps),
		Title:        proto.Int32(0),
		Frame:        proto.Int32(0),
		Avatar:       proto.Int32(defaultAvatarID),
	}, nil
}

// seedDefaultRoomScheme creates a default room decoration scheme for first-time users.
// Ensures the client always has at least one room scheme to reference, preventing
// nil crashes in DecorationModel.getInGameSchemeInfoById.
func (s *Server) seedDefaultRoomScheme(ctx context.Context, uid int64) (*gen.RoomDCScheme, error) {
	schemeID := uuid.NewString()
	now := time.Now().Unix()
	_, err := s.DB.ExecContext(ctx,
		`INSERT INTO user_room_dc_schemes(uid, scheme_id, name, create_at, rand, skin_id)
		 VALUES($1,$2,'Default',$3,0,$4)`,
		uid, schemeID, now, pq.Array([]int32{defaultSkinID()}))
	if err != nil {
		return nil, err
	}
	return &gen.RoomDCScheme{
		SchemeId: proto.String(schemeID),
		Name:     proto.String("Default"),
		CreateAt: proto.Int32(int32(now)),
		Rand:     proto.Int32(0),
		SkinId:   []int32{defaultSkinID()},
	}, nil
}

// removeString returns a copy of list with all occurrences of v removed.
func removeString(list []string, v string) []string {
	out := make([]string, 0, len(list))
	for _, s := range list {
		if s != v {
			out = append(out, s)
		}
	}
	return out
}

// firstRemainingLobbyScheme returns the first lobby scheme (by create_at) that
// is not in excludeRand. Used to auto-select a replacement after deleting the
// active scheme, so the DeleteDCSchemeRSP always carries a non-empty id (the
// client ignores empty values per the proto contract).
func (s *Server) firstRemainingLobbyScheme(ctx context.Context, uid int64, excludeRand []string) string {
	excl := make(map[string]bool, len(excludeRand))
	for _, id := range excludeRand {
		excl[id] = true
	}
	rows, err := s.DB.QueryContext(ctx,
		`SELECT scheme_id FROM user_dc_schemes WHERE uid=$1 ORDER BY create_at LIMIT 1`, uid)
	if err != nil {
		return ""
	}
	defer rows.Close()
	for rows.Next() {
		var id string
		if rows.Scan(&id) == nil && !excl[id] {
			return id
		}
	}
	return ""
}

// firstRemainingRoomScheme returns the first room scheme (by create_at) that
// is not in excludeRand.
func (s *Server) firstRemainingRoomScheme(ctx context.Context, uid int64, excludeRand []string) string {
	excl := make(map[string]bool, len(excludeRand))
	for _, id := range excludeRand {
		excl[id] = true
	}
	rows, err := s.DB.QueryContext(ctx,
		`SELECT scheme_id FROM user_room_dc_schemes WHERE uid=$1 ORDER BY create_at LIMIT 1`, uid)
	if err != nil {
		return ""
	}
	defer rows.Close()
	for rows.Next() {
		var id string
		if rows.Scan(&id) == nil && !excl[id] {
			return id
		}
	}
	return ""
}

// nonNilSchemes returns s if non-nil, otherwise an empty slice. Used at assignment
// sites that receive req.GetSchemes() (which may be nil) to keep state slices
// non-nil and consistent with the NOT NULL DB columns.
func nonNilSchemes(s []string) []string {
	if s == nil {
		return []string{}
	}
	return s
}

func (s *Server) HandleGetDCSchemeREQ(sess *Session, pkt *Packet) {
	ctx := context.Background()
	uid := sess.UID
	if uid == 0 {
		return
	}

	st, err := s.loadDCSchemeState(ctx, uid)
	if err != nil {
		s.Logger.Warn("loadDCSchemeState failed", zap.Error(err))
		return
	}
	lobbySchemes, err := s.loadLobbyDCSchemes(ctx, uid)
	if err != nil {
		s.Logger.Warn("loadLobbyDCSchemes failed", zap.Error(err))
		return
	}
	roomSchemes, err := s.loadRoomDCSchemes(ctx, uid)
	if err != nil {
		s.Logger.Warn("loadRoomDCSchemes failed", zap.Error(err))
		return
	}

	// Seed a default lobby scheme the first time, and auto-designate it as the
	// using + specify scheme so the client's changeCurScheme() always resolves
	// to a valid scheme_id.
	if len(lobbySchemes) == 0 {
		if def, err := s.seedDefaultLobbyScheme(ctx, uid); err != nil {
			s.Logger.Warn("seedDefaultLobbyScheme failed", zap.Error(err))
		} else {
			lobbySchemes = append(lobbySchemes, def)
			if st.lobbyUsingScheme == "" {
				st.lobbyUsingScheme = def.GetSchemeId()
			}
			if st.lobbySpecifyScheme == "" {
				st.lobbySpecifyScheme = def.GetSchemeId()
			}
			_ = s.saveDCSchemeState(ctx, uid, st)
		}
	}

	// Seed a default room scheme the first time, and auto-designate it as the
	// room specify scheme so the client has a valid active selection.
	if len(roomSchemes) == 0 {
		if def, err := s.seedDefaultRoomScheme(ctx, uid); err != nil {
			s.Logger.Warn("seedDefaultRoomScheme failed", zap.Error(err))
		} else {
			roomSchemes = append(roomSchemes, def)
			if st.roomSpecifyScheme == "" {
				st.roomSpecifyScheme = def.GetSchemeId()
				_ = s.saveDCSchemeState(ctx, uid, st)
			}
		}
	}

	rsp := &gen.GetDCSchemeRSP{
		Code:                  proto.Int32(0),
		SchemeList:            lobbySchemes,
		UsingDecorationScheme: proto.String(st.lobbyUsingScheme),
		RandDcSchemeFlag:      proto.Int32(st.randDCSchemeFlag),
		LobbyRandSchemes:      st.lobbyRandSchemes,
		LobbySpecifyScheme:    proto.String(st.lobbySpecifyScheme),
		RoomSchemeList:        roomSchemes,
		RoomRandSchemes:       st.roomRandSchemes,
		RoomSpecifyScheme:     proto.String(st.roomSpecifyScheme),
		RoomMode:              proto.Int32(st.roomMode),
	}
	sess.SendPacket("pb.GetDCSchemeRSP", pkt.RoomID, mustMarshal(rsp))
}

func (s *Server) HandleSaveDCSchemeREQ(sess *Session, pkt *Packet) {
	ctx := context.Background()
	uid := sess.UID
	if uid == 0 {
		return
	}
	req := &gen.SaveDCSchemeREQ{}
	if err := proto.Unmarshal(pkt.Body, req); err != nil {
		return
	}

	now := time.Now().Unix()
	schemeID := req.GetSchemeId()
	opType := req.GetOperationType()
	// Defensive: if the client sends operation_type=1 (modify) with an empty
	// scheme_id (happens when using a default scheme that has no DB row),
	// coerce to a new-scheme insert so the change is not silently lost.
	if opType == 1 && schemeID == "" {
		opType = 0
	}
	if opType == 0 {
		// New scheme - generate UUID if not provided.
		if schemeID == "" {
			schemeID = uuid.NewString()
		}
		if _, err := s.DB.ExecContext(ctx,
			`INSERT INTO user_dc_schemes(uid, scheme_id, skin_id, lobby_scene_id,
			        lobby_bgm_list, create_at, property_list, title, frame, avatar)
			 VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9,$10)`,
			uid, schemeID, req.GetSkinId(), req.GetLobbySceneId(),
			pq.Array(req.LobbyBgmList), now, pq.Array(req.PropertyList),
			req.GetTitle(), req.GetFrame(), req.GetAvatar()); err != nil {
			s.Logger.Warn("insert lobby scheme failed", zap.Error(err))
			sess.SendPacket("pb.SaveDCSchemeRSP", pkt.RoomID,
				mustMarshal(&gen.SaveDCSchemeRSP{Code: proto.Int32(-1)}))
			return
		}
	} else {
		// Update existing scheme.
		if _, err := s.DB.ExecContext(ctx,
			`UPDATE user_dc_schemes SET skin_id=$3, lobby_scene_id=$4, lobby_bgm_list=$5,
			        property_list=$6, title=$7, frame=$8, avatar=$9, updated_at=NOW()
			 WHERE uid=$1 AND scheme_id=$2`,
			uid, schemeID, req.GetSkinId(), req.GetLobbySceneId(),
			pq.Array(req.LobbyBgmList), pq.Array(req.PropertyList),
			req.GetTitle(), req.GetFrame(), req.GetAvatar()); err != nil {
			s.Logger.Warn("update lobby scheme failed", zap.Error(err))
			sess.SendPacket("pb.SaveDCSchemeRSP", pkt.RoomID,
				mustMarshal(&gen.SaveDCSchemeRSP{Code: proto.Int32(-1)}))
			return
		}
	}

	saved := &gen.DCSchemeItem{
		SchemeId:     proto.String(schemeID),
		SkinId:       proto.Int32(req.GetSkinId()),
		LobbySceneId: proto.Int32(req.GetLobbySceneId()),
		LobbyBgmList: req.LobbyBgmList,
		CreateAt:     proto.Int32(int32(now)),
		PropertyList: req.PropertyList,
		Title:        proto.Int32(req.GetTitle()),
		Frame:        proto.Int32(req.GetFrame()),
		Avatar:       proto.Int32(req.GetAvatar()),
	}

	if req.GetSetAsUsing() {
		if st, err := s.loadDCSchemeState(ctx, uid); err == nil {
			st.lobbyUsingScheme = schemeID
			_ = s.saveDCSchemeState(ctx, uid, st)
		}
	}

	rsp := &gen.SaveDCSchemeRSP{
		Code:       proto.Int32(0),
		Scheme:     saved,
		SetAsUsing: req.SetAsUsing,
	}
	s.Logger.Info("SaveDCSchemeREQ",
		zap.Int64("uid", uid),
		zap.String("scheme_id", schemeID),
		zap.Int32("op_type", opType),
		zap.Int32("skin_id", req.GetSkinId()),
		zap.Int32("lobby_scene_id", req.GetLobbySceneId()),
		zap.Int32("avatar", req.GetAvatar()),
		zap.Int32("frame", req.GetFrame()),
		zap.Int32("title", req.GetTitle()),
		zap.Int32s("bgm_list", req.GetLobbyBgmList()),
		zap.Int32s("property_list", req.GetPropertyList()),
		zap.Bool("set_as_using", req.GetSetAsUsing()),
	)
	sess.SendPacket("pb.SaveDCSchemeRSP", pkt.RoomID, mustMarshal(rsp))
}

func (s *Server) HandleSaveRoomDCSchemeREQ(sess *Session, pkt *Packet) {
	ctx := context.Background()
	uid := sess.UID
	if uid == 0 {
		return
	}
	req := &gen.SaveRoomDCSchemeREQ{}
	if err := proto.Unmarshal(pkt.Body, req); err != nil {
		return
	}

	sch := req.GetScheme()
	if sch == nil {
		sess.SendPacket("pb.SaveRoomDCSchemeRSP", pkt.RoomID,
			mustMarshal(&gen.SaveRoomDCSchemeRSP{Code: proto.Int32(-1)}))
		return
	}

	now := time.Now().Unix()
	schemeID := sch.GetSchemeId()
	opType := req.GetOperationType()
	if opType == 0 {
		// New scheme.
		if schemeID == "" {
			schemeID = uuid.NewString()
		}
		createAt := sch.GetCreateAt()
		if createAt == 0 {
			createAt = int32(now)
		}
		if _, err := s.DB.ExecContext(ctx,
			`INSERT INTO user_room_dc_schemes(uid, scheme_id, name, create_at, rand,
			        emojis, title, "table", card_back, card_front, all_in_animation,
			        bgm_list, skin_id, show_card, open_card)
			 VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15)`,
			uid, schemeID, sch.GetName(), createAt, sch.GetRand(),
			pq.Array(sch.Emojis), pq.Array(sch.Title), pq.Array(sch.Table),
			pq.Array(sch.CardBack), pq.Array(sch.CardFront), pq.Array(sch.AllInAnimation),
			pq.Array(sch.BgmList), pq.Array(sch.SkinId), pq.Array(sch.ShowCard), pq.Array(sch.OpenCard)); err != nil {
			s.Logger.Warn("insert room scheme failed", zap.Error(err))
			sess.SendPacket("pb.SaveRoomDCSchemeRSP", pkt.RoomID,
				mustMarshal(&gen.SaveRoomDCSchemeRSP{Code: proto.Int32(-1)}))
			return
		}
		sch.SchemeId = proto.String(schemeID)
		sch.CreateAt = proto.Int32(createAt)
	} else {
		// Update existing scheme.
		if _, err := s.DB.ExecContext(ctx,
			`UPDATE user_room_dc_schemes SET name=$3, rand=$4, emojis=$5, title=$6,
			        "table"=$7, card_back=$8, card_front=$9, all_in_animation=$10,
			        bgm_list=$11, skin_id=$12, show_card=$13, open_card=$14, updated_at=NOW()
			 WHERE uid=$1 AND scheme_id=$2`,
			uid, schemeID, sch.GetName(), sch.GetRand(),
			pq.Array(sch.Emojis), pq.Array(sch.Title), pq.Array(sch.Table),
			pq.Array(sch.CardBack), pq.Array(sch.CardFront), pq.Array(sch.AllInAnimation),
			pq.Array(sch.BgmList), pq.Array(sch.SkinId), pq.Array(sch.ShowCard), pq.Array(sch.OpenCard)); err != nil {
			s.Logger.Warn("update room scheme failed", zap.Error(err))
			sess.SendPacket("pb.SaveRoomDCSchemeRSP", pkt.RoomID,
				mustMarshal(&gen.SaveRoomDCSchemeRSP{Code: proto.Int32(-1)}))
			return
		}
	}

	rsp := &gen.SaveRoomDCSchemeRSP{
		Code:          proto.Int32(0),
		OperationType: proto.Int32(opType),
		Scheme:        sch,
	}
	sess.SendPacket("pb.SaveRoomDCSchemeRSP", pkt.RoomID, mustMarshal(rsp))
}

func (s *Server) HandleDeleteDCSchemeREQ(sess *Session, pkt *Packet) {
	ctx := context.Background()
	uid := sess.UID
	if uid == 0 {
		return
	}
	req := &gen.DeleteDCSchemeREQ{}
	if err := proto.Unmarshal(pkt.Body, req); err != nil {
		return
	}

	schemeID := req.GetSchemeId()
	schemeType := req.GetType() // 1=lobby, 2=room
	if schemeID == "" {
		return
	}

	table := "user_room_dc_schemes"
	if schemeType == 1 {
		table = "user_dc_schemes"
	}
	if _, err := s.DB.ExecContext(ctx,
		`DELETE FROM `+table+` WHERE uid=$1 AND scheme_id=$2`, uid, schemeID); err != nil {
		s.Logger.Warn("delete scheme failed", zap.Error(err))
	}

	// Recompute state - clear using/specify and remove from rand lists.
	// Per the proto, the client only updates its local specify/using fields
	// when the response value is non-empty, so we must auto-select a
	// replacement scheme when the active one is deleted.
	st, _ := s.loadDCSchemeState(ctx, uid)
	var specifyScheme, usingScheme string
	var randSchemes []string
	if st != nil {
		st.lobbyRandSchemes = removeString(st.lobbyRandSchemes, schemeID)
		st.roomRandSchemes = removeString(st.roomRandSchemes, schemeID)
		if schemeType == 1 {
			if st.lobbyUsingScheme == schemeID || st.lobbySpecifyScheme == schemeID {
				replacement := s.firstRemainingLobbyScheme(ctx, uid, st.lobbyRandSchemes)
				if st.lobbyUsingScheme == schemeID {
					st.lobbyUsingScheme = replacement
				}
				if st.lobbySpecifyScheme == schemeID {
					st.lobbySpecifyScheme = replacement
				}
			}
			specifyScheme = st.lobbySpecifyScheme
			randSchemes = st.lobbyRandSchemes
			usingScheme = st.lobbyUsingScheme
		} else {
			if st.roomSpecifyScheme == schemeID {
				st.roomSpecifyScheme = s.firstRemainingRoomScheme(ctx, uid, st.roomRandSchemes)
			}
			specifyScheme = st.roomSpecifyScheme
			randSchemes = st.roomRandSchemes
		}
		_ = s.saveDCSchemeState(ctx, uid, st)
	}

	rsp := &gen.DeleteDCSchemeRSP{
		Code:                  proto.Int32(0),
		SchemeId:              proto.String(schemeID),
		Type:                  proto.Int32(schemeType),
		SpecifyScheme:         proto.String(specifyScheme),
		RandSchemes:           randSchemes,
		UsingDecorationScheme: proto.String(usingScheme),
	}
	sess.SendPacket("pb.DeleteDCSchemeRSP", pkt.RoomID, mustMarshal(rsp))
}

func (s *Server) HandleSetSchemeInfoREQ(sess *Session, pkt *Packet) {
	ctx := context.Background()
	uid := sess.UID
	if uid == 0 {
		return
	}
	req := &gen.SetSchemeInfoREQ{}
	if err := proto.Unmarshal(pkt.Body, req); err != nil {
		return
	}

	schemeType := req.GetType() // 1=lobby, 2=room
	modType := req.GetModType() // 0=specify, 1=random
	schemes := req.GetSchemes()

	st, err := s.loadDCSchemeState(ctx, uid)
	if err != nil {
		s.Logger.Warn("loadDCSchemeState failed", zap.Error(err))
		st = &dcSchemeState{
			lobbyRandSchemes: []string{},
			roomRandSchemes:  []string{},
		}
	}

	var specifyScheme string
	if schemeType == 1 {
		if modType == 0 {
			if len(schemes) > 0 {
				st.lobbySpecifyScheme = schemes[0]
				// Keep lobbyUsingScheme in sync with the client, which sets
				// _lobby_using_scheme = msg.schemes[1] in evt_SetSchemeInfoRSP.
				st.lobbyUsingScheme = schemes[0]
			} else {
				st.lobbySpecifyScheme = ""
				st.lobbyUsingScheme = ""
			}
			specifyScheme = st.lobbySpecifyScheme
		} else {
			st.lobbyRandSchemes = nonNilSchemes(schemes)
		}
	} else {
		if modType == 0 {
			if len(schemes) > 0 {
				st.roomSpecifyScheme = schemes[0]
			} else {
				st.roomSpecifyScheme = ""
			}
			specifyScheme = st.roomSpecifyScheme
			st.roomMode = 0
		} else {
			st.roomRandSchemes = nonNilSchemes(schemes)
			st.roomMode = 1
		}
	}
	if err := s.saveDCSchemeState(ctx, uid, st); err != nil {
		s.Logger.Warn("saveDCSchemeState failed", zap.Error(err))
	}

	rsp := &gen.SetSchemeInfoRSP{
		Code:          proto.Int32(0),
		Type:          proto.Int32(schemeType),
		ModType:       proto.Int32(modType),
		Schemes:       schemes,
		SpecifyScheme: proto.String(specifyScheme),
	}
	s.Logger.Info("SetSchemeInfoREQ",
		zap.Int64("uid", uid),
		zap.Int32("type", schemeType),
		zap.Int32("mod_type", modType),
		zap.Strings("schemes", schemes),
		zap.String("specify_scheme", specifyScheme),
	)
	sess.SendPacket("pb.SetSchemeInfoRSP", pkt.RoomID, mustMarshal(rsp))
}

func (s *Server) HandleSetSchemeNameREQ(sess *Session, pkt *Packet) {
	ctx := context.Background()
	uid := sess.UID
	if uid == 0 {
		return
	}
	req := &gen.SetSchemeNameREQ{}
	if err := proto.Unmarshal(pkt.Body, req); err != nil {
		return
	}

	schemeID := req.GetSchemeId()
	name := req.GetName()
	if schemeID == "" {
		return
	}
	if _, err := s.DB.ExecContext(ctx,
		`UPDATE user_room_dc_schemes SET name=$3, updated_at=NOW() WHERE uid=$1 AND scheme_id=$2`,
		uid, schemeID, name); err != nil {
		s.Logger.Warn("set scheme name failed", zap.Error(err))
	}

	rsp := &gen.SetSchemeNameRSP{
		Code:     proto.Int32(0),
		Name:     proto.String(name),
		SchemeId: proto.String(schemeID),
	}
	sess.SendPacket("pb.SetSchemeNameRSP", pkt.RoomID, mustMarshal(rsp))
}

// Deprecated since 1.5.7 - still backed by state for legacy clients.
func (s *Server) HandleChangeUsingDCSchemeREQ(sess *Session, pkt *Packet) {
	ctx := context.Background()
	uid := sess.UID
	if uid == 0 {
		return
	}
	req := &gen.ChangeUsingDCSchemeREQ{}
	if err := proto.Unmarshal(pkt.Body, req); err != nil {
		return
	}

	schemeID := req.GetSchemeId()
	if st, err := s.loadDCSchemeState(ctx, uid); err == nil {
		st.lobbyUsingScheme = schemeID
		_ = s.saveDCSchemeState(ctx, uid, st)
	}

	rsp := &gen.ChangeUsingDCSchemeRSP{
		Code:     proto.Int32(0),
		SchemeId: proto.String(schemeID),
	}
	sess.SendPacket("pb.ChangeUsingDCSchemeRSP", pkt.RoomID, mustMarshal(rsp))
}

// Deprecated since 1.5.7 - still backed by state for legacy clients.
func (s *Server) HandleUpdateDCSchemeRandFlagREQ(sess *Session, pkt *Packet) {
	ctx := context.Background()
	uid := sess.UID
	if uid == 0 {
		return
	}
	req := &gen.UpdateDCSchemeRandFlagREQ{}
	if err := proto.Unmarshal(pkt.Body, req); err != nil {
		return
	}

	flag := req.GetRandomSchemeFlag()
	if st, err := s.loadDCSchemeState(ctx, uid); err == nil {
		st.randDCSchemeFlag = flag
		_ = s.saveDCSchemeState(ctx, uid, st)
	}

	rsp := &gen.UpdateDCSchemeRandFlagRSP{
		Code:             proto.Int32(0),
		RandomSchemeFlag: proto.Int32(flag),
	}
	sess.SendPacket("pb.UpdateDCSchemeRandFlagRSP", pkt.RoomID, mustMarshal(rsp))
}

func (s *Server) HandleChangeAnimationREQ(sess *Session, pkt *Packet) {
	req := &gen.ChangeAnimationREQ{}
	if err := proto.Unmarshal(pkt.Body, req); err != nil {
		return
	}
	rsp := &gen.ChangeAnimationRSP{Code: proto.Int32(0), Ftype: req.Ftype, ItemId: req.ItemId}
	sess.SendPacket("pb.ChangeAnimationRSP", pkt.RoomID, mustMarshal(rsp))
}
