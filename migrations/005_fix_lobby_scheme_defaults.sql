-- 005_fix_lobby_scheme_defaults.sql
-- 修复 seedDefaultLobbyScheme 早期版本写入的无效默认值。
-- lobby_scene_id=0 会导致客户端 DecorationModel:getCurLobbyScene() 在
-- tpl_props[0] 上崩溃，冻结 LobbyLayer UI。
UPDATE user_dc_schemes
SET lobby_scene_id = 20900001,
    lobby_bgm_list = ARRAY[20700001]::INT[],
    avatar = CASE WHEN avatar = 0 THEN 20110101 ELSE avatar END
WHERE lobby_scene_id = 0;
