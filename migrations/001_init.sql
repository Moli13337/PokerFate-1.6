-- 001_init.sql
CREATE TABLE users (
    id BIGSERIAL PRIMARY KEY,
    uid BIGINT UNIQUE NOT NULL,
    name VARCHAR(64) NOT NULL DEFAULT '',
    token VARCHAR(256) NOT NULL,
    login_type SMALLINT NOT NULL DEFAULT 1,
    os VARCHAR(16) NOT NULL DEFAULT 'windows',
    imei VARCHAR(128) NOT NULL DEFAULT '',
    email VARCHAR(256),
    password VARCHAR(128),
    vip_level SMALLINT NOT NULL DEFAULT 0,
    level INT NOT NULL DEFAULT 1,
    exp BIGINT NOT NULL DEFAULT 0,
    gold BIGINT NOT NULL DEFAULT 100000,
    avatar INT NOT NULL DEFAULT 0,
    frame INT NOT NULL DEFAULT 0,
    title INT NOT NULL DEFAULT 0,
    using_role_id INT NOT NULL DEFAULT 1001,
    using_skin_id INT NOT NULL DEFAULT 1,
    newer_guide_step INT NOT NULL DEFAULT 9999,
    client_def_str TEXT NOT NULL DEFAULT '',
    lang VARCHAR(8) NOT NULL DEFAULT 'en',
    chnl SMALLINT NOT NULL DEFAULT 2,
    register_time TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    login_time TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    login_ip VARCHAR(45),
    is_deleted BOOLEAN NOT NULL DEFAULT FALSE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE user_items (
    id BIGSERIAL PRIMARY KEY,
    uid BIGINT NOT NULL REFERENCES users(uid),
    item_id INT NOT NULL,
    count INT NOT NULL DEFAULT 0,
    expire_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE(uid, item_id)
);

CREATE TABLE user_roles (
    id BIGSERIAL PRIMARY KEY,
    uid BIGINT NOT NULL REFERENCES users(uid),
    role_id INT NOT NULL,
    star BOOLEAN NOT NULL DEFAULT FALSE,
    bond INT NOT NULL DEFAULT 0,
    awakened BOOLEAN NOT NULL DEFAULT FALSE,
    skins INT[] NOT NULL DEFAULT '{}',
    using_skin INT NOT NULL DEFAULT 1,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE(uid, role_id)
);

CREATE TABLE user_achievements (
    id BIGSERIAL PRIMARY KEY,
    uid BIGINT NOT NULL REFERENCES users(uid),
    category VARCHAR(32) NOT NULL,
    data JSONB NOT NULL DEFAULT '{}',
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE(uid, category)
);

CREATE TABLE friends (
    id BIGSERIAL PRIMARY KEY,
    uid BIGINT NOT NULL,
    friend_uid BIGINT NOT NULL,
    mark VARCHAR(64) NOT NULL DEFAULT '',
    blocked BOOLEAN NOT NULL DEFAULT FALSE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE(uid, friend_uid)
);

CREATE TABLE mails (
    id BIGSERIAL PRIMARY KEY,
    uid BIGINT NOT NULL,
    type SMALLINT NOT NULL DEFAULT 0,
    title VARCHAR(256) NOT NULL DEFAULT '',
    content TEXT NOT NULL DEFAULT '',
    rewards JSONB NOT NULL DEFAULT '[]',
    is_read BOOLEAN NOT NULL DEFAULT FALSE,
    is_received BOOLEAN NOT NULL DEFAULT FALSE,
    expire_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE shop_orders (
    id BIGSERIAL PRIMARY KEY,
    uid BIGINT NOT NULL,
    order_id VARCHAR(128) UNIQUE NOT NULL,
    product_id VARCHAR(128) NOT NULL,
    status SMALLINT NOT NULL DEFAULT 0,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_users_uid ON users(uid);
CREATE INDEX idx_users_email ON users(email);
CREATE INDEX idx_user_items_uid ON user_items(uid);
CREATE INDEX idx_user_roles_uid ON user_roles(uid);
CREATE INDEX idx_friends_uid ON friends(uid);
CREATE INDEX idx_mails_uid ON mails(uid);
CREATE INDEX idx_shop_orders_uid ON shop_orders(uid);
