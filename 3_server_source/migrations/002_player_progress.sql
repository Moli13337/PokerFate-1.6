-- 002_player_progress.sql
-- Persistent storage for player profile extensions, tasks, activities,
-- collection cards and shop purchase limits. Designed to back the HTTP
-- handlers in internal/httpapi/*_handlers.go.

-- =============================================================================
-- Player profile extensions
-- =============================================================================
ALTER TABLE users
    ADD COLUMN IF NOT EXISTS declaration        TEXT     NOT NULL DEFAULT '',
    ADD COLUMN IF NOT EXISTS monthly_card_exp   BIGINT   NOT NULL DEFAULT 0,
    ADD COLUMN IF NOT EXISTS auth_cert_url      TEXT     NOT NULL DEFAULT '',
    ADD COLUMN IF NOT EXISTS auth_cert_time     BIGINT   NOT NULL DEFAULT 0,
    ADD COLUMN IF NOT EXISTS assoc_pwd          VARCHAR(64) NOT NULL DEFAULT '',
    ADD COLUMN IF NOT EXISTS assoc_pwd_expired  BIGINT   NOT NULL DEFAULT 0,
    ADD COLUMN IF NOT EXISTS assoc_err_num      SMALLINT NOT NULL DEFAULT 0,
    ADD COLUMN IF NOT EXISTS delete_time        BIGINT   NOT NULL DEFAULT 0,
    ADD COLUMN IF NOT EXISTS stove_guid         BIGINT   NOT NULL DEFAULT 0,
    ADD COLUMN IF NOT EXISTS favorite_roles     JSONB    NOT NULL DEFAULT '[]';

-- Generic per-player key/value store. Used for client-side persisted flags
-- such as shown system-guide ids, story-progress flags, etc.
CREATE TABLE IF NOT EXISTS user_settings (
    uid       BIGINT  NOT NULL,
    cfg_key   VARCHAR(64) NOT NULL,
    cfg_value JSONB   NOT NULL DEFAULT '{}',
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    PRIMARY KEY (uid, cfg_key)
);

-- =============================================================================
-- Player game statistics (per game_type)
-- =============================================================================
CREATE TABLE IF NOT EXISTS user_game_stats (
    uid                       BIGINT  NOT NULL,
    game_type                 SMALLINT NOT NULL,
    play_times                INT     NOT NULL DEFAULT 0,
    win_play_times            INT     NOT NULL DEFAULT 0,
    profit                    BIGINT  NOT NULL DEFAULT 0,
    fire_power                INT     NOT NULL DEFAULT 0,
    champion_points           INT     NOT NULL DEFAULT 0,
    tour_round                INT     NOT NULL DEFAULT 0,
    tour_win_round            INT     NOT NULL DEFAULT 0,
    tour_max_profit           BIGINT  NOT NULL DEFAULT 0,
    tour_profit               BIGINT  NOT NULL DEFAULT 0,
    pool_entry_rate           INT     NOT NULL DEFAULT 0,
    add_before_flipping_rate  INT     NOT NULL DEFAULT 0,
    three_bet_rate            INT     NOT NULL DEFAULT 0,
    show_hand_rate            INT     NOT NULL DEFAULT 0,
    active_rate               INT     NOT NULL DEFAULT 0,
    c_bete_rate               INT     NOT NULL DEFAULT 0,
    max_profit_cards          JSONB   NOT NULL DEFAULT '{}',
    best_cards                JSONB   NOT NULL DEFAULT '{}',
    updated_at                TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    PRIMARY KEY (uid, game_type)
);

-- Tournament placement history, used for the trend chart.
CREATE TABLE IF NOT EXISTS user_tour_records (
    id         BIGSERIAL PRIMARY KEY,
    uid        BIGINT  NOT NULL,
    game_type  SMALLINT NOT NULL,
    placement  SMALLINT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
CREATE INDEX IF NOT EXISTS idx_user_tour_records_uid_game ON user_tour_records(uid, game_type, created_at DESC);

-- Daily hand counters (level/bond progression). Reset on the day boundary.
CREATE TABLE IF NOT EXISTS user_daily_hands (
    uid       BIGINT  NOT NULL,
    hands     INT     NOT NULL DEFAULT 0,
    sng_hands INT     NOT NULL DEFAULT 0,
    mtt_hands INT     NOT NULL DEFAULT 0,
    reset_at  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    PRIMARY KEY (uid)
);

-- =============================================================================
-- Task system (covers daily / weekly / challenge / seven-day / theme / festival)
-- =============================================================================
-- task_cate: 1=daily, 2=weekly, 3=seven-day, 4=theme activity, 5=festival, 6=challenge, 7=role-task
CREATE TABLE IF NOT EXISTS user_tasks (
    id                 BIGSERIAL PRIMARY KEY,
    uid                BIGINT  NOT NULL,
    task_cate          SMALLINT NOT NULL,
    task_id            INT     NOT NULL,
    instance_id        INT     NOT NULL DEFAULT 0,
    status             SMALLINT NOT NULL DEFAULT 1,
    current_value      INT     NOT NULL DEFAULT 0,
    target_values      JSONB   NOT NULL DEFAULT '[]',
    sort               INT     NOT NULL DEFAULT 0,
    monthly_card_task  BOOLEAN NOT NULL DEFAULT FALSE,
    activity_id        INT     NOT NULL DEFAULT 0,
    role_id            INT     NOT NULL DEFAULT 0,
    created_at         TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at         TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE (uid, task_cate, task_id)
);
CREATE INDEX IF NOT EXISTS idx_user_tasks_uid_cate ON user_tasks(uid, task_cate);

-- Active-point totals and claimed point-reward ids per category.
CREATE TABLE IF NOT EXISTS user_task_points (
    uid                  BIGINT  NOT NULL,
    task_cate            SMALLINT NOT NULL,
    point                INT     NOT NULL DEFAULT 0,
    claimed_reward_ids   JSONB   NOT NULL DEFAULT '[]',
    updated_at           TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    PRIMARY KEY (uid, task_cate)
);

-- Seven-day tutorial chapter progression.
CREATE TABLE IF NOT EXISTS user_seven_day_progress (
    uid                    BIGINT  NOT NULL PRIMARY KEY,
    cur_day                SMALLINT NOT NULL DEFAULT 1,
    status                 SMALLINT NOT NULL DEFAULT 1,
    chapter_rewards_claimed JSONB  NOT NULL DEFAULT '[]',
    auth_cert_url          TEXT    NOT NULL DEFAULT '',
    auth_cert_time         BIGINT  NOT NULL DEFAULT 0,
    updated_at             TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- Achievement progress (per achievement task_id).
-- Note: 001_init.sql created a different user_achievements table; this is the
-- proper schema used by the achievement handlers.
CREATE TABLE IF NOT EXISTS user_achievement_progress (
    uid          BIGINT  NOT NULL,
    task_id      INT     NOT NULL,
    status       SMALLINT NOT NULL DEFAULT 1,
    current_value INT    NOT NULL DEFAULT 0,
    rate         INT     NOT NULL DEFAULT 0,
    finish       BOOLEAN NOT NULL DEFAULT FALSE,
    created_at   TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at   TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    PRIMARY KEY (uid, task_id)
);

-- Achievement metadata: theme-reward claims + cleared achievement ids per theme.
CREATE TABLE IF NOT EXISTS user_achievement_meta (
    uid                       BIGINT  NOT NULL,
    theme_id                  INT     NOT NULL,
    claimed_theme_reward_ids  JSONB   NOT NULL DEFAULT '[]',
    cleared_ach_ids           JSONB   NOT NULL DEFAULT '[]',
    updated_at                TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    PRIMARY KEY (uid, theme_id)
);

-- =============================================================================
-- Collection cards (replays)
-- =============================================================================
-- Summary row is returned by /collCard/list and /collCard/recentlyCardList.
-- Full replay payload (returned by /collCard/detail) is stored in replay_data.
CREATE TABLE IF NOT EXISTS user_collected_cards (
    id              BIGSERIAL PRIMARY KEY,
    uid             BIGINT  NOT NULL,
    gameid          VARCHAR(64) NOT NULL,
    game_type       SMALLINT NOT NULL,
    profit          BIGINT  NOT NULL DEFAULT 0,
    hand_type       INT     NOT NULL DEFAULT 0,
    cards           JSONB   NOT NULL DEFAULT '[]',
    small_blind     INT     NOT NULL DEFAULT 0,
    big_blind       INT     NOT NULL DEFAULT 0,
    ante            INT     NOT NULL DEFAULT 0,
    tour_name       VARCHAR(128) NOT NULL DEFAULT '',
    game_start_time BIGINT  NOT NULL DEFAULT 0,
    replay_data     JSONB   NOT NULL DEFAULT '{}',
    collected       BOOLEAN NOT NULL DEFAULT TRUE,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE (uid, gameid)
);
CREATE INDEX IF NOT EXISTS idx_user_collected_cards_uid ON user_collected_cards(uid, created_at DESC);

-- =============================================================================
-- Shop purchase limits
-- =============================================================================
-- limit_type: 1=daily, 2=weekly, 3=monthly, 4=permanent
CREATE TABLE IF NOT EXISTS user_shop_limits (
    uid                     BIGINT  NOT NULL,
    shop_type               SMALLINT NOT NULL,
    product_id              INT     NOT NULL,
    count                   INT     NOT NULL DEFAULT 0,
    period_count            INT     NOT NULL DEFAULT 0,
    double_reward_claimed   BOOLEAN NOT NULL DEFAULT FALSE,
    reward_claimed          BOOLEAN NOT NULL DEFAULT FALSE,
    first_recharge_claimed  BOOLEAN NOT NULL DEFAULT FALSE,
    limit_type              SMALLINT NOT NULL DEFAULT 0,
    reset_at                TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at              TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    PRIMARY KEY (uid, shop_type, product_id)
);

-- =============================================================================
-- Activity system
-- =============================================================================
-- Sign-in / check-in progress. signin_type: 1=newman, 2=seven-sign
CREATE TABLE IF NOT EXISTS user_signin (
    uid            BIGINT  NOT NULL,
    signin_type    SMALLINT NOT NULL,
    activity_id    INT     NOT NULL,
    days           JSONB   NOT NULL DEFAULT '[]',
    next_sign_ts   BIGINT  NOT NULL DEFAULT 0,
    miss_sign_cnt  INT     NOT NULL DEFAULT 0,
    cycle_ts       BIGINT  NOT NULL DEFAULT 0,
    updated_at     TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    PRIMARY KEY (uid, signin_type, activity_id)
);

-- Generic activity progress. activity_type: theme/festival/rebate/soccer/ranking/survey/...
CREATE TABLE IF NOT EXISTS user_activity_progress (
    uid            BIGINT  NOT NULL,
    activity_type  VARCHAR(32) NOT NULL,
    activity_id    INT     NOT NULL DEFAULT 0,
    data           JSONB   NOT NULL DEFAULT '{}',
    updated_at     TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    PRIMARY KEY (uid, activity_type, activity_id)
);

-- Seven-day quiz answers.
CREATE TABLE IF NOT EXISTS user_question_answers (
    uid          BIGINT  NOT NULL,
    group_id     INT     NOT NULL,
    question_id  INT     NOT NULL,
    answered_at  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    PRIMARY KEY (uid, group_id, question_id)
);

-- Soccer betting entries.
CREATE TABLE IF NOT EXISTS user_soccer_bets (
    id          BIGSERIAL PRIMARY KEY,
    uid         BIGINT  NOT NULL,
    bet_id      INT     NOT NULL,
    bet_area    SMALLINT NOT NULL,
    amount      BIGINT  NOT NULL,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
CREATE INDEX IF NOT EXISTS idx_user_soccer_bets_uid ON user_soccer_bets(uid, bet_id);
