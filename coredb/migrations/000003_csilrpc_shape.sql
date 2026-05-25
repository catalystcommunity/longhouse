-- +goose Up

-- 000003 prepares the schema for the CSIL-RPC-only api:
--   * Task: drop the legacy single-assignee column; introduce task_assignees
--     join (set semantics, no ordering guarantees), and add `tag` +
--     `estimate_minutes` columns the dashboard rolls up.
--   * Project: add `category` (free-form display string), and the
--     project_members / project_owners joins the detail page needs.
--   * Milestones: timeline markers per project; `position` orders them
--     visually, `state` enumerates done/current/future.
--
-- The drop of tasks.assigned_to_member_id is destructive — any prior values
-- are lost. Local dev is the only environment running this migration today
-- (production hasn't deployed it), so the loss is acceptable.

-- ---- Tasks ------------------------------------------------------------

ALTER TABLE tasks
    DROP CONSTRAINT IF EXISTS tasks_assigned_exclusive,
    DROP COLUMN  IF EXISTS assigned_to_member_id,
    ADD  COLUMN  IF NOT EXISTS tag              text,
    ADD  COLUMN  IF NOT EXISTS estimate_minutes integer
        CHECK (estimate_minutes IS NULL OR estimate_minutes >= 0);

CREATE TABLE task_assignees (
    task_id    uuid        NOT NULL REFERENCES tasks(task_id)    ON DELETE CASCADE,
    member_id  uuid        NOT NULL REFERENCES members(member_id) ON DELETE CASCADE,
    created_at timestamptz NOT NULL DEFAULT now(),
    PRIMARY KEY (task_id, member_id)
);
CREATE INDEX task_assignees_member_idx ON task_assignees(member_id);

-- ---- Projects ---------------------------------------------------------

ALTER TABLE projects
    ADD COLUMN IF NOT EXISTS category text;

CREATE TABLE project_members (
    project_id uuid        NOT NULL REFERENCES projects(project_id) ON DELETE CASCADE,
    member_id  uuid        NOT NULL REFERENCES members(member_id)   ON DELETE CASCADE,
    created_at timestamptz NOT NULL DEFAULT now(),
    PRIMARY KEY (project_id, member_id)
);
CREATE INDEX project_members_member_idx ON project_members(member_id);

CREATE TABLE project_owners (
    project_id uuid        NOT NULL REFERENCES projects(project_id) ON DELETE CASCADE,
    member_id  uuid        NOT NULL REFERENCES members(member_id)   ON DELETE CASCADE,
    created_at timestamptz NOT NULL DEFAULT now(),
    PRIMARY KEY (project_id, member_id)
);
CREATE INDEX project_owners_member_idx ON project_owners(member_id);

-- ---- Milestones -------------------------------------------------------

CREATE TYPE milestone_state AS ENUM ('done', 'current', 'future');

CREATE TABLE milestones (
    milestone_id uuid            PRIMARY KEY DEFAULT gen_random_uuid(),
    project_id   uuid            NOT NULL REFERENCES projects(project_id) ON DELETE CASCADE,
    label        text            NOT NULL CHECK (length(label) BETWEEN 1 AND 256),
    when_label   text            NOT NULL,
    state        milestone_state NOT NULL,
    position     integer         NOT NULL,
    created_at   timestamptz     NOT NULL DEFAULT now(),
    updated_at   timestamptz     NOT NULL DEFAULT now()
);
CREATE INDEX milestones_project_position_idx ON milestones(project_id, position);

-- +goose Down

DROP INDEX IF EXISTS milestones_project_position_idx;
DROP TABLE IF EXISTS milestones;
DROP TYPE  IF EXISTS milestone_state;

DROP INDEX IF EXISTS project_owners_member_idx;
DROP TABLE IF EXISTS project_owners;
DROP INDEX IF EXISTS project_members_member_idx;
DROP TABLE IF EXISTS project_members;

ALTER TABLE projects DROP COLUMN IF EXISTS category;

DROP INDEX IF EXISTS task_assignees_member_idx;
DROP TABLE IF EXISTS task_assignees;

ALTER TABLE tasks
    DROP COLUMN IF EXISTS estimate_minutes,
    DROP COLUMN IF EXISTS tag,
    ADD  COLUMN IF NOT EXISTS assigned_to_member_id uuid
        REFERENCES members(member_id) ON DELETE SET NULL,
    ADD  CONSTRAINT tasks_assigned_exclusive CHECK (
        assigned_to_member_id IS NULL OR assigned_to_skill_id IS NULL
    );
