-- +goose Up

-- 000018 adds the claim-backed `handle` member field — the member's
-- human-readable linkkeys handle (e.g. "todpunk"). Same two-column pattern as
-- the other claim-backed fields (see 000016_member_claims):
--
--   handle          — the Longhouse-owned value the app displays (@handle)
--   handle_claimed  — shadow mirror of the last value linkkeys released
--
-- The RP now requests `handle` as a REQUIRED claim at sign-in, so every new
-- sign-in releases it and auth.reconcileMemberClaims seeds it; existing members
-- pick it up on their next full sign-in. Reconciliation tracks upstream while
-- handle == handle_claimed and preserves a user override otherwise — identical
-- to email/avatar_url. Unlike email/avatar the UI never lets the user set it
-- (it is receive-only / verification territory), but the mirror column keeps
-- the reconcile logic uniform.

ALTER TABLE members
    ADD COLUMN handle         text NOT NULL DEFAULT '',
    ADD COLUMN handle_claimed text NOT NULL DEFAULT '';

-- +goose Down

ALTER TABLE members
    DROP COLUMN handle,
    DROP COLUMN handle_claimed;
