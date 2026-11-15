-- +goose Up
-- ZellaPay's own staff. They never share tables, sessions or credentials with
-- merchant users. Passkeys (WebAuthn) are their second factor.

CREATE TABLE staff_users (
    id                  text PRIMARY KEY CHECK (id <> ''),
    email               text NOT NULL UNIQUE
                        CHECK (email <> '' AND email = lower(email)),
    name                text NOT NULL CHECK (name <> ''),
    role                text NOT NULL
                        CHECK (role IN ('support', 'risk_analyst', 'finance', 'admin', 'super_admin')),
    status              text NOT NULL CHECK (status IN ('active', 'disabled')),
    password_hash       text NOT NULL CHECK (password_hash <> ''),
    password_changed_at timestamptz NOT NULL,
    created_by          text NOT NULL CHECK (created_by <> ''),
    created_at          timestamptz NOT NULL
);

CREATE TABLE staff_credentials (
    id            text PRIMARY KEY CHECK (id <> ''),
    user_id       text NOT NULL REFERENCES staff_users (id),
    credential_id bytea NOT NULL UNIQUE,
    label         text NOT NULL CHECK (label <> '' AND char_length(label) <= 64),
    data          jsonb NOT NULL,
    created_at    timestamptz NOT NULL,
    last_used_at  timestamptz
);

CREATE INDEX staff_credentials_user_idx ON staff_credentials (user_id);

CREATE TABLE staff_sessions (
    id           text PRIMARY KEY CHECK (id <> ''),
    user_id      text NOT NULL REFERENCES staff_users (id),
    token_hash   bytea NOT NULL UNIQUE CHECK (octet_length(token_hash) = 32),
    created_at   timestamptz NOT NULL,
    last_seen_at timestamptz NOT NULL,
    expires_at   timestamptz NOT NULL,
    mfa_at       timestamptz,
    ip           text NOT NULL DEFAULT '',
    user_agent   text NOT NULL DEFAULT '',
    revoked_at   timestamptz
);

CREATE INDEX staff_sessions_user_open_idx ON staff_sessions (user_id)
    WHERE revoked_at IS NULL;

-- One pending WebAuthn ceremony per session and kind, used once.
CREATE TABLE staff_challenges (
    session_id text NOT NULL REFERENCES staff_sessions (id),
    kind       text NOT NULL CHECK (kind IN ('register', 'login')),
    data       jsonb NOT NULL,
    expires_at timestamptz NOT NULL,
    PRIMARY KEY (session_id, kind)
);

-- +goose Down
DROP TABLE staff_challenges;
DROP TABLE staff_sessions;
DROP TABLE staff_credentials;
DROP TABLE staff_users;
