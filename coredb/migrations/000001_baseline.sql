-- +goose Up
-- Used for ULID generation
CREATE EXTENSION IF NOT EXISTS pgcrypto;

-- The ULID generator
CREATE OR REPLACE FUNCTION generate_ulid() RETURNS uuid
    AS $$ SELECT (lpad(to_hex(floor(extract(epoch FROM clock_timestamp()) * 1000)::bigint), 12, '0') || encode(gen_random_bytes(10), 'hex'))::uuid $$
    LANGUAGE SQL;

-- Houses are the organizational unit: a company, neighborhood, group, etc.
CREATE TABLE houses (
    house_id    uuid DEFAULT generate_ulid() NOT NULL PRIMARY KEY,
    name        text NOT NULL,
    description text NOT NULL DEFAULT '',
    created_at  timestamptz DEFAULT timezone('utc', now()) NOT NULL,
    updated_at  timestamptz DEFAULT timezone('utc', now()) NOT NULL
);

-- Members are local user records cached from linkkeys identity assertions.
-- Each member belongs to a house and is identified by their linkkeys domain + user ID.
CREATE TABLE members (
    member_id         uuid DEFAULT generate_ulid() NOT NULL PRIMARY KEY,
    house_id          uuid NOT NULL REFERENCES houses ON DELETE CASCADE,
    linkkeys_domain   text NOT NULL,
    linkkeys_user_id  text NOT NULL,
    display_name      text NOT NULL DEFAULT '',
    cached_public_key bytea,
    roles             text[] NOT NULL DEFAULT ARRAY['member']::text[],
    created_at        timestamptz DEFAULT timezone('utc', now()) NOT NULL,
    updated_at        timestamptz DEFAULT timezone('utc', now()) NOT NULL,
    last_seen_at      timestamptz,
    UNIQUE (house_id, linkkeys_domain, linkkeys_user_id)
);
CREATE INDEX members_house_id_idx ON members(house_id);
CREATE INDEX members_linkkeys_identity_idx ON members(linkkeys_domain, linkkeys_user_id);

-- Trusted domains: linkkeys domains whose users are allowed to authenticate
-- into a house without explicit member records.
CREATE TABLE trusted_domains (
    trusted_domain_id uuid DEFAULT generate_ulid() NOT NULL PRIMARY KEY,
    house_id          uuid NOT NULL REFERENCES houses ON DELETE CASCADE,
    domain            text NOT NULL,
    created_at        timestamptz DEFAULT timezone('utc', now()) NOT NULL,
    UNIQUE (house_id, domain)
);
CREATE INDEX trusted_domains_house_id_idx ON trusted_domains(house_id);

-- Events are calendar items with optional time ranges and locations.
CREATE TABLE events (
    event_id    uuid DEFAULT generate_ulid() NOT NULL PRIMARY KEY,
    house_id    uuid NOT NULL REFERENCES houses ON DELETE CASCADE,
    created_by  uuid NOT NULL REFERENCES members ON DELETE SET NULL,
    title       text NOT NULL,
    description text NOT NULL DEFAULT '',
    location    text NOT NULL DEFAULT '',
    starts_at   timestamptz,
    ends_at     timestamptz,
    all_day     boolean NOT NULL DEFAULT false,
    created_at  timestamptz DEFAULT timezone('utc', now()) NOT NULL,
    updated_at  timestamptz DEFAULT timezone('utc', now()) NOT NULL
);
CREATE INDEX events_house_id_idx ON events(house_id);
CREATE INDEX events_starts_at_idx ON events(starts_at);

-- Tasks are actionable items with status tracking and optional assignment.
CREATE TYPE task_status AS ENUM ('open', 'in_progress', 'done', 'cancelled');

CREATE TABLE tasks (
    task_id     uuid DEFAULT generate_ulid() NOT NULL PRIMARY KEY,
    house_id    uuid NOT NULL REFERENCES houses ON DELETE CASCADE,
    created_by  uuid NOT NULL REFERENCES members ON DELETE SET NULL,
    assigned_to uuid REFERENCES members ON DELETE SET NULL,
    title       text NOT NULL,
    description text NOT NULL DEFAULT '',
    status      task_status NOT NULL DEFAULT 'open',
    due_at      timestamptz,
    created_at  timestamptz DEFAULT timezone('utc', now()) NOT NULL,
    updated_at  timestamptz DEFAULT timezone('utc', now()) NOT NULL
);
CREATE INDEX tasks_house_id_idx ON tasks(house_id);
CREATE INDEX tasks_status_idx ON tasks(status);
CREATE INDEX tasks_assigned_to_idx ON tasks(assigned_to);

-- Comments are discussions attached to events or tasks.
CREATE TABLE comments (
    comment_id  uuid DEFAULT generate_ulid() NOT NULL PRIMARY KEY,
    house_id    uuid NOT NULL REFERENCES houses ON DELETE CASCADE,
    member_id   uuid NOT NULL REFERENCES members ON DELETE SET NULL,
    target_type text NOT NULL CHECK (target_type IN ('event', 'task')),
    target_id   uuid NOT NULL,
    body        text NOT NULL,
    created_at  timestamptz DEFAULT timezone('utc', now()) NOT NULL,
    updated_at  timestamptz DEFAULT timezone('utc', now()) NOT NULL
);
CREATE INDEX comments_house_id_idx ON comments(house_id);
CREATE INDEX comments_target_idx ON comments(target_type, target_id);

-- Shares grant per-resource access to external linkkeys users who are not
-- members of the house. Currently only READ access is supported.
CREATE TABLE shares (
    share_id         uuid DEFAULT generate_ulid() NOT NULL PRIMARY KEY,
    house_id         uuid NOT NULL REFERENCES houses ON DELETE CASCADE,
    shared_by        uuid NOT NULL REFERENCES members ON DELETE SET NULL,
    linkkeys_domain  text NOT NULL,
    linkkeys_user_id text NOT NULL,
    resource_type    text NOT NULL CHECK (resource_type IN ('event', 'task', 'house')),
    resource_id      uuid NOT NULL,
    access_level     text NOT NULL DEFAULT 'read' CHECK (access_level IN ('read')),
    created_at       timestamptz DEFAULT timezone('utc', now()) NOT NULL,
    expires_at       timestamptz
);
CREATE INDEX shares_house_id_idx ON shares(house_id);
CREATE INDEX shares_linkkeys_identity_idx ON shares(linkkeys_domain, linkkeys_user_id);
CREATE INDEX shares_resource_idx ON shares(resource_type, resource_id);

-- +goose Down
DROP TABLE IF EXISTS shares;
DROP TABLE IF EXISTS comments;
DROP TABLE IF EXISTS tasks;
DROP TYPE IF EXISTS task_status;
DROP TABLE IF EXISTS events;
DROP TABLE IF EXISTS trusted_domains;
DROP TABLE IF EXISTS members;
DROP TABLE IF EXISTS houses;
DROP FUNCTION IF EXISTS generate_ulid();
