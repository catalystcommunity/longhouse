-- +goose Up

-- 000006: complete the event-recurrence model so the recurrence worker
-- can spawn real Event rows for each occurrence (mirroring how tasks
-- already work via tasks.recurrence_root_task_id + tasks.next_recurrence_at).
-- Adds:
--   * events.next_recurrence_at     — when the next child should spawn.
--                                     NULL once the spawner has caught
--                                     up to the 2y horizon, or on rows
--                                     that aren't recurring.
--   * events.recurrence_root_event_id — set on spawned children, NULL on
--                                     the root. Cascading delete on the
--                                     root takes the whole series with it.
--
-- The "delete this and future" flow leaves the root row in place but
-- clears its recurrence_freq + next_recurrence_at, so the spawner moves
-- on; we hard-delete any children with starts_at >= the chosen instance.

ALTER TABLE events
    ADD COLUMN IF NOT EXISTS next_recurrence_at        timestamptz,
    ADD COLUMN IF NOT EXISTS recurrence_root_event_id  uuid
        REFERENCES events(event_id) ON DELETE CASCADE;

CREATE INDEX IF NOT EXISTS events_next_recurrence_due_idx
    ON events(next_recurrence_at)
    WHERE recurrence_freq IS NOT NULL AND next_recurrence_at IS NOT NULL;

CREATE INDEX IF NOT EXISTS events_recurrence_root_idx
    ON events(recurrence_root_event_id)
    WHERE recurrence_root_event_id IS NOT NULL;

-- +goose Down

DROP INDEX IF EXISTS events_recurrence_root_idx;
DROP INDEX IF EXISTS events_next_recurrence_due_idx;
ALTER TABLE events
    DROP COLUMN IF EXISTS recurrence_root_event_id,
    DROP COLUMN IF EXISTS next_recurrence_at;
