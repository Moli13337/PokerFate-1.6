-- VIP reward claim tracking. Each (uid, level_id) pair can only be claimed once.
CREATE TABLE IF NOT EXISTS user_vip_claims (
    uid BIGINT NOT NULL,
    level_id INT NOT NULL,
    claimed_at BIGINT NOT NULL DEFAULT 0,
    PRIMARY KEY (uid, level_id)
);
