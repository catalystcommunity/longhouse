-- +goose Up

-- Browser sign-ins reuse the CLI session tables. A browser session keeps its
-- refresh token in an httpOnly cookie, and its expiry slides forward each time
-- the browser resumes it. CLI sessions keep a fixed expiry.
ALTER TABLE cli_sessions
    ADD COLUMN kind text NOT NULL DEFAULT 'cli'
        CHECK (kind IN ('cli', 'browser'));

-- +goose Down

ALTER TABLE cli_sessions DROP COLUMN kind;
