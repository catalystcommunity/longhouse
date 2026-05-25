-- +goose Up

-- 000004 adds group_skills: a join table letting a Group itself "hold" a
-- Skill, independent of its members' individual skills. The API does NOT
-- transitively merge group skills into a member's skill list — readers
-- (UIs, schedulers) interested in the union should query both surfaces
-- and merge themselves. That avoids hidden coupling and keeps the
-- inheritance semantics explicit.

CREATE TABLE group_skills (
    group_id   uuid        NOT NULL REFERENCES groups(group_id) ON DELETE CASCADE,
    skill_id   uuid        NOT NULL REFERENCES skills(skill_id) ON DELETE CASCADE,
    created_at timestamptz NOT NULL DEFAULT now(),
    PRIMARY KEY (group_id, skill_id)
);
CREATE INDEX group_skills_skill_idx ON group_skills(skill_id);

-- +goose Down

DROP INDEX IF EXISTS group_skills_skill_idx;
DROP TABLE IF EXISTS group_skills;
