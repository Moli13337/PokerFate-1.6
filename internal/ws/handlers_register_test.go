package ws

import (
	"testing"

	"go.uber.org/zap"

	"poker-fate-server/internal/config"
)

// expectedPackTypes is the golden list of every pb.XxxREQ the server must
// handle. Add a line here whenever a new handler is introduced; the test
// will fail if the registration is missing.
var expectedPackTypes = []string{
	// auth
	"pb.UserLoginREQ", "pb.UserLogoutREQ", "pb.CancelLoginQueueREQ",
	// user
	"pb.SelfUserInfoREQ", "pb.UserValueREQ", "pb.SngMatchPlayerNumREQ",
	"pb.SetNewerGuideStepREQ",
	// role
	"pb.RoleListREQ", "pb.SwitchRoleREQ", "pb.SwitchRoleSkinREQ",
	"pb.RoleSetStarREQ", "pb.RoleGiftREQ", "pb.RoleAwakenREQ",
	"pb.EditFavoriteRoleREQ", "pb.GetSkinGameDetailREQ",
	// item
	"pb.ItemListREQ", "pb.RefreshItemREQ", "pb.RecycleItemREQ",
	"pb.UseTreasureBoxREQ", "pb.UseExpItemREQ", "pb.ChangeAvatarREQ",
	"pb.ChangeFrameREQ", "pb.ChangeTitleREQ", "pb.ChangeOutFitsREQ",
	// social
	"pb.GetBlockChatListREQ",
	// tutorial / lobby / room
	"pb.SetClientDefStrREQ", "pb.GetServerTimeREQ", "pb.SetTableFlagREQ",
	"pb.TableInfoREQ", "pb.GetRoomDataREQ", "pb.RoomWinnerRewardsInfoREQ",
	"pb.GetUserLevelInfoREQ", "pb.GetUserBondInfoREQ", "pb.UnlockNewModuleREQ",
	"pb.GetAllinGameTimeREQ", "pb.QuickStartREQ", "pb.EnterRoomREQ",
	"pb.LeaveRoomREQ", "pb.SitDownREQ", "pb.StandUpREQ",
	"pb.SelfAchievementsREQ", "pb.GetOtherDetailInfoREQ",
	"pb.ReserveSeatREQ", "pb.ChangeObserveViewREQ",
	// decoration
	"pb.GetDCSchemeREQ", "pb.SaveDCSchemeREQ", "pb.DeleteDCSchemeREQ",
	"pb.ChangeUsingDCSchemeREQ", "pb.UpdateDCSchemeRandFlagREQ",
	"pb.SetSchemeNameREQ", "pb.SaveRoomDCSchemeREQ",
	"pb.SetSchemeInfoREQ", "pb.ChangeAnimationREQ",
	// sidegame
	"pb.PushGiftREQ", "pb.GetSideGameConfREQ", "pb.GetSideGameHisRecordREQ",
	"pb.ColorGameActionREQ", "pb.GetColorGameConfREQ", "pb.PinballActionREQ",
	// bustprotect
	"pb.BustProtectInfoREQ", "pb.BustProtectRewardREQ",
	// heartbeat
	"pb.HeartBeatREQ",
	// lobby jackpot
	"pb.LobJackPotREQ", "pb.LobJackPotConfigREQ",
	// tournament (SNG / MTT / Tour)
	"pb.TourListREQ", "pb.TourHistoryListREQ", "pb.TourDetailInfoREQ",
	"pb.SngSignREQ", "pb.TourRoomDetailREQ",
	"pb.MttRankREQ", "pb.MttRankRewardREQ", "pb.MttPotRewardStructureREQ",
	"pb.MttSignREQ", "pb.MttCancelSignREQ", "pb.MttRoomDetailREQ",
	"pb.GetMttRoomREQ", "pb.GetMttTourBriefInfoREQ", "pb.SngCancelSignREQ",
	// room (chat / friend-room / leave)
	"pb.DelayLeaveRoomREQ", "pb.FaceREQ", "pb.TextREQ", "pb.CustomTextREQ",
	"pb.SetBlockChatREQ", "pb.CreateFriendRoomREQ", "pb.JoinFriendRoomREQ",
	"pb.FriendRoomListREQ", "pb.InvitePlayREQ", "pb.GetFriendHisRecordREQ",
	"pb.GetScoreboardREQ",
	// poker (CSHoldem stubs)
	"pb.PreActionREQ", "pb.ShowMyCardREQ", "pb.CancelWaitBlindREQ",
	"pb.SetWaitBlindTypeREQ", "pb.RebyREQ", "pb.SetRebyREQ",
	"pb.ActionREQ", "pb.GetCardsREQ", "pb.GetHandsListREQ",
	"pb.RoundStartDisplayFinishREQ", "pb.SendEmojiREQ",
}

func TestRegisterAllHandlersRegistersEveryExpectedPackType(t *testing.T) {
	srv := NewServer(&config.Config{}, nil, nil, zap.NewNop())
	srv.RegisterAllHandlers()

	for _, pt := range expectedPackTypes {
		t.Run(pt, func(t *testing.T) {
			srv.mu.RLock()
			fn, ok := srv.handlers[pt]
			srv.mu.RUnlock()
			if !ok {
				t.Fatalf("pack type %s not registered", pt)
			}
			if fn == nil {
				t.Fatalf("pack type %s registered with nil handler", pt)
			}
		})
	}

	if got := len(srv.handlers); got != len(expectedPackTypes) {
		t.Fatalf("handler count mismatch: got %d, expected %d (golden list out of sync or duplicate registration)",
			got, len(expectedPackTypes))
	}
}
