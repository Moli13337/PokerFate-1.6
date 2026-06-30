-- 006_vip_exp.sql
-- Adds vip_exp to users so /vip/data can return real progression based on
-- tpl_vip_level.upgrade_exp thresholds. vip_level remains as a denormalized
-- cache for handlers that read it directly (e.g. login.go).
ALTER TABLE users
    ADD COLUMN IF NOT EXISTS vip_exp BIGINT NOT NULL DEFAULT 0;
