-- +goose Up
-- Server-side sessions, TOTP credentials and single-use recovery codes.
-- Session tokens and recovery codes are stored only as hashes. The TOTP
-- secret is stored encrypted.

CREATE TABLE sessions (
    id           text PRIMARY KEY CHECK (id <> ''),
    user_id      text NOT NULL REFERENCES merchant_users (id),
    token_hash   bytea NOT NULL UNIQUE CHECK (octet_length(token_hash) = 32),
    created_at   timestamptz NOT NULL,
    last_seen_at timestamptz NOT NULL,
    expires_at   timestamptz NOT NULL,
    mfa_at       timestamptz,
    ip           text NOT NULL DEFAULT '',
    user_agent   text NOT NULL DEFAULT '',
    revoked_at   timestamptz
);

CREATE INDEX sessions_user_open_idx ON sessions (user_id) WHERE revoked_at IS NULL;

CREATE TABLE mfa_credentials (
    user_id      text PRIMARY KEY REFERENCES merchant_users (id),
    secret_enc   text NOT NULL CHECK (secret_enc <> ''),
    confirmed_at timestamptz,
    last_step    bigint NOT NULL DEFAULT 0 CHECK (last_step >= 0),
    created_at   timestamptz NOT NULL
);

CREATE TABLE recovery_codes (
    id         text PRIMARY KEY CHECK (id <> ''),
    user_id    text NOT NULL REFERENCES merchant_users (id),
    code_hash  bytea NOT NULL UNIQUE CHECK (octet_length(code_hash) = 32),
    used_at    timestamptz,
    created_at timestamptz NOT NULL
);

CREATE INDEX recovery_codes_user_idx ON recovery_codes (user_id) WHERE used_at IS NULL;

-- +goose Down
DROP TABLE recovery_codes;
DROP TABLE mfa_credentials;
DROP TABLE sessions;
