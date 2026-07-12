-- +goose Up

-- 000019 adds per-member calendar views. A member's "calendar view" is the set
-- of OTHER members' calendars they've added, each with an enabled flag —
-- exactly the "Other calendars" list a normal calendar app persists. Stored
-- server-side (not in the browser) so the choice follows the member across
-- devices and survives logout.
--
-- Purely a display preference: events are already house-readable, so this table
-- only records which owners' events a viewer wants rendered. One row per viewer;
-- the subscription list is a JSON array of { subject_member_id, enabled }.

CREATE TABLE member_calendar_views (
    viewer_member_id uuid PRIMARY KEY REFERENCES members(member_id) ON DELETE CASCADE,
    house_id         uuid NOT NULL,
    subscriptions    jsonb NOT NULL DEFAULT '[]'::jsonb,
    created_at       timestamptz NOT NULL DEFAULT now(),
    updated_at       timestamptz NOT NULL DEFAULT now()
);

-- +goose Down

DROP TABLE member_calendar_views;
