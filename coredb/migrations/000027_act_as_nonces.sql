-- +goose Up
CREATE TABLE act_as_nonces (
    subject_domain text NOT NULL,
    grant_id text NOT NULL,
    nonce text NOT NULL,
    expires_at timestamptz NOT NULL,
    PRIMARY KEY (subject_domain, grant_id, nonce)
);
CREATE INDEX act_as_nonces_expiry ON act_as_nonces (expires_at);

-- +goose Down
DROP TABLE act_as_nonces;
