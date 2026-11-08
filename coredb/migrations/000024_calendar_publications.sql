-- +goose Up
-- Store the exact request before the first remote call. A retry must use the
-- same key and content, including after a process restart or client upgrade.
CREATE TABLE calendar_publications (
    event_id uuid NOT NULL REFERENCES events(event_id) ON DELETE CASCADE,
    revision bigint NOT NULL,
    calendar_id text NOT NULL,
    idempotency_key text NOT NULL UNIQUE,
    payload bytea,
    remote_event_id text NOT NULL DEFAULT '',
    remote_state text NOT NULL DEFAULT '',
    trash_id text NOT NULL DEFAULT '',
    PRIMARY KEY (event_id, revision)
);

-- A read can discover a committed event before the create receipt is saved.
-- Keep that reference and the original proposal reference as stable aliases.
ALTER TABLE calendar_refs DROP CONSTRAINT calendar_refs_house_id_remote_event_id_recurrence_id_key;
CREATE INDEX calendar_refs_remote_idx ON calendar_refs(house_id,remote_event_id,recurrence_id,ref_id);

-- +goose Down
-- Refuse rollback if aliases exist. Do not remove their links or comments.
ALTER TABLE calendar_refs ADD CONSTRAINT calendar_refs_house_id_remote_event_id_recurrence_id_key UNIQUE(house_id, remote_event_id, recurrence_id);
DROP INDEX calendar_refs_remote_idx;
DROP TABLE calendar_publications;
