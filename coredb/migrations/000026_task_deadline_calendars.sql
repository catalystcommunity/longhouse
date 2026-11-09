-- +goose Up
-- Keep these records until remote cleanup succeeds, including after house deletion.
CREATE TABLE calendar_deadline_sync (
  house_id uuid PRIMARY KEY REFERENCES house_calendars(house_id) ON DELETE CASCADE,
  generation bigint NOT NULL DEFAULT 1,
  synced_generation bigint NOT NULL DEFAULT 0,
  next_attempt_at timestamptz NOT NULL DEFAULT now(),
  last_error text NOT NULL DEFAULT ''
);
CREATE TABLE calendar_categories (
  house_id uuid NOT NULL REFERENCES house_calendars(house_id) ON DELETE CASCADE,
  category text NOT NULL,
  calendar_id text NOT NULL,
  PRIMARY KEY (house_id, category)
);
CREATE TABLE calendar_deadlines (
  house_id uuid NOT NULL,
  category text NOT NULL,
  task_id uuid NOT NULL,
  creation_key text NOT NULL DEFAULT gen_random_uuid()::text,
  create_request bytea NOT NULL,
  remote_event_id text NOT NULL DEFAULT '',
  applied_body bytea,
  PRIMARY KEY (house_id, category, task_id),
  FOREIGN KEY (house_id, category) REFERENCES calendar_categories(house_id, category) ON DELETE CASCADE
);
INSERT INTO calendar_deadline_sync(house_id) SELECT house_id FROM house_calendars;

-- +goose StatementBegin
CREATE FUNCTION calendar_deadlines_changed() RETURNS trigger LANGUAGE plpgsql AS $$
DECLARE h uuid;
BEGIN
  IF TG_TABLE_NAME = 'house_calendars' THEN
    INSERT INTO calendar_deadline_sync(house_id) VALUES (NEW.house_id) ON CONFLICT DO NOTHING;
    RETURN NULL;
  END IF;
  IF TG_TABLE_NAME = 'project_tasks' THEN
    IF TG_OP = 'DELETE' THEN SELECT house_id INTO h FROM projects WHERE project_id=OLD.project_id;
    ELSE SELECT house_id INTO h FROM projects WHERE project_id=NEW.project_id; END IF;
  ELSE
    IF TG_OP = 'DELETE' THEN h := OLD.house_id; ELSE h := NEW.house_id; END IF;
  END IF;
  UPDATE calendar_deadline_sync SET generation=generation+1, next_attempt_at=now() WHERE house_id=h;
  RETURN NULL;
END $$;
-- +goose StatementEnd
CREATE TRIGGER calendar_deadlines_setup AFTER INSERT ON house_calendars FOR EACH ROW EXECUTE FUNCTION calendar_deadlines_changed();
CREATE TRIGGER calendar_deadlines_tasks AFTER INSERT OR UPDATE OR DELETE ON tasks FOR EACH ROW EXECUTE FUNCTION calendar_deadlines_changed();
CREATE TRIGGER calendar_deadlines_projects AFTER INSERT OR UPDATE OR DELETE ON projects FOR EACH ROW EXECUTE FUNCTION calendar_deadlines_changed();
CREATE TRIGGER calendar_deadlines_links AFTER INSERT OR UPDATE OR DELETE ON project_tasks FOR EACH ROW EXECUTE FUNCTION calendar_deadlines_changed();

-- +goose Down
DROP TRIGGER calendar_deadlines_links ON project_tasks;
DROP TRIGGER calendar_deadlines_projects ON projects;
DROP TRIGGER calendar_deadlines_tasks ON tasks;
DROP TRIGGER calendar_deadlines_setup ON house_calendars;
DROP FUNCTION calendar_deadlines_changed();
DROP TABLE calendar_deadlines, calendar_categories, calendar_deadline_sync;
