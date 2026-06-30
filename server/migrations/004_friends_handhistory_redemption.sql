-- 004_friends_handhistory_redemption.sql
-- Phase 2: friend request state, poker hand history, redemption anti-replay,
-- achievement theme filter.

-- Friend apply/accept state. 1=pending apply, 2=accepted friend.
-- Default 2 keeps any pre-existing rows accepted for backwards compatibility.
ALTER TABLE friends ADD COLUMN IF NOT EXISTS status SMALLINT NOT NULL DEFAULT 2;
ALTER TABLE friends ADD COLUMN IF NOT EXISTS blocked_until BIGINT NOT NULL DEFAULT 0;

-- Poker hand history for GetHandsListREQ (WS) and disconnect reconnect.
CREATE TABLE IF NOT EXISTS user_hand_history (
    id          BIGSERIAL PRIMARY KEY,
    uid         BIGINT NOT NULL,
    gameid      VARCHAR(64) NOT NULL,
    hand_id     INT NOT NULL,
    game_type   SMALLINT NOT NULL DEFAULT 1,
    room_id     INT NOT NULL,
    hands_data  JSONB NOT NULL DEFAULT '{}',
    created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE(uid, gameid)
);
CREATE INDEX IF NOT EXISTS idx_user_hand_history_uid ON user_hand_history(uid, created_at DESC);

-- Redemption code anti-replay: records which uid redeemed which code.
CREATE TABLE IF NOT EXISTS redemption_codes (
    uid         BIGINT NOT NULL,
    code        VARCHAR(64) NOT NULL,
    redeemed_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    PRIMARY KEY(uid, code)
);

-- Achievement theme filter: user_achievement_progress previously had no
-- theme_id column. Add it so loadAchList can filter per theme.
ALTER TABLE user_achievement_progress ADD COLUMN IF NOT EXISTS theme_id INT NOT NULL DEFAULT 0;
CREATE INDEX IF NOT EXISTS idx_user_ach_progress_uid_theme ON user_achievement_progress(uid, theme_id);
