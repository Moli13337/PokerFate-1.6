package httpapi

import (
	"database/sql"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/redis/go-redis/v9"
	"go.uber.org/zap"

	"poker-fate-server/internal/config"
	"poker-fate-server/internal/ws"
)

type Router struct {
	Engine *gin.Engine
	Config *config.Config
	DB     *sql.DB
	Redis  *redis.Client
	WSSrv  *ws.Server
	Logger *zap.Logger
}

func NewRouter(cfg *config.Config, db *sql.DB, rdb *redis.Client, wsSrv *ws.Server, logger *zap.Logger) *Router {
	gin.SetMode(gin.ReleaseMode)
	return &Router{
		Engine: gin.New(),
		Config: cfg,
		DB:     db,
		Redis:  rdb,
		WSSrv:  wsSrv,
		Logger: logger,
	}
}

func (r *Router) Setup() {
	r.Engine.Use(gin.Logger(), gin.Recovery())

	r.Engine.GET("/ping", func(c *gin.Context) {
		c.JSON(200, gin.H{"message": "pong"})
	})

	// Serve uploaded auth-certificate signature images.
	r.Engine.Static("/uploads", "./uploads")

	// Version check endpoint - return no-update-needed response
	// Game requests: GET /1.0.0_version.json (G_REMOTE_RES_HOST + version + "_version.json")
	// The "res" field must match the game's local resource version to skip downloads
	versionData := gin.H{
		"windows": gin.H{
			"ver":     "1.5.3",
			"min_ver": "1.5.3",
			"url":     "StandaloneWindows64",
			"res":     "1.0.0",
			"min_res": "1.0.0",
		},
		"android": gin.H{
			"ver":       "1.5.3",
			"min_ver":   "1.5.3",
			"url":       "Android",
			"res":       "1.0.0",
			"min_res":   "1.0.0",
			"limit_ver": "0.0.0",
		},
		"android_official": gin.H{
			"ver":     "1.5.3",
			"min_ver": "1.5.3",
			"url":     "Android_official",
			"res":     "1.0.0",
			"min_res": "1.0.0",
		},
		"ios": gin.H{
			"ver":       "1.5.10",
			"min_ver":   "1.5.1",
			"url":       "iOS",
			"res":       "1.0.0",
			"min_res":   "1.0.0",
			"limit_ver": "0.0.0",
		},
	}
	r.Engine.GET("/client/remote_res/release/:filename", func(c *gin.Context) {
		c.JSON(200, versionData)
	})

	// Catch-all for version.json requests
	r.Engine.NoRoute(func(c *gin.Context) {
		path := c.Request.URL.Path
		// Match both _version.json and _version_beta.json
		if strings.HasSuffix(path, "_version.json") || strings.HasSuffix(path, "_version_beta.json") {
			c.JSON(200, versionData)
			return
		}
		c.JSON(404, gin.H{"error": "not found"})
	})

	public := r.Engine.Group("")
	public.Use(r.SignMiddleware())
	{
		public.POST("/login", r.LoginHandler)
		public.POST("/register/email", r.EmailRegisterHandler)
		public.POST("/captcha", r.CaptchaHandler)
		public.POST("/forgotPassword", r.ForgotPasswordHandler)
		public.POST("/xOauth", r.XOauthHandler)
		public.POST("/open/checkServer", r.CheckServerHandler)
		public.POST("/open/newVersionPicture", r.EmptyDataHandler)
	}

	auth := r.Engine.Group("")
	auth.Use(r.SignMiddleware(), r.AuthMiddleware())
	{
		// --- Shop ---
		auth.POST("/shop/createOrder", r.ShopCreateOrderHandler)
		auth.POST("/shop/steamPay", r.ShopSteamPayHandler)
		auth.POST("/shop/finishSteamPay", r.ShopPayDirectHandler)
		auth.POST("/shop/googlePay", r.ShopPayDirectHandler)
		auth.POST("/shop/applePay", r.ShopPayDirectHandler)
		auth.POST("/v2/shop/applePay", r.ShopPayDirectHandler)
		auth.POST("/v2/shop/thirdPay", r.ShopPayDirectHandler)
		auth.POST("/shop/reward", r.ShopRewardHandler)
		auth.POST("/shop/buyWithProp", r.ShopBuyWithPropHandler)
		auth.POST("/shop/limit", r.ShopLimitHandler)
		auth.POST("/shop/pid", r.ShopPidHandler)
		auth.POST("/shop/paymentMethod", r.ShopPaymentMethodHandler)

		// --- Friend ---
		auth.POST("/friend/list", r.FriendListHandler)
		auth.POST("/friend/applyList", r.FriendApplyListHandler)
		auth.POST("/friend/blockedList", r.FriendBlockedListHandler)
		auth.POST("/friend/gameList", r.FriendGameListHandler)
		auth.POST("/friend/searchList", r.FriendSearchListHandler)
		auth.POST("/friend/apply", r.FriendApplyHandler)
		auth.POST("/friend/mark", r.FriendMarkHandler)
		auth.POST("/friend/del", r.FriendDelHandler)
		auth.POST("/friend/blocked", r.FriendBlockedHandler)

		// --- Mail ---
		auth.POST("/mail/list", r.MailListHandler)
		auth.POST("/mail/num", r.MailNumHandler)
		auth.POST("/mail/detail", r.MailDetailHandler)
		auth.POST("/mail/receive", r.MailReceiveHandler)
		auth.POST("/mail/del", r.MailDeleteHandler)
		auth.POST("/mail/coll", r.MailCollHandler)

		// --- Task ---
		auth.POST("/task/list", r.TaskListHandler)
		auth.POST("/task/report", r.TaskReportHandler)
		auth.POST("/task/recReward", r.TaskRecRewardHandler)
		auth.POST("/task/recPointReward", r.TaskRecPointRewardHandler)
		auth.POST("/task/recChapterReward", r.TaskRecChapterRewardHandler)
		auth.POST("/task/openNextChapter", r.TaskOpenNextChapterHandler)
		auth.POST("/task/uploadAuthCert", r.TaskUploadAuthCertHandler)
		auth.POST("/task/sevenTaskConf", r.SevenTaskConfHandler)
		auth.POST("/task/sevenTaskList", r.SevenTaskListHandler)
		auth.POST("/task/festivalTaskList", r.FestivalTaskListHandler)
		auth.POST("/task/activityTaskList", r.ActivityTaskListHandler)
		auth.POST("/task/achTaskList", r.AchTaskListHandler)
		auth.POST("/task/achTaskCount", r.AchTaskCountHandler)
		auth.POST("/task/recentlyAchTaskList", r.RecentlyAchTaskListHandler)
		auth.POST("/task/recAchReward", r.RecAchRewardHandler)
		auth.POST("/task/roleTaskList", r.RoleTaskListHandler)
		auth.POST("/task/recRoleTaskRw", r.RecRoleTaskRwHandler)

		// --- Activity ---
		auth.POST("/activity/eventCheckInData", r.EventCheckInDataHandler)
		auth.POST("/activity/recEventCheckIn", r.RecEventCheckInHandler)
		auth.POST("/activity/rebateStatus", r.RebateStatusHandler)
		auth.POST("/activity/rebate", r.RebateHandler)
		auth.POST("/activity/themeActivity", r.ThemeActivityHandler)
		auth.POST("/activity/themeStory", r.ThemeStoryHandler)
		auth.POST("/activity/survey", r.SurveyHandler)
		auth.POST("/activity/recThemeActStoryRw", r.RecThemeActStoryRwHandler)
		auth.POST("/activity/sevenSignData", r.SevenSignDataHandler)
		auth.POST("/activity/sevenSign", r.SevenSignHandler)
		auth.POST("/activity/rankingList", r.RankingListHandler)
		auth.POST("/activity/likeRanking", r.LikeRankingHandler)
		auth.POST("/activity/answerList", r.AnswerListHandler)
		auth.POST("/activity/answer", r.AnswerHandler)
		auth.POST("/activity/betStatus", r.BetStatusHandler)
		auth.POST("/activity/betList", r.BetListHandler)
		auth.POST("/activity/betHistory", r.BetHistoryHandler)
		auth.POST("/activity/bet", r.BetHandler)
		auth.POST("/activity/betDetail", r.BetDetailHandler)
		auth.POST("/activity/festivalActivity", r.FestivalActivityHandler)
		auth.POST("/activity/festivalOpenPool", r.FestivalOpenPoolHandler)
		auth.POST("/activity/festivalRewardList", r.FestivalRewardListHandler)

		// --- Player ---
		auth.POST("/player/valid", r.PlayerValidHandler)
		auth.POST("/player/reportGuide", r.PlayerReportGuideHandler)
		auth.POST("/player/guideList", r.PlayerGuideListHandler)
		auth.POST("/player/cancelDeleteAccount", r.PlayerCancelDeleteHandler)
		auth.POST("/player/deleteAccount", r.PlayerDeleteAccountHandler)
		auth.POST("/player/bindStove", r.PlayerBindStoveHandler)
		auth.POST("/player/bindEmail", r.PlayerBindEmailHandler)
		auth.POST("/player/payInfo", r.PlayerPayInfoHandler)
		auth.POST("/player/gameData", r.PlayerGameDataHandler)
		auth.POST("/player/tourRecord", r.PlayerTourRecordHandler)
		auth.POST("/player/updateNickname", r.PlayerUpdateNicknameHandler)
		auth.POST("/player/updateDeclaration", r.PlayerUpdateDeclarationHandler)
		auth.POST("/player/genAssocPwd", r.PlayerGenAssocPwdHandler)
		auth.POST("/player/getAssocPwd", r.PlayerGetAssocPwdHandler)
		auth.POST("/player/assocUser", r.PlayerAssocUserHandler)

		// --- Notice / Role / Draw ---
		auth.POST("/notice/list", r.NoticeListHandler)
		auth.POST("/role/updateNickname", r.PlayerUpdateNicknameHandler)
		auth.POST("/draw/list", r.DrawListHandler)
		auth.POST("/draw/card", r.DrawCardHandler)

		// --- Collection cards ---
		auth.POST("/collCard/list", r.CollCardListHandler)
		auth.POST("/collCard/update", r.CollCardUpdateHandler)
		auth.POST("/collCard/collNum", r.CollCardNumHandler)
		auth.POST("/collCard/detail", r.CollCardDetailHandler)
		auth.POST("/collCard/recentlyCardList", r.CollCardRecentlyListHandler)

		// --- Game social / share ---
		auth.POST("/game/getFollowMedia", r.GetFollowMediaHandler)
		auth.POST("/game/followMedia", r.FollowMediaHandler)
		auth.POST("/game/getBindRw", r.GetBindRwHandler)
		auth.POST("/game/recBindRw", r.RecBindRwHandler)
		auth.POST("/game/getSharePage", r.GetSharePageHandler)
		auth.POST("/game/sharePage", r.SharePageHandler)
		auth.POST("/game/appComment", r.AppCommentHandler)
		auth.POST("/game/getAppComment", r.GetAppCommentHandler)

		// --- VIP / Redemption / Newbie ---
		auth.POST("/vip/data", r.VipDataHandler)
		auth.POST("/vip/reward", r.VipRewardHandler)
		auth.POST("/redemption/exchange", r.RedemptionHandler)
		auth.POST("/newbie/nickname", r.NewbieNicknameHandler)
		auth.POST("/newbie/setNickname", r.PlayerUpdateNicknameHandler)
	}

	r.Engine.GET("/ws", func(c *gin.Context) {
		r.WSSrv.UpgradeHTTP(c.Writer, c.Request)
	})
}

func (r *Router) Run(addr string) error {
	return r.Engine.Run(addr)
}
