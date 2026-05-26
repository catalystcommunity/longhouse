-- +goose Up

-- 000005 adds recurrence_freq + recurrence_interval to events. The model
-- is intentionally simpler than tasks: there's no `next_recurrence_at`
-- column or background spawner — clients expand recurring instances from
-- `starts_at` inside the visible calendar range. Editing the root event
-- edits the whole series; per-instance overrides aren't supported in v1.

ALTER TABLE events
    ADD COLUMN IF NOT EXISTS recurrence_freq     recurrence_freq,
    ADD COLUMN IF NOT EXISTS recurrence_interval integer NOT NULL DEFAULT 1
        CHECK (recurrence_interval >= 1);

-- +goose Down

ALTER TABLE events
    DROP COLUMN IF EXISTS recurrence_interval,
    DROP COLUMN IF EXISTS recurrence_freq;
