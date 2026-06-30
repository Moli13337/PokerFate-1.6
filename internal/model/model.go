package model

import (
	"encoding/json"
	"time"
)

// User is the core player account record. Fields are kept in sync with the
// users table (migrations 001 and 002).
type User struct {
	ID             int64     `json:"id"`
	UID            int64     `json:"uid"`
	Name           string    `json:"name"`
	Token          string    `json:"-"`
	LoginType      int       `json:"login_type"`
	OS             string    `json:"os"`
	IMEI           string    `json:"imei"`
	Email          string    `json:"email,omitempty"`
	Password       string    `json:"-"`
	VipLevel       int       `json:"vip_level"`
	Level          int       `json:"level"`
	Exp            int64     `json:"exp"`
	Gold           int64     `json:"gold"`
	Avatar         int       `json:"avatar"`
	Frame          int       `json:"frame"`
	Title          int       `json:"title"`
	UsingRoleID    int       `json:"using_role_id"`
	UsingSkinID    int       `json:"using_skin_id"`
	NewerGuideStep int       `json:"newer_guide_step"`
	ClientDefStr   string    `json:"client_def_str"`
	Lang           string    `json:"lang"`
	Chnl           int       `json:"chnl"`
	RegisterTime   time.Time `json:"register_time"`
	LoginTime      time.Time `json:"login_time"`
	LoginIP        string    `json:"login_ip,omitempty"`
	IsDeleted      bool      `json:"is_deleted"`
	// Profile extensions (migration 002).
	Declaration     string          `json:"declaration"`
	MonthlyCardExp  int64           `json:"monthly_card_exp"`
	AuthCertURL     string          `json:"auth_cert_url"`
	AuthCertTime    int64           `json:"auth_cert_time"`
	AssocPwd        string          `json:"assoc_pwd"`
	AssocPwdExpired int64           `json:"assoc_pwd_expired"`
	AssocErrNum     int             `json:"assoc_err_num"`
	DeleteTime      int64           `json:"delete_time"`
	StoveGUID       int64           `json:"stove_guid"`
	FavoriteRoles   json.RawMessage `json:"favorite_roles"`
}

// Item is an inventory row.
type Item struct {
	ID       int64      `json:"id"`
	UID      int64      `json:"uid"`
	ItemID   int        `json:"item_id"`
	Count    int        `json:"count"`
	ExpireAt *time.Time `json:"expire_at,omitempty"`
}

// Role is a player-owned character.
type Role struct {
	ID        int64 `json:"id"`
	UID       int64 `json:"uid"`
	RoleID    int   `json:"role_id"`
	Star      bool  `json:"star"`
	Bond      int   `json:"bond"`
	Awakened  bool  `json:"awakened"`
	Skins     []int `json:"skins"`
	UsingSkin int   `json:"using_skin"`
}

// Mail is a system email entry.
type Mail struct {
	ID         int64      `json:"id"`
	UID        int64      `json:"uid"`
	Type       int        `json:"type"`
	Title      string     `json:"title"`
	Content    string     `json:"content"`
	Rewards    string     `json:"rewards"`
	IsRead     bool       `json:"is_read"`
	IsReceived bool       `json:"is_received"`
	ExpireAt   *time.Time `json:"expire_at,omitempty"`
	CreatedAt  time.Time  `json:"created_at"`
}

// ShopOrder is a purchase order (pending or completed).
type ShopOrder struct {
	ID        int64     `json:"id"`
	UID       int64     `json:"uid"`
	OrderID   string    `json:"order_id"`
	ProductID string    `json:"product_id"`
	Status    int       `json:"status"`
	CreatedAt time.Time `json:"created_at"`
}

// GameStat is per-game-type statistics for /player/gameData.
type GameStat struct {
	UID                   int64           `json:"uid"`
	GameType              int             `json:"game_type"`
	PlayTimes             int             `json:"play_times"`
	WinPlayTimes          int             `json:"win_play_times"`
	Profit                int64           `json:"profit"`
	FirePower             int             `json:"fire_power"`
	ChampionPoints        int             `json:"champion_points"`
	TourRound             int             `json:"tour_round"`
	TourWinRound          int             `json:"tour_win_round"`
	TourMaxProfit         int64           `json:"tour_max_profit"`
	TourProfit            int64           `json:"tour_profit"`
	PoolEntryRate         int             `json:"pool_entry_rate"`
	AddBeforeFlippingRate int             `json:"add_before_flipping_rate"`
	ThreeBetRate          int             `json:"three_bet_rate"`
	ShowHandRate          int             `json:"show_hand_rate"`
	ActiveRate            int             `json:"active_rate"`
	CBeteRate             int             `json:"c_bete_rate"`
	MaxProfitCards        json.RawMessage `json:"max_profit_cards"`
	BestCards             json.RawMessage `json:"best_cards"`
}

// TourRecord is a single tournament placement entry.
type TourRecord struct {
	ID        int64     `json:"id"`
	UID       int64     `json:"uid"`
	GameType  int       `json:"game_type"`
	Placement int       `json:"placement"`
	CreatedAt time.Time `json:"created_at"`
}

// DailyHands tracks the daily hand counters for level/bond progression.
type DailyHands struct {
	UID      int64     `json:"uid"`
	Hands    int       `json:"hands"`
	SngHands int       `json:"sng_hands"`
	MttHands int       `json:"mtt_hands"`
	ResetAt  time.Time `json:"reset_at"`
}

