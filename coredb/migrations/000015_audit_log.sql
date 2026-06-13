-- +goose Up

-- 000015 adds the per-house audit log: an append-only record of mutations and
-- security events (logins/refreshes/failures), readable only by house admins
-- (enforced in the AuditService handlers, not here).
--
-- Scale: this is "the log of everything", so it is RANGE-partitioned by month
-- on created_at — never one giant table. Inserts hit the current month's
-- partition; retention is a cheap DROP of old monthly partitions; per-house
-- queries are served by the house_id-leading indexes. A rolling-window worker
-- (LONGHOUSE_AUDIT_*) creates partitions ahead and drops aged-out ones. The
-- DEFAULT partition is insurance so an audit write never fails even if the
-- worker hasn't yet created the month's partition.
--
-- No foreign keys: the log must OUTLIVE the rows it references (a deleted house
-- or member must not cascade-delete its audit history), and FKs into a
-- partitioned table aren't supported anyway. Ids are stored as plain values
-- (resource_id is text — resources are polymorphic, some have composite ids).

-- +goose StatementBegin
CREATE TABLE audit_log (
    audit_id         uuid        NOT NULL DEFAULT generate_ulid(),
    created_at       timestamptz NOT NULL DEFAULT timezone('utc', now()),
    house_id         uuid,            -- NULL = global security scope (unattributable auth events)
    actor_member_id  uuid,            -- the acting member in house_id, when known
    actor_domain     text        NOT NULL DEFAULT '',
    actor_user_id    text        NOT NULL DEFAULT '',
    service          text        NOT NULL DEFAULT '',
    method           text        NOT NULL DEFAULT '',
    action           text        NOT NULL,   -- create|update|delete|restore|purge|login|login_failed|logout|refresh|...
    resource_type    text,
    resource_id      text,
    outcome          text        NOT NULL DEFAULT 'ok',  -- ok|denied|error
    before           jsonb,
    after            jsonb,
    detail           jsonb,
    PRIMARY KEY (audit_id, created_at)
) PARTITION BY RANGE (created_at);
-- +goose StatementEnd

-- Indexes declared on the parent propagate to every partition (PG11+).
CREATE INDEX audit_log_house_time_idx     ON audit_log (house_id, created_at DESC);
CREATE INDEX audit_log_house_resource_idx ON audit_log (house_id, resource_type, resource_id);
CREATE INDEX audit_log_house_actor_idx    ON audit_log (house_id, actor_member_id, created_at DESC);

-- Catch-all so inserts never fail before the maintenance worker has created
-- the needed month. The worker creates explicit monthly partitions ahead of
-- time, so in steady state this stays empty.
CREATE TABLE audit_log_default PARTITION OF audit_log DEFAULT;

-- Seed the current month plus the next two so writes route to real monthly
-- partitions immediately (the worker maintains the window thereafter).
-- +goose StatementBegin
DO $$
DECLARE
    base date := date_trunc('month', timezone('utc', now()))::date;
    i    int;
    s    date;
    e    date;
    pname text;
BEGIN
    FOR i IN 0..2 LOOP
        s := (base + (i || ' month')::interval)::date;
        e := (base + ((i + 1) || ' month')::interval)::date;
        pname := 'audit_log_' || to_char(s, 'YYYY_MM');
        EXECUTE format(
            'CREATE TABLE IF NOT EXISTS %I PARTITION OF audit_log FOR VALUES FROM (%L) TO (%L)',
            pname, s, e
        );
    END LOOP;
END $$;
-- +goose StatementEnd

-- +goose Down

DROP TABLE IF EXISTS audit_log;
