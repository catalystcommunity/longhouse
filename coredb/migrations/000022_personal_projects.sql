-- +goose Up

-- A member's private "Personal" project. TaskService.CreatePersonalTask makes
-- it on first use and puts the member's quick tasks in it, so they are
-- private through containment (a loose task cannot be private). At most one
-- live personal project per member; a deleted one is replaced on next use.
ALTER TABLE projects
    ADD COLUMN personal_member_id uuid REFERENCES members(member_id) ON DELETE CASCADE;

CREATE UNIQUE INDEX projects_personal_member_live_idx
    ON projects (personal_member_id)
    WHERE personal_member_id IS NOT NULL AND deleted_at IS NULL;

-- +goose Down

DROP INDEX projects_personal_member_live_idx;
ALTER TABLE projects DROP COLUMN personal_member_id;
