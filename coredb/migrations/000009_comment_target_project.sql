-- +goose Up

-- 000009: allow projects to be a comment target. The baseline comments table
-- constrained target_type to ('event', 'task'); discussion threads now also
-- hang off projects. target_id stays a uuid (project_id is a ULID-as-uuid,
-- same as task/event ids), so only the CHECK needs widening.

ALTER TABLE comments
    DROP CONSTRAINT IF EXISTS comments_target_type_check;

ALTER TABLE comments
    ADD CONSTRAINT comments_target_type_check
        CHECK (target_type IN ('event', 'task', 'project'));

-- +goose Down

ALTER TABLE comments
    DROP CONSTRAINT IF EXISTS comments_target_type_check;

ALTER TABLE comments
    ADD CONSTRAINT comments_target_type_check
        CHECK (target_type IN ('event', 'task'));
