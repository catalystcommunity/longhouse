-- +goose Up

-- 000016 adds claim-backed member fields. linkkeys can release user claims
-- (display_name, email, avatar_url, ...) subject to the user's consent and the
-- IDP's policy. Longhouse treats a released claim as an upstream SOURCE that
-- seeds a Longhouse-owned field — never a hard dependency.
--
-- Two columns per used claim:
--   <field>          — the Longhouse-owned value (user-editable; what the app uses)
--   <field>_claimed  — a shadow mirror of the last value linkkeys released
--
-- Reconcile at login (see auth.reconcileMemberClaims): while <field> still
-- equals its mirror the user hasn't touched it, so we track upstream; once the
-- user overrides it (<field> != mirror) we leave their value alone but keep the
-- mirror current so the divergence stays visible and clearing the override
-- re-syncs. A claim that isn't released leaves every column untouched.
--
-- display_name already exists (000001); it only gains its mirror here.

ALTER TABLE members
    ADD COLUMN email                text NOT NULL DEFAULT '',
    ADD COLUMN avatar_url           text NOT NULL DEFAULT '',
    ADD COLUMN display_name_claimed text NOT NULL DEFAULT '',
    ADD COLUMN email_claimed        text NOT NULL DEFAULT '',
    ADD COLUMN avatar_url_claimed   text NOT NULL DEFAULT '';

-- +goose Down

ALTER TABLE members
    DROP COLUMN email,
    DROP COLUMN avatar_url,
    DROP COLUMN display_name_claimed,
    DROP COLUMN email_claimed,
    DROP COLUMN avatar_url_claimed;