// UserTask is a task instance owned by a player.
// TaskCate values: 1=daily, 2=weekly, 3=seven-day, 4=theme activity,
// 5=festival, 6=challenge, 7=role-task.
type UserTask struct {
	ID              int64           `json:"id"`
	UID             int64           `json:"uid"`
	TaskCate        int             `json:"task_cate"`
	TaskID          int             `json:"task_id"`
	InstanceID      int             `json:"instance_id"`
	Status          int             `json:"status"`
	CurrentValue    int             `json:"current_value"`
	TargetValues    json.RawMessage `json:"target_values"`
	Sort            int             `json:"sort"`
	MonthlyCardTask bool            `json:"monthly_card_task"`
	ActivityID      int             `json:"activity_id"`
	RoleID          int             `json:"role_id"`
	CreatedAt       time.Time       `json:"created_at"`
	UpdatedAt       time.Time       `json:"updated_at"`
}

// UserTaskPoint is the active-point total + claimed reward ids per category.
type UserTaskPoint struct {
	UID              int64           `json:"uid"`
	TaskCate         int             `json:"task_cate"`
	Point            int             `json:"point"`
	ClaimedRewardIDs json.RawMessage `json:"claimed_reward_ids"`
}

// SevenDayProgress is the seven-day tutorial chapter progression.
type SevenDayProgress struct {
	UID                   int64           `json:"uid"`
	CurDay                int             `json:"cur_day"`
	Status                int             `json:"status"`
	ChapterRewardsClaimed json.RawMessage `json:"chapter_rewards_claimed"`
	AuthCertURL           string          `json:"auth_cert_url"`
	AuthCertTime          int64           `json:"auth_cert_time"`
}

// AchievementProgress is per-achievement progress.
type AchievementProgress struct {
	UID          int64     `json:"uid"`
	TaskID       int       `json:"task_id"`
	Status       int       `json:"status"`
	CurrentValue int       `json:"current_value"`
	Rate         int       `json:"rate"`
	Finish       bool      `json:"finish"`
	CreatedAt    time.Time `json:"created_at"`
}

// AchievementMeta is per-theme metadata (claimed theme rewards + cleared ach ids).
type AchievementMeta struct {
	UID                   int64           `json:"uid"`
	ThemeID               int             `json:"theme_id"`
	ClaimedThemeRewardIDs json.RawMessage `json:"claimed_theme_reward_ids"`
	ClearedAchIDs         json.RawMessage `json:"cleared_ach_ids"`
}

// CollectedCard is a saved replay card.
type CollectedCard struct {
	ID            int64           `json:"id"`
	UID           int64           `json:"uid"`
	GameID        string          `json:"gameid"`
	GameType      int             `json:"game_type"`
	Profit        int64           `json:"profit"`
	HandType      int             `json:"hand_type"`
	Cards         json.RawMessage `json:"cards"`
	SmallBlind    int             `json:"small_blind"`
	BigBlind      int             `json:"big_blind"`
	Ante          int             `json:"ante"`
	TourName      string          `json:"tour_name"`
	GameStartTime int64           `json:"game_start_time"`
	ReplayData    json.RawMessage `json:"replay_data"`
	Collected     bool            `json:"collected"`
	CreatedAt     time.Time       `json:"created_at"`
}

// ShopLimit tracks purchase limits and reward-claim flags per product.
type ShopLimit struct {
	UID                  int64     `json:"uid"`
	ShopType             int       `json:"shop_type"`
	ProductID            int       `json:"product_id"`
	Count                int       `json:"count"`
	PeriodCount          int       `json:"period_count"`
	DoubleRewardClaimed  bool      `json:"double_reward_claimed"`
	RewardClaimed        bool      `json:"reward_claimed"`
	FirstRechargeClaimed bool      `json:"first_recharge_claimed"`
	LimitType            int       `json:"limit_type"`
	ResetAt              time.Time `json:"reset_at"`
}

// SignIn is sign-in / check-in progress.
// SignInType: 1=newman check-in, 2=seven-day sign-in.
type SignIn struct {
	UID         int64           `json:"uid"`
	SignInType  int             `json:"signin_type"`
	ActivityID  int             `json:"activity_id"`
	Days        json.RawMessage `json:"days"`
	NextSignTs  int64           `json:"next_sign_ts"`
	MissSignCnt int             `json:"miss_sign_cnt"`
	CycleTs     int64           `json:"cycle_ts"`
}

// ActivityProgress is generic per-activity progress.
type ActivityProgress struct {
	UID          int64           `json:"uid"`
	ActivityType string          `json:"activity_type"`
	ActivityID   int             `json:"activity_id"`
	Data         json.RawMessage `json:"data"`
}

// UserSetting is a generic per-player key/value setting entry.
type UserSetting struct {
	UID   int64           `json:"uid"`
	Key   string          `json:"cfg_key"`
	Value json.RawMessage `json:"cfg_value"`
}

// Constants mirroring client-side enum values (app/EnumConfig.lua).
const (
	TaskCateDaily     = 1
	TaskCateWeekly    = 2
	TaskCateSevenDay  = 3
	TaskCateTheme     = 4
	TaskCateFestival  = 5
	TaskCateChallenge = 6
	TaskCateRole      = 7

	TaskStatusInProgress = 1
	TaskStatusCompleted  = 2
	TaskStatusReceived   = 3

	SevenDayStatusRunning   = 1
	SevenDayStatusRewarded  = 2
	SevenDayStatusCompleted = 3
	SevenDayStatusUpload    = 4

	ShopLimitTypeDaily     = 1
	ShopLimitTypeWeekly    = 2
	ShopLimitTypeMonthly   = 3
	ShopLimitTypePermanent = 4

	SignInTypeNewman   = 1
	SignInTypeSevenDay = 2
)

// RewardItem is the standard reward shape used by item_list responses.
type RewardItem struct {
	MajorType int `json:"major_type"`
	ItemID    int `json:"item_id"`
	Num       int `json:"num"`
}
