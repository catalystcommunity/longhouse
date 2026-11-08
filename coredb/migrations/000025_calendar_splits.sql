-- +goose Up
ALTER TABLE events ADD COLUMN edit_pending_member_id uuid;
ALTER TABLE events ADD COLUMN calendar_unbound boolean NOT NULL DEFAULT false;
CREATE TABLE calendar_splits (
    event_id uuid NOT NULL REFERENCES events(event_id) ON DELETE CASCADE,
    revision bigint NOT NULL,
    house_id uuid NOT NULL,
    calendar_id text NOT NULL,
    series_id text NOT NULL,
    member_id uuid NOT NULL,
    idempotency_key text NOT NULL UNIQUE,
    recurrence_id text NOT NULL,
    payload bytea,
    completed boolean NOT NULL DEFAULT false,
    successor_id text NOT NULL DEFAULT '',
    PRIMARY KEY(event_id,revision)
);
CREATE UNIQUE INDEX calendar_splits_pending ON calendar_splits(house_id,series_id) WHERE NOT completed;

-- +goose StatementBegin
CREATE FUNCTION calendar_split_guard() RETURNS trigger LANGUAGE plpgsql AS $$
BEGIN
    IF OLD.edit_pending_member_id IS NOT NULL AND NEW.edit_pending_member_id IS NOT NULL AND
       (NEW.title,NEW.description,NEW.location,NEW.starts_at,NEW.ends_at,NEW.all_day,
        NEW.recurrence_freq,NEW.recurrence_interval,NEW.recurrence_by_weekday,NEW.recurrence_by_setpos,
        NEW.owner_member_id,NEW.revision,NEW.timezone,NEW.recurrence_rule,NEW.deleted_at)
       IS DISTINCT FROM
       (OLD.title,OLD.description,OLD.location,OLD.starts_at,OLD.ends_at,OLD.all_day,
        OLD.recurrence_freq,OLD.recurrence_interval,OLD.recurrence_by_weekday,OLD.recurrence_by_setpos,
        OLD.owner_member_id,OLD.revision,OLD.timezone,OLD.recurrence_rule,OLD.deleted_at) THEN
        RAISE EXCEPTION 'the saved calendar edit is locked';
    END IF;
    RETURN NEW;
END $$;
-- +goose StatementEnd
CREATE TRIGGER calendar_split_guard BEFORE UPDATE ON events FOR EACH ROW EXECUTE FUNCTION calendar_split_guard();

-- +goose Down
DROP TRIGGER calendar_split_guard ON events;
DROP FUNCTION calendar_split_guard();
DROP TABLE calendar_splits;
ALTER TABLE events DROP COLUMN edit_pending_member_id, DROP COLUMN calendar_unbound;
