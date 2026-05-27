-- +goose Up

-- 000008: extends the recurrence model so monthly/quarterly/yearly schedules
-- can target a specific weekday-in-period (e.g. "second Thursday of the
-- month", "last Tuesday of the quarter"). Adds:
--   * events.recurrence_by_weekday   — ISO-weekday filter (0=Sun..6=Sat),
--                                       NULL = "any day in the period."
--   * events.recurrence_by_setpos    — 1..5 = first..fifth matching weekday;
--                                       -1 = last matching weekday;
--                                       NULL = behaves as before (anchor
--                                       date / weekly fan-out).
--   * tasks.recurrence_by_setpos     — same semantics for task recurrence.
--
-- Drops tasks_by_weekday_only_weekly: with by_setpos, by_weekday is now
-- meaningful for monthly/quarterly/yearly recurrences too. The worker
-- enforces the right combinations.

ALTER TABLE events
    ADD COLUMN IF NOT EXISTS recurrence_by_weekday integer[],
    ADD COLUMN IF NOT EXISTS recurrence_by_setpos  integer
        CHECK (recurrence_by_setpos IS NULL
               OR (recurrence_by_setpos BETWEEN 1 AND 5)
               OR recurrence_by_setpos = -1);

ALTER TABLE tasks
    ADD COLUMN IF NOT EXISTS recurrence_by_setpos integer
        CHECK (recurrence_by_setpos IS NULL
               OR (recurrence_by_setpos BETWEEN 1 AND 5)
               OR recurrence_by_setpos = -1);

ALTER TABLE tasks
    DROP CONSTRAINT IF EXISTS tasks_by_weekday_only_weekly;

-- +goose Down

ALTER TABLE tasks
    ADD CONSTRAINT tasks_by_weekday_only_weekly CHECK (
        recurrence_by_weekday IS NULL OR recurrence_freq = 'weekly'
    );

ALTER TABLE tasks
    DROP COLUMN IF EXISTS recurrence_by_setpos;

ALTER TABLE events
    DROP COLUMN IF EXISTS recurrence_by_setpos,
    DROP COLUMN IF EXISTS recurrence_by_weekday;
