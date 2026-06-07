-- +goose Up

-- 000013 adds dependency edges between work items (tasks and projects).
--
-- The model is deliberately one-directional and minimal: a single
-- polymorphic table holds one row per edge, flowing FROM the dependent
-- (the thing that has the dependency) TO the dependency (the thing it
-- requires). Either end may be a task or a project.
--
-- The reverse view ("what depends on X") is NOT stored — it is computed by
-- querying this same table with the columns swapped, served by the
-- dependencies-by-target index below.
--
-- No foreign keys point at tasks/projects: the table is polymorphic (a
-- single column can't reference two tables) and tasks are soft-deleted
-- (so a CASCADE wouldn't fire on the common delete path anyway). Dangling
-- endpoints are filtered at read time; project hard-deletes clear their
-- edges in the delete handler. The house_id FK gives bulk cleanup when a
-- house is removed and scopes the cycle check / reverse lookups.

CREATE TYPE dependency_node_type AS ENUM ('task', 'project');

CREATE TABLE dependencies (
    house_id        uuid                 NOT NULL REFERENCES houses ON DELETE CASCADE,
    dependent_type  dependency_node_type NOT NULL,
    dependent_id    uuid                 NOT NULL,
    dependency_type dependency_node_type NOT NULL,
    dependency_id   uuid                 NOT NULL,
    created_at      timestamptz          NOT NULL DEFAULT timezone('utc', now()),
    PRIMARY KEY (dependent_type, dependent_id, dependency_type, dependency_id)
);

-- Reverse lookup ("what depends on this node?") and the recursive cycle
-- check, which walks forward from the proposed dependency along
-- dependent -> dependency edges.
CREATE INDEX dependencies_dependency_idx ON dependencies (dependency_type, dependency_id);
CREATE INDEX dependencies_house_idx      ON dependencies (house_id);

-- +goose Down

DROP INDEX IF EXISTS dependencies_house_idx;
DROP INDEX IF EXISTS dependencies_dependency_idx;
DROP TABLE IF EXISTS dependencies;
DROP TYPE IF EXISTS dependency_node_type;
