-- +goose Up

-- 000014 generalizes the soft-delete / trash-bin model that `tasks` already
-- had (migration 000002) to every first-class, restorable entity.
--
-- Three columns per table:
--   deleted_at            — NULL = live; non-NULL = in the trash (set at delete)
--   deleted_by_member_id  — who deleted it (no FK, so the actor survives a later
--                           purge of that member; mirrors how the audit log must
--                           outlive the rows it references)
--   deleted_op_id         — groups every row touched by ONE logical delete action
--                           (a recurring series, a "this & future" sweep, a parent
--                           + its edges) so restore can revert the whole batch.
--                           A ULID minted once per delete op (SELECT generate_ulid()).
--
-- Additive and inert: reads only start filtering `deleted_at IS NULL` and deletes
-- only start soft-deleting once the handler/store changes ship. `tasks` keeps its
-- existing `deleted_at`; it only gains the two new grouping columns.

-- tasks already has deleted_at (000002); add the grouping columns only.
ALTER TABLE tasks
    ADD COLUMN deleted_by_member_id uuid,
    ADD COLUMN deleted_op_id        uuid;

ALTER TABLE projects
    ADD COLUMN deleted_at           timestamptz,
    ADD COLUMN deleted_by_member_id uuid,
    ADD COLUMN deleted_op_id        uuid;

ALTER TABLE events
    ADD COLUMN deleted_at           timestamptz,
    ADD COLUMN deleted_by_member_id uuid,
    ADD COLUMN deleted_op_id        uuid;

ALTER TABLE comments
    ADD COLUMN deleted_at           timestamptz,
    ADD COLUMN deleted_by_member_id uuid,
    ADD COLUMN deleted_op_id        uuid;

ALTER TABLE milestones
    ADD COLUMN deleted_at           timestamptz,
    ADD COLUMN deleted_by_member_id uuid,
    ADD COLUMN deleted_op_id        uuid;

-- Members are NOT trashed/deleted — removing a person from a house keeps their
-- record and their owned content (tasks/events/comments stay attributed) and
-- simply denies future login. So members get a deactivation marker instead of
-- the soft-delete trio.
ALTER TABLE members
    ADD COLUMN deactivated_at           timestamptz,
    ADD COLUMN deactivated_by_member_id uuid;

ALTER TABLE roles
    ADD COLUMN deleted_at           timestamptz,
    ADD COLUMN deleted_by_member_id uuid,
    ADD COLUMN deleted_op_id        uuid;

ALTER TABLE skills
    ADD COLUMN deleted_at           timestamptz,
    ADD COLUMN deleted_by_member_id uuid,
    ADD COLUMN deleted_op_id        uuid;

ALTER TABLE groups
    ADD COLUMN deleted_at           timestamptz,
    ADD COLUMN deleted_by_member_id uuid,
    ADD COLUMN deleted_op_id        uuid;

-- Trash listing + purge sweep scan only soft-deleted rows; restore-by-batch
-- looks up an op id. Both are partial indexes (the common case is deleted_at
-- IS NULL, which these skip entirely, keeping them tiny). House-scoped tables
-- lead with house_id so the per-house trash list stays single-tenant.
CREATE INDEX projects_trash_idx     ON projects(house_id, deleted_at) WHERE deleted_at IS NOT NULL;
CREATE INDEX events_trash_idx       ON events(house_id, deleted_at)   WHERE deleted_at IS NOT NULL;
CREATE INDEX comments_trash_idx     ON comments(house_id, deleted_at) WHERE deleted_at IS NOT NULL;
CREATE INDEX members_deactivated_idx ON members(house_id, deactivated_at) WHERE deactivated_at IS NOT NULL;
CREATE INDEX roles_trash_idx        ON roles(house_id, deleted_at)    WHERE deleted_at IS NOT NULL;
CREATE INDEX skills_trash_idx       ON skills(house_id, deleted_at)   WHERE deleted_at IS NOT NULL;
CREATE INDEX groups_trash_idx       ON groups(house_id, deleted_at)   WHERE deleted_at IS NOT NULL;
-- tasks already has tasks_deleted_at_idx; add a house-scoped partial for the list.
CREATE INDEX tasks_trash_idx        ON tasks(house_id, deleted_at)    WHERE deleted_at IS NOT NULL;
-- milestones have no house_id (scoped via project_id).
CREATE INDEX milestones_trash_idx   ON milestones(deleted_at)         WHERE deleted_at IS NOT NULL;

