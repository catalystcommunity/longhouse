-- +goose Up

-- 000017 caches avatar images locally so Longhouse stops depending on the
-- remote avatar_url (a linkkeys-released claim) staying alive. Keyed by a hash
-- of the source URL so identical URLs across members/houses dedupe and a changed
-- URL is a natural cache miss. Bytes live in Postgres (bytea) — avatars are
-- small and few at this scale; a blob/object backend can replace this table
-- behind the same store methods later without touching callers.
--
-- Separate from members on purpose: the image bytes must never be loaded by the
-- ordinary member queries.

CREATE TABLE cached_avatars (
    url_hash     text PRIMARY KEY,              -- sha256 hex of source_url
    source_url   text        NOT NULL,
    content_type text        NOT NULL,
    bytes        bytea       NOT NULL,
    width        integer     NOT NULL,
    height       integer     NOT NULL,
    byte_size    integer     NOT NULL,
    fetched_at   timestamptz NOT NULL DEFAULT timezone('utc', now())
);

-- +goose Down

DROP TABLE cached_avatars;
