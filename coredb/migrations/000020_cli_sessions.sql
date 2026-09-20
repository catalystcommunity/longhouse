-- +goose Up

-- A CLI login starts without credentials. The browser approves the short-lived
-- request with its normal Longhouse bearer. Only hashes of the device and user
-- codes are stored. The device code is the secret that can collect the result.
CREATE TABLE cli_login_requests (
    login_id          uuid        PRIMARY KEY DEFAULT generate_ulid(),
    device_code_hash  bytea       NOT NULL UNIQUE,
    user_code_hash    bytea       NOT NULL UNIQUE,
    client_name       text        NOT NULL,
    status            text        NOT NULL DEFAULT 'pending'
                                  CHECK (status IN ('pending', 'approved', 'denied', 'consumed')),
    subject_domain    text,
    subject_user_id   text,
    display_name      text,
    created_at        timestamptz NOT NULL DEFAULT timezone('utc', now()),
    expires_at        timestamptz NOT NULL,
    approved_at       timestamptz,
    consumed_at       timestamptz
);

CREATE INDEX cli_login_requests_expiry_idx ON cli_login_requests (expires_at);

-- One row is one revocable CLI installation. Its absolute expiry does not
-- move when a refresh token rotates.
CREATE TABLE cli_sessions (
    session_id       uuid        PRIMARY KEY DEFAULT generate_ulid(),
    subject_domain   text        NOT NULL,
    subject_user_id  text        NOT NULL,
    display_name     text        NOT NULL DEFAULT '',
    client_name      text        NOT NULL,
    created_at       timestamptz NOT NULL DEFAULT timezone('utc', now()),
    last_used_at     timestamptz NOT NULL DEFAULT timezone('utc', now()),
    expires_at       timestamptz NOT NULL,
    revoked_at       timestamptz
);

CREATE INDEX cli_sessions_subject_idx
    ON cli_sessions (subject_domain, subject_user_id, created_at DESC);
CREATE INDEX cli_sessions_expiry_idx ON cli_sessions (expires_at);

-- Used refresh-token hashes stay until their session expires. If an old token
-- appears again, the server can detect the reuse and revoke the whole session.
CREATE TABLE cli_refresh_tokens (
    token_hash  bytea        PRIMARY KEY,
    session_id  uuid         NOT NULL REFERENCES cli_sessions(session_id) ON DELETE CASCADE,
    created_at  timestamptz  NOT NULL DEFAULT timezone('utc', now()),
    used_at     timestamptz
);

CREATE INDEX cli_refresh_tokens_session_idx ON cli_refresh_tokens (session_id);
CREATE UNIQUE INDEX cli_refresh_tokens_active_idx
    ON cli_refresh_tokens (session_id) WHERE used_at IS NULL;

-- +goose Down

DROP TABLE cli_refresh_tokens;
DROP TABLE cli_sessions;
DROP TABLE cli_login_requests;
