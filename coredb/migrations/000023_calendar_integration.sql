-- +goose Up
ALTER TABLE events ADD COLUMN calendar_state text NOT NULL DEFAULT 'legacy';
ALTER TABLE events ADD COLUMN revision bigint NOT NULL DEFAULT 1;
ALTER TABLE events ADD COLUMN proposed_by_member_id uuid REFERENCES members(member_id) ON DELETE SET NULL;
ALTER TABLE events ADD COLUMN accepted_at timestamptz;
ALTER TABLE events ADD COLUMN accepted_revision bigint;
ALTER TABLE events ADD COLUMN publication_error text NOT NULL DEFAULT '';
ALTER TABLE events ADD COLUMN timezone text NOT NULL DEFAULT 'UTC';
ALTER TABLE events ADD COLUMN recurrence_rule text NOT NULL DEFAULT '';
ALTER TABLE events ADD CONSTRAINT events_calendar_state CHECK
  (calendar_state IN ('legacy','local','pending','declined','publishing','remote'));

-- No house foreign key: deletion leaves a durable remote cleanup job.
CREATE TABLE house_calendars (
  house_id uuid PRIMARY KEY,
  enabled boolean NOT NULL DEFAULT false,
  calendar_id text NOT NULL DEFAULT '',
  members_group_id text NOT NULL DEFAULT '',
  admins_group_id text NOT NULL DEFAULT '',
  ready boolean NOT NULL DEFAULT false,
  sync_generation bigint NOT NULL DEFAULT 1,
  synced_generation bigint NOT NULL DEFAULT 0,
  sync_pending_since timestamptz NOT NULL DEFAULT now(),
  next_attempt_at timestamptz NOT NULL DEFAULT now(),
  last_error text NOT NULL DEFAULT '',
  deleted boolean NOT NULL DEFAULT false
);
CREATE TABLE calendar_refs (
  ref_id uuid PRIMARY KEY REFERENCES events(event_id) ON DELETE CASCADE,
  house_id uuid NOT NULL,
  remote_event_id text NOT NULL,
  recurrence_id text NOT NULL DEFAULT '',
  UNIQUE (house_id, remote_event_id, recurrence_id)
);
CREATE TABLE calendar_connections (
  member_id uuid PRIMARY KEY REFERENCES members(member_id) ON DELETE CASCADE,
  grant_id text NOT NULL DEFAULT '',
  nonce text NOT NULL DEFAULT '',
  nonce_expires_at timestamptz,
  broken boolean NOT NULL DEFAULT false
);
CREATE TABLE calendar_acceptances (
  event_id uuid NOT NULL,
  revision bigint NOT NULL,
  member_id uuid NOT NULL,
  accepted_at timestamptz NOT NULL DEFAULT now(),
  PRIMARY KEY(event_id,revision)
);
-- Preserve source bodies for migration review and recovery.
CREATE TABLE calendar_legacy_backup (
  event_id uuid PRIMARY KEY,
  house_id uuid NOT NULL,
  body jsonb NOT NULL,
  captured_at timestamptz NOT NULL DEFAULT now()
);

-- The outbox update is part of the membership transaction.
-- +goose StatementBegin
CREATE FUNCTION calendar_membership_changed() RETURNS trigger LANGUAGE plpgsql AS $$
DECLARE h uuid;
BEGIN
  IF TG_TABLE_NAME = 'members' THEN
    IF TG_OP = 'DELETE' THEN h := OLD.house_id; ELSE h := NEW.house_id; END IF;
  ELSIF TG_TABLE_NAME = 'member_roles' THEN
    IF TG_OP = 'DELETE' THEN SELECT house_id INTO h FROM members WHERE member_id=OLD.member_id;
    ELSE SELECT house_id INTO h FROM members WHERE member_id=NEW.member_id; END IF;
  ELSE
    IF TG_OP = 'DELETE' THEN h := OLD.house_id; ELSE h := NEW.house_id; END IF;
  END IF;
  UPDATE house_calendars SET sync_generation=sync_generation+1,
    sync_pending_since=CASE WHEN sync_generation=synced_generation THEN now() ELSE sync_pending_since END,
    next_attempt_at=now() WHERE house_id=h;
  RETURN NULL;
