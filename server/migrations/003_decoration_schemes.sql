-- 003_decoration_schemes.sql
-- Persistent storage for the decoration-scheme system (装饰方案).
-- Backs the WS handlers pb.GetDCSchemeREQ / SaveDCSchemeREQ / SaveRoomDCSchemeREQ
-- / DeleteDCSchemeREQ / SetSchemeInfoREQ / SetSchemeNameREQ.
--
-- Two scheme kinds live in separate tables:
--   user_dc_schemes        - lobby decoration schemes (DCSchemeItem)
--   user_room_dc_schemes   - in-game/room decoration schemes (RoomDCScheme)
-- Per-user selection state (specify / random list / using) is tracked in
-- user_dc_scheme_state.

-- Lobby decoration schemes. Maps 1:1 to the DCSchemeItem proto.
CREATE TABLE IF NOT EXISTS user_dc_schemes (
    uid            BIGINT      NOT NULL,
    scheme_id      VARCHAR(64) NOT NULL,
    skin_id        INT         NOT NULL DEFAULT 0,
    lobby_scene_id INT         NOT NULL DEFAULT 0,
    lobby_bgm_list INT[]       NOT NULL DEFAULT '{}',
    create_at      BIGINT      NOT NULL DEFAULT 0,
    property_list  INT[]       NOT NULL DEFAULT '{}',
    title          INT         NOT NULL DEFAULT 0,
    frame          INT         NOT NULL DEFAULT 0,
    avatar         INT         NOT NULL DEFAULT 0,
    updated_at     TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    PRIMARY KEY (uid, scheme_id)
);
CREATE INDEX IF NOT EXISTS idx_user_dc_schemes_uid ON user_dc_schemes(uid, create_at);

-- Room/in-game decoration schemes. Maps 1:1 to the RoomDCScheme proto.
CREATE TABLE IF NOT EXISTS user_room_dc_schemes (
    uid               BIGINT       NOT NULL,
    scheme_id         VARCHAR(64)  NOT NULL,
    name              VARCHAR(128) NOT NULL DEFAULT '',
    create_at         BIGINT       NOT NULL DEFAULT 0,
    rand              INT          NOT NULL DEFAULT 0,
    emojis            INT[]        NOT NULL DEFAULT '{}',
    title             INT[]        NOT NULL DEFAULT '{}',
    "table"           INT[]        NOT NULL DEFAULT '{}',
    card_back         INT[]        NOT NULL DEFAULT '{}',
    card_front        INT[]        NOT NULL DEFAULT '{}',
    all_in_animation  INT[]        NOT NULL DEFAULT '{}',
    bgm_list          INT[]        NOT NULL DEFAULT '{}',
    skin_id           INT[]        NOT NULL DEFAULT '{}',
    show_card         INT[]        NOT NULL DEFAULT '{}',
    open_card         INT[]        NOT NULL DEFAULT '{}',
    updated_at        TIMESTAMPTZ  NOT NULL DEFAULT NOW(),
    PRIMARY KEY (uid, scheme_id)
);
CREATE INDEX IF NOT EXISTS idx_user_room_dc_schemes_uid ON user_room_dc_schemes(uid, create_at);

-- Per-user scheme selection state.
--   lobby_specify_scheme / room_specify_scheme - the active "specify" scheme id
--   lobby_rand_schemes  / room_rand_schemes    - the random-pool scheme id list
--   lobby_using_scheme                          - lobby currently-applied scheme id
--   room_mode                                   - 0 specify, 1 random (mirrors proto)
--   rand_dc_scheme_flag                         - deprecated 1.5.6 random flag
CREATE TABLE IF NOT EXISTS user_dc_scheme_state (
    uid                   BIGINT      NOT NULL PRIMARY KEY,
    lobby_specify_scheme  VARCHAR(64) NOT NULL DEFAULT '',
    lobby_rand_schemes    TEXT[]      NOT NULL DEFAULT '{}',
    lobby_using_scheme    VARCHAR(64) NOT NULL DEFAULT '',
    room_specify_scheme   VARCHAR(64) NOT NULL DEFAULT '',
    room_rand_schemes     TEXT[]      NOT NULL DEFAULT '{}',
    room_mode             SMALLINT    NOT NULL DEFAULT 0,
    rand_dc_scheme_flag   SMALLINT    NOT NULL DEFAULT 0,
    updated_at            TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
