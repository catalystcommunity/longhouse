-- +goose Up

-- ----------------------------------------------------------------------------
-- Roles & member_roles: replace members.roles[] with relational tables.
-- ----------------------------------------------------------------------------
CREATE TABLE roles (
    role_id     uuid DEFAULT generate_ulid() NOT NULL PRIMARY KEY,
    house_id    uuid NOT NULL REFERENCES houses ON DELETE CASCADE,
    name        text NOT NULL,
    description text NOT NULL DEFAULT '',
    created_at  timestamptz DEFAULT timezone('utc', now()) NOT NULL,
    updated_at  timestamptz DEFAULT timezone('utc', now()) NOT NULL,
    UNIQUE (house_id, name)
);
CREATE INDEX roles_house_id_idx ON roles(house_id);

-- Pre-create canonical admin and member roles for every existing house.
-- New houses created at runtime are responsible for creating their own
-- canonical roles (see SeedInitialAdmin and any future house-creation flow).
INSERT INTO roles (house_id, name, description)
    SELECT house_id, 'admin', 'Full administrative access' FROM houses
    UNION ALL
    SELECT house_id, 'member', 'Standard member' FROM houses;

CREATE TABLE member_roles (
    member_id  uuid NOT NULL REFERENCES members ON DELETE CASCADE,
    role_id    uuid NOT NULL REFERENCES roles ON DELETE CASCADE,
    created_at timestamptz DEFAULT timezone('utc', now()) NOT NULL,
    PRIMARY KEY (member_id, role_id)
);
CREATE INDEX member_roles_role_id_idx ON member_roles(role_id);

-- Backfill member_roles from the legacy members.roles[] array, joining on
-- (house_id, role_name) so each member gets the correct per-house role rows.
INSERT INTO member_roles (member_id, role_id)
    SELECT m.member_id, r.role_id
    FROM members m
    CROSS JOIN LATERAL unnest(m.roles) AS role_name
    JOIN roles r ON r.house_id = m.house_id AND r.name = role_name;

ALTER TABLE members DROP COLUMN roles;

-- ----------------------------------------------------------------------------
-- Skills: free-form per-house tags assigned to members; tasks may be
-- assigned to a skill instead of (or before) an individual member.
-- ----------------------------------------------------------------------------
CREATE TABLE skills (
    skill_id    uuid DEFAULT generate_ulid() NOT NULL PRIMARY KEY,
    house_id    uuid NOT NULL REFERENCES houses ON DELETE CASCADE,
    name        text NOT NULL,
    description text NOT NULL DEFAULT '',
    created_at  timestamptz DEFAULT timezone('utc', now()) NOT NULL,
    updated_at  timestamptz DEFAULT timezone('utc', now()) NOT NULL,
    UNIQUE (house_id, name)
);
CREATE INDEX skills_house_id_idx ON skills(house_id);

CREATE TABLE member_skills (
    member_id  uuid NOT NULL REFERENCES members ON DELETE CASCADE,
    skill_id   uuid NOT NULL REFERENCES skills ON DELETE CASCADE,
    created_at timestamptz DEFAULT timezone('utc', now()) NOT NULL,
    PRIMARY KEY (member_id, skill_id)
);
CREATE INDEX member_skills_skill_id_idx ON member_skills(skill_id);

-- ----------------------------------------------------------------------------
-- Groups: ad-hoc sets of members. Distinct from roles (privilege) and
-- skills (capability) — used for routing, mentions, etc.
-- ----------------------------------------------------------------------------
CREATE TABLE groups (
    group_id    uuid DEFAULT generate_ulid() NOT NULL PRIMARY KEY,
    house_id    uuid NOT NULL REFERENCES houses ON DELETE CASCADE,
    name        text NOT NULL,
    description text NOT NULL DEFAULT '',
    created_at  timestamptz DEFAULT timezone('utc', now()) NOT NULL,
    updated_at  timestamptz DEFAULT timezone('utc', now()) NOT NULL,
    UNIQUE (house_id, name)
);
CREATE INDEX groups_house_id_idx ON groups(house_id);

