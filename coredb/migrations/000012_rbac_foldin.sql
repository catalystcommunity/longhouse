-- +goose Up

-- 000012 folds the display-only project_members / project_owners join
-- tables into project_grants (the additive grant model from 000011) and
-- drops them, making project_grants the single source of truth for project
-- access. See docs/rbac.md §9.
--
--   * project_owners  -> project_grants @ full   (governance)
--   * project_members -> project_grants @ edit   (content collaborators)
--
-- Owners win when a member appears in both: insert owners first as full,
-- then insert members as edit only where no row exists yet (ON CONFLICT DO
-- NOTHING keyed on the grant PK).
--
-- Also backfills projects.created_by_member_id from the earliest owner row
-- so the owner-falls-back-to-creator rule has a creator to fall back to for
-- pre-existing projects. After this runs, ProjectService/BugService/
-- comment.go read and write project access through project_grants.

INSERT INTO project_grants (project_id, house_id, grantee_type, grantee_id, access_level, created_at)
SELECT po.project_id, p.house_id, 'member', po.member_id, 'full', po.created_at
FROM project_owners po
JOIN projects p ON p.project_id = po.project_id
ON CONFLICT (project_id, grantee_type, grantee_id) DO UPDATE SET access_level = 'full';

INSERT INTO project_grants (project_id, house_id, grantee_type, grantee_id, access_level, created_at)
SELECT pm.project_id, p.house_id, 'member', pm.member_id, 'edit', pm.created_at
FROM project_members pm
JOIN projects p ON p.project_id = pm.project_id
ON CONFLICT (project_id, grantee_type, grantee_id) DO NOTHING;

-- Seed created_by from the earliest owner so existing projects have a
-- creator fallback. Projects with no owners are left null (the resolver
-- falls back to admins for those).
UPDATE projects p
SET created_by_member_id = sub.member_id
FROM (
    SELECT DISTINCT ON (project_id) project_id, member_id
    FROM project_owners
    ORDER BY project_id, created_at ASC
) sub
WHERE p.project_id = sub.project_id
  AND p.created_by_member_id IS NULL;

DROP INDEX IF EXISTS project_owners_member_idx;
DROP TABLE IF EXISTS project_owners;
DROP INDEX IF EXISTS project_members_member_idx;
DROP TABLE IF EXISTS project_members;

-- +goose Down

-- Recreate the join tables (shape from 000003_csilrpc_shape.sql) and
-- repopulate them from project_grants: full -> owners, full+edit -> members
-- (members was a superset of owners in the old model).
CREATE TABLE project_members (
    project_id uuid        NOT NULL REFERENCES projects(project_id) ON DELETE CASCADE,
    member_id  uuid        NOT NULL REFERENCES members(member_id)   ON DELETE CASCADE,
    created_at timestamptz NOT NULL DEFAULT timezone('utc', now()),
    PRIMARY KEY (project_id, member_id)
);
CREATE INDEX project_members_member_idx ON project_members(member_id);

CREATE TABLE project_owners (
    project_id uuid        NOT NULL REFERENCES projects(project_id) ON DELETE CASCADE,
    member_id  uuid        NOT NULL REFERENCES members(member_id)   ON DELETE CASCADE,
    created_at timestamptz NOT NULL DEFAULT timezone('utc', now()),
    PRIMARY KEY (project_id, member_id)
);
CREATE INDEX project_owners_member_idx ON project_owners(member_id);

INSERT INTO project_owners (project_id, member_id, created_at)
SELECT project_id, grantee_id::uuid, created_at
FROM project_grants
WHERE grantee_type = 'member' AND access_level = 'full'
ON CONFLICT DO NOTHING;

INSERT INTO project_members (project_id, member_id, created_at)
SELECT project_id, grantee_id::uuid, created_at
FROM project_grants
WHERE grantee_type = 'member' AND access_level IN ('edit', 'full')
ON CONFLICT DO NOTHING;