CREATE INDEX tasks_deleted_op_idx      ON tasks(deleted_op_id)      WHERE deleted_op_id IS NOT NULL;
CREATE INDEX projects_deleted_op_idx   ON projects(deleted_op_id)   WHERE deleted_op_id IS NOT NULL;
CREATE INDEX events_deleted_op_idx     ON events(deleted_op_id)     WHERE deleted_op_id IS NOT NULL;
CREATE INDEX comments_deleted_op_idx   ON comments(deleted_op_id)   WHERE deleted_op_id IS NOT NULL;
CREATE INDEX milestones_deleted_op_idx ON milestones(deleted_op_id) WHERE deleted_op_id IS NOT NULL;
CREATE INDEX roles_deleted_op_idx      ON roles(deleted_op_id)      WHERE deleted_op_id IS NOT NULL;
CREATE INDEX skills_deleted_op_idx     ON skills(deleted_op_id)     WHERE deleted_op_id IS NOT NULL;
CREATE INDEX groups_deleted_op_idx     ON groups(deleted_op_id)     WHERE deleted_op_id IS NOT NULL;

-- +goose Down

DROP INDEX IF EXISTS groups_deleted_op_idx;
DROP INDEX IF EXISTS skills_deleted_op_idx;
DROP INDEX IF EXISTS roles_deleted_op_idx;
DROP INDEX IF EXISTS milestones_deleted_op_idx;
DROP INDEX IF EXISTS comments_deleted_op_idx;
DROP INDEX IF EXISTS events_deleted_op_idx;
DROP INDEX IF EXISTS projects_deleted_op_idx;
DROP INDEX IF EXISTS tasks_deleted_op_idx;

DROP INDEX IF EXISTS milestones_trash_idx;
DROP INDEX IF EXISTS tasks_trash_idx;
DROP INDEX IF EXISTS groups_trash_idx;
DROP INDEX IF EXISTS skills_trash_idx;
DROP INDEX IF EXISTS roles_trash_idx;
DROP INDEX IF EXISTS members_deactivated_idx;
DROP INDEX IF EXISTS comments_trash_idx;
DROP INDEX IF EXISTS events_trash_idx;
DROP INDEX IF EXISTS projects_trash_idx;

ALTER TABLE groups   DROP COLUMN IF EXISTS deleted_op_id, DROP COLUMN IF EXISTS deleted_by_member_id, DROP COLUMN IF EXISTS deleted_at;
ALTER TABLE skills   DROP COLUMN IF EXISTS deleted_op_id, DROP COLUMN IF EXISTS deleted_by_member_id, DROP COLUMN IF EXISTS deleted_at;
ALTER TABLE roles    DROP COLUMN IF EXISTS deleted_op_id, DROP COLUMN IF EXISTS deleted_by_member_id, DROP COLUMN IF EXISTS deleted_at;
ALTER TABLE members  DROP COLUMN IF EXISTS deactivated_by_member_id, DROP COLUMN IF EXISTS deactivated_at;
ALTER TABLE milestones DROP COLUMN IF EXISTS deleted_op_id, DROP COLUMN IF EXISTS deleted_by_member_id, DROP COLUMN IF EXISTS deleted_at;
ALTER TABLE comments DROP COLUMN IF EXISTS deleted_op_id, DROP COLUMN IF EXISTS deleted_by_member_id, DROP COLUMN IF EXISTS deleted_at;
ALTER TABLE events   DROP COLUMN IF EXISTS deleted_op_id, DROP COLUMN IF EXISTS deleted_by_member_id, DROP COLUMN IF EXISTS deleted_at;
ALTER TABLE projects DROP COLUMN IF EXISTS deleted_op_id, DROP COLUMN IF EXISTS deleted_by_member_id, DROP COLUMN IF EXISTS deleted_at;
ALTER TABLE tasks    DROP COLUMN IF EXISTS deleted_op_id, DROP COLUMN IF EXISTS deleted_by_member_id;
