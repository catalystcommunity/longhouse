-- +goose Up

-- 000010: the notification feed.
--
-- Two tables, deliberately fan-out-on-write:
--
--   notification_events — one row per real-world event (e.g. a single new
--     comment), regardless of how many people are notified. A self-contained
--     SNAPSHOT: it denormalizes everything needed to render the feed item
--     (actor_name, target_title, body) and holds only OPAQUE references to
--     whatever produced it. target_type/target_id are plain metadata for
--     optional deep-linking — there is intentionally NO foreign key back to
--     tasks/projects/comments, so deleting the source resource never erases
--     the notification and the snapshot still renders on its own.
--
--   notifications — one row PER RECIPIENT per event (no storage thrift: 200
--     watchers => 200 rows). Each points at the shared event snapshot and
--     carries that recipient's own read state. read_at NULL = unread.
--
-- The cull worker prunes by age (default ~6 months) by deleting old
-- notification_events; the ON DELETE CASCADE drops the per-recipient rows.

CREATE TABLE notification_events (
    notification_event_id uuid DEFAULT generate_ulid() NOT NULL PRIMARY KEY,
    house_id              uuid NOT NULL REFERENCES houses ON DELETE CASCADE,
    kind                  text NOT NULL,
    actor_member_id       uuid,                 -- snapshot only; intentionally no FK
    actor_name            text NOT NULL DEFAULT '',
    target_type           text,                 -- 'task' | 'project' (opaque metadata)
    target_id             text,                 -- opaque id; NOT a foreign key
    target_title          text NOT NULL DEFAULT '',
    body                  text NOT NULL DEFAULT '',
    created_at            timestamptz DEFAULT timezone('utc', now()) NOT NULL
);
CREATE INDEX notification_events_house_idx   ON notification_events(house_id);
CREATE INDEX notification_events_created_idx ON notification_events(created_at);

CREATE TABLE notifications (
    notification_id       uuid DEFAULT generate_ulid() NOT NULL PRIMARY KEY,
    notification_event_id uuid NOT NULL REFERENCES notification_events ON DELETE CASCADE,
    house_id              uuid NOT NULL REFERENCES houses ON DELETE CASCADE,
    member_id             uuid NOT NULL REFERENCES members ON DELETE CASCADE,
    read_at               timestamptz,
    created_at            timestamptz DEFAULT timezone('utc', now()) NOT NULL
);
CREATE INDEX notifications_member_created_idx ON notifications(member_id, created_at DESC);
CREATE INDEX notifications_unread_idx         ON notifications(member_id) WHERE read_at IS NULL;
CREATE INDEX notifications_event_idx          ON notifications(notification_event_id);
CREATE INDEX notifications_created_idx        ON notifications(created_at);

-- +goose Down

DROP TABLE IF EXISTS notifications;
DROP TABLE IF EXISTS notification_events;
