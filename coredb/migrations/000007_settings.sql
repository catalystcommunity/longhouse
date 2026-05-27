-- +goose Up

-- 000007: house-scoped settings. Each (house_id, key) holds one JSON value
-- the SettingsService merges into the EffectiveSettings response the SPA
-- renders against. JSON-typed value lets us add new keys without an ALTER
-- per setting; the CSIL spec is the source of truth for the supported keys
-- and their value shapes.
--
-- All current keys are house-layer (admin-writable). User-layer settings
-- (spanning every house the user belongs to) would land in a parallel
-- `user_settings` table keyed on the linkkeys (domain, user_id) tuple —
-- not added yet because no user-scoped key exists.

CREATE TABLE house_settings (
    house_id   uuid        NOT NULL REFERENCES houses(house_id)  ON DELETE CASCADE,
    key        text        NOT NULL,
    value      jsonb       NOT NULL,
    updated_at timestamptz NOT NULL DEFAULT now(),
    updated_by uuid        REFERENCES members(member_id) ON DELETE SET NULL,
    PRIMARY KEY (house_id, key)
);

-- +goose Down

DROP TABLE IF EXISTS house_settings;
