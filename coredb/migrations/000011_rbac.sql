-- +goose Up

-- 000011 adds resource-level access control & privacy for tasks and
-- projects. See docs/rbac.md for the full model. Summary:
--
--   * Four access levels everywhere: none < read < edit < full.
--   * `visibility` on tasks/projects is the "house-at-large" surface: the
--     level a house member gets when they reach the resource through no
--     project and no grant. Default 'read' preserves today's behaviour
--     (every house member can see everything), so this migration is inert
--     until the authz resolver ships.
--   * Per-resource grant tables (task_grants, project_grants) carry additive
--     grants to members (and, soon, groups). Effective access is the MAX over
--     every surface that reaches the caller (house default, each containing
--     project, grants, owner/admin). External sharing stays in `shares`.
--   * Projects gain created_by_member_id so "owner falls back to creator"
--     works (tasks already satisfy it: owner_member_id IS the creator).
--
-- Deliberately additive only. The metadata-only project_members /
-- project_owners joins are NOT touched here: they're still the live source
-- of truth for ProjectService/BugService/comment fan-out. Folding them into
-- project_grants (owners -> full, members -> read) and dropping them happens
-- in the same change that rewrites those handlers + regenerates CSIL.

CREATE TYPE access_level AS ENUM ('none', 'read', 'edit', 'full');
CREATE TYPE grantee_type AS ENUM ('member', 'group');

-- Visibility: the house-at-large surface. Default 'read' == current behaviour.
ALTER TABLE tasks    ADD COLUMN visibility access_level NOT NULL DEFAULT 'read';
ALTER TABLE projects ADD COLUMN visibility access_level NOT NULL DEFAULT 'read';

-- Projects get a creator so owner-falls-back-to-creator works (tasks already
-- have it via owner_member_id, which is the renamed created_by column).
ALTER TABLE projects
    ADD COLUMN created_by_member_id uuid REFERENCES members ON DELETE SET NULL;

-- Additive per-resource grants. PK dedupes a grantee per resource; the
-- grantee_idx serves "what can this member/group see in this house?" and the
-- list-page filter. Indexes lead with house_id (tenant = shard boundary).
CREATE TABLE task_grants (
    task_id      uuid         NOT NULL REFERENCES tasks  ON DELETE CASCADE,
    house_id     uuid         NOT NULL REFERENCES houses ON DELETE CASCADE,
    grantee_type grantee_type NOT NULL,
    grantee_id   uuid         NOT NULL,
    access_level access_level NOT NULL,
    created_at   timestamptz  NOT NULL DEFAULT timezone('utc', now()),
    PRIMARY KEY (task_id, grantee_type, grantee_id)
);
CREATE INDEX task_grants_grantee_idx ON task_grants (house_id, grantee_type, grantee_id);

CREATE TABLE project_grants (
    project_id   uuid         NOT NULL REFERENCES projects ON DELETE CASCADE,
    house_id     uuid         NOT NULL REFERENCES houses   ON DELETE CASCADE,
    grantee_type grantee_type NOT NULL,
    grantee_id   uuid         NOT NULL,
    access_level access_level NOT NULL,
    created_at   timestamptz  NOT NULL DEFAULT timezone('utc', now()),
    PRIMARY KEY (project_id, grantee_type, grantee_id)
);
CREATE INDEX project_grants_grantee_idx ON project_grants (house_id, grantee_type, grantee_id);

-- +goose Down

DROP INDEX IF EXISTS project_grants_grantee_idx;
DROP TABLE IF EXISTS project_grants;
DROP INDEX IF EXISTS task_grants_grantee_idx;
DROP TABLE IF EXISTS task_grants;

ALTER TABLE projects DROP COLUMN IF EXISTS created_by_member_id;
ALTER TABLE projects DROP COLUMN IF EXISTS visibility;
ALTER TABLE tasks    DROP COLUMN IF EXISTS visibility;

DROP TYPE IF EXISTS grantee_type;
DROP TYPE IF EXISTS access_level;