END $$;
-- +goose StatementEnd
CREATE TRIGGER calendar_members_changed AFTER INSERT OR UPDATE OR DELETE ON members FOR EACH ROW EXECUTE FUNCTION calendar_membership_changed();
CREATE TRIGGER calendar_member_roles_changed AFTER INSERT OR UPDATE OR DELETE ON member_roles FOR EACH ROW EXECUTE FUNCTION calendar_membership_changed();
CREATE TRIGGER calendar_roles_changed AFTER INSERT OR UPDATE OR DELETE ON roles FOR EACH ROW EXECUTE FUNCTION calendar_membership_changed();
-- +goose StatementBegin
CREATE FUNCTION calendar_house_deleted() RETURNS trigger LANGUAGE plpgsql AS $$
BEGIN
  UPDATE house_calendars SET deleted=true, ready=false, next_attempt_at=now(),
    sync_generation=sync_generation+1 WHERE house_id=OLD.house_id;
  RETURN NULL;
END $$;
-- +goose StatementEnd
CREATE TRIGGER calendar_house_deleted AFTER DELETE ON houses FOR EACH ROW EXECUTE FUNCTION calendar_house_deleted();

-- +goose StatementBegin
CREATE FUNCTION calendar_event_guard() RETURNS trigger LANGUAGE plpgsql AS $$
BEGIN
  IF TG_OP = 'DELETE' THEN
    IF OLD.calendar_state IN ('publishing','remote') AND EXISTS (SELECT 1 FROM houses WHERE house_id=OLD.house_id) THEN
      RAISE EXCEPTION 'published event references must be retained';
    END IF;
    RETURN OLD;
  END IF;
  IF TG_OP = 'UPDATE' THEN
    IF OLD.calendar_state IN ('publishing','remote') AND NEW.deleted_at IS DISTINCT FROM OLD.deleted_at THEN
      RAISE EXCEPTION 'publication prevents local deletion';
    END IF;
    IF OLD.calendar_state='publishing' AND NEW.calendar_state='publishing' AND
      (NEW.title,NEW.description,NEW.location,NEW.starts_at,NEW.ends_at,NEW.all_day,
       NEW.recurrence_freq,NEW.recurrence_interval,NEW.recurrence_by_weekday,NEW.recurrence_by_setpos,
       NEW.owner_member_id,NEW.revision,NEW.timezone,NEW.recurrence_rule) IS DISTINCT FROM
      (OLD.title,OLD.description,OLD.location,OLD.starts_at,OLD.ends_at,OLD.all_day,
       OLD.recurrence_freq,OLD.recurrence_interval,OLD.recurrence_by_weekday,OLD.recurrence_by_setpos,
       OLD.owner_member_id,OLD.revision,OLD.timezone,OLD.recurrence_rule) THEN
      RAISE EXCEPTION 'the accepted event revision is locked';
    END IF;
  END IF;
  IF TG_OP = 'INSERT' AND NEW.recurrence_root_event_id IS NOT NULL THEN
    IF EXISTS (SELECT 1 FROM house_calendars WHERE house_id=NEW.house_id AND enabled) OR
       NOT EXISTS (SELECT 1 FROM events WHERE event_id=NEW.recurrence_root_event_id AND calendar_state IN ('local','legacy') AND deleted_at IS NULL) THEN
      RAISE EXCEPTION 'local recurrence is not active for this event';
    END IF;
  END IF;
  IF NEW.calendar_state IN ('pending','declined','publishing','remote') THEN
    NEW.next_recurrence_at := NULL;
  END IF;
  RETURN NEW;
END $$;
-- +goose StatementEnd
CREATE TRIGGER calendar_event_guard BEFORE INSERT OR UPDATE OR DELETE ON events FOR EACH ROW EXECUTE FUNCTION calendar_event_guard();

-- +goose Down
DROP TRIGGER calendar_event_guard ON events;
DROP FUNCTION calendar_event_guard();
DROP TRIGGER calendar_house_deleted ON houses;
DROP FUNCTION calendar_house_deleted();
DROP TRIGGER calendar_roles_changed ON roles;
DROP TRIGGER calendar_member_roles_changed ON member_roles;
DROP TRIGGER calendar_members_changed ON members;
DROP FUNCTION calendar_membership_changed();
DROP TABLE calendar_legacy_backup, calendar_acceptances, calendar_connections, calendar_refs, house_calendars;
ALTER TABLE events DROP COLUMN calendar_state, DROP COLUMN revision,
  DROP COLUMN proposed_by_member_id, DROP COLUMN accepted_at,
  DROP COLUMN accepted_revision, DROP COLUMN publication_error,
  DROP COLUMN timezone, DROP COLUMN recurrence_rule;