CREATE TABLE group_members (
    group_id   uuid NOT NULL REFERENCES groups ON DELETE CASCADE,
    member_id  uuid NOT NULL REFERENCES members ON DELETE CASCADE,
    created_at timestamptz DEFAULT timezone('utc', now()) NOT NULL,
    PRIMARY KEY (group_id, member_id)
);
CREATE INDEX group_members_member_id_idx ON group_members(member_id);

-- ----------------------------------------------------------------------------
-- Projects: ordered task collections. The same task may appear in multiple
-- projects with independent positions.
-- ----------------------------------------------------------------------------
CREATE TABLE projects (
    project_id  uuid DEFAULT generate_ulid() NOT NULL PRIMARY KEY,
    house_id    uuid NOT NULL REFERENCES houses ON DELETE CASCADE,
    name        text NOT NULL,
    description text NOT NULL DEFAULT '',
    status      text NOT NULL DEFAULT 'active' CHECK (status IN ('active', 'archived')),
    created_at  timestamptz DEFAULT timezone('utc', now()) NOT NULL,
    updated_at  timestamptz DEFAULT timezone('utc', now()) NOT NULL
);
CREATE INDEX projects_house_id_idx ON projects(house_id);

CREATE TABLE project_tasks (
    project_id uuid NOT NULL REFERENCES projects ON DELETE CASCADE,
    task_id    uuid NOT NULL REFERENCES tasks ON DELETE CASCADE,
    position   integer NOT NULL,
    created_at timestamptz DEFAULT timezone('utc', now()) NOT NULL,
    PRIMARY KEY (project_id, task_id)
);
CREATE INDEX project_tasks_task_id_idx ON project_tasks(task_id);
CREATE INDEX project_tasks_project_position_idx ON project_tasks(project_id, position);

-- ----------------------------------------------------------------------------
-- Member audit log: append-only record of role/skill/group attachments and
-- other admin actions against members. Permissions are intentionally
-- omitted — captured by `action` text + optional jsonb detail.
-- ----------------------------------------------------------------------------
CREATE TABLE member_audits (
    audit_id          uuid DEFAULT generate_ulid() NOT NULL PRIMARY KEY,
    house_id          uuid NOT NULL REFERENCES houses ON DELETE CASCADE,
    subject_member_id uuid NOT NULL REFERENCES members ON DELETE CASCADE,
    actor_member_id   uuid REFERENCES members ON DELETE SET NULL,
    action            text NOT NULL,
    target_type       text,
    target_id         uuid,
    detail            jsonb,
    created_at        timestamptz DEFAULT timezone('utc', now()) NOT NULL
);
CREATE INDEX member_audits_house_id_idx ON member_audits(house_id);
CREATE INDEX member_audits_subject_idx ON member_audits(subject_member_id, created_at DESC);

-- ----------------------------------------------------------------------------
-- Tasks: ownership rename, subtasks, skill-or-member assignment, recurrence,
-- and soft delete (recurrence templates can be soft-deleted to stop
-- spawning new occurrences without losing the historical row).
-- ----------------------------------------------------------------------------
CREATE TYPE recurrence_freq AS ENUM ('hourly', 'daily', 'weekly', 'monthly', 'quarterly', 'yearly');

ALTER TABLE tasks RENAME COLUMN created_by  TO owner_member_id;
ALTER TABLE tasks RENAME COLUMN assigned_to TO assigned_to_member_id;

ALTER TABLE tasks
    ADD COLUMN assigned_to_skill_id    uuid REFERENCES skills ON DELETE SET NULL,
    ADD COLUMN parent_task_id          uuid REFERENCES tasks  ON DELETE CASCADE,
    ADD COLUMN recurrence_freq         recurrence_freq,
    ADD COLUMN recurrence_interval     integer NOT NULL DEFAULT 1 CHECK (recurrence_interval > 0),
    ADD COLUMN recurrence_by_weekday   integer[],
    ADD COLUMN next_recurrence_at      timestamptz,
    ADD COLUMN recurrence_root_task_id uuid REFERENCES tasks ON DELETE SET NULL,
    ADD COLUMN deleted_at              timestamptz,
    ADD CONSTRAINT tasks_assigned_exclusive CHECK (
        assigned_to_member_id IS NULL OR assigned_to_skill_id IS NULL
    ),
    ADD CONSTRAINT tasks_recurrence_consistent CHECK (
        recurrence_freq IS NOT NULL OR next_recurrence_at IS NULL
    ),
    ADD CONSTRAINT tasks_by_weekday_only_weekly CHECK (
        recurrence_by_weekday IS NULL OR recurrence_freq = 'weekly'
    );

CREATE INDEX tasks_parent_task_id_idx       ON tasks(parent_task_id);
CREATE INDEX tasks_recurrence_root_idx      ON tasks(recurrence_root_task_id);
CREATE INDEX tasks_assigned_to_skill_id_idx ON tasks(assigned_to_skill_id);
CREATE INDEX tasks_deleted_at_idx           ON tasks(deleted_at);
CREATE INDEX tasks_next_recurrence_due_idx  ON tasks(next_recurrence_at)
    WHERE next_recurrence_at IS NOT NULL AND deleted_at IS NULL;

-- ----------------------------------------------------------------------------
-- Events: ownership rename only. iCal export uses event_id as the UID at
-- export time; no in-table iCal-specific columns are needed yet.
-- ----------------------------------------------------------------------------
ALTER TABLE events RENAME COLUMN created_by TO owner_member_id;

-- +goose Down

ALTER TABLE events RENAME COLUMN owner_member_id TO created_by;

DROP INDEX IF EXISTS tasks_next_recurrence_due_idx;
DROP INDEX IF EXISTS tasks_deleted_at_idx;
DROP INDEX IF EXISTS tasks_assigned_to_skill_id_idx;
DROP INDEX IF EXISTS tasks_recurrence_root_idx;
DROP INDEX IF EXISTS tasks_parent_task_id_idx;

ALTER TABLE tasks
    DROP CONSTRAINT IF EXISTS tasks_by_weekday_only_weekly,
    DROP CONSTRAINT IF EXISTS tasks_recurrence_consistent,
    DROP CONSTRAINT IF EXISTS tasks_assigned_exclusive,
    DROP COLUMN  IF EXISTS deleted_at,
    DROP COLUMN  IF EXISTS recurrence_root_task_id,
    DROP COLUMN  IF EXISTS next_recurrence_at,
    DROP COLUMN  IF EXISTS recurrence_by_weekday,
    DROP COLUMN  IF EXISTS recurrence_interval,
    DROP COLUMN  IF EXISTS recurrence_freq,
    DROP COLUMN  IF EXISTS parent_task_id,
    DROP COLUMN  IF EXISTS assigned_to_skill_id;

ALTER TABLE tasks RENAME COLUMN assigned_to_member_id TO assigned_to;
ALTER TABLE tasks RENAME COLUMN owner_member_id      TO created_by;

DROP TYPE IF EXISTS recurrence_freq;

DROP TABLE IF EXISTS member_audits;
DROP TABLE IF EXISTS project_tasks;
DROP TABLE IF EXISTS projects;
DROP TABLE IF EXISTS group_members;
DROP TABLE IF EXISTS groups;
DROP TABLE IF EXISTS member_skills;
DROP TABLE IF EXISTS skills;

-- Restore members.roles[] from member_roles before dropping the join.
ALTER TABLE members ADD COLUMN roles text[] NOT NULL DEFAULT ARRAY['member']::text[];
UPDATE members m
    SET roles = COALESCE((
        SELECT array_agg(r.name ORDER BY r.name)
        FROM member_roles mr
        JOIN roles r ON r.role_id = mr.role_id
        WHERE mr.member_id = m.member_id
    ), ARRAY['member']::text[]);

DROP TABLE IF EXISTS member_roles;
DROP TABLE IF EXISTS roles;
