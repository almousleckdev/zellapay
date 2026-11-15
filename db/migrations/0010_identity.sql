-- +goose Up
-- Merchant users, their invitations and their single-use email tokens.
-- Tokens are stored only as SHA-256 hashes. Passwords only as argon2id hashes.

CREATE TABLE merchant_users (
    id                text PRIMARY KEY CHECK (id <> ''),
    merchant_id       text NOT NULL REFERENCES merchants (id),
    email             text NOT NULL UNIQUE
                      CHECK (email <> '' AND email = lower(email)),
    name              text NOT NULL CHECK (name <> ''),
    role              text NOT NULL
                      CHECK (role IN ('owner', 'admin', 'developer', 'finance', 'support')),
    status            text NOT NULL CHECK (status IN ('active', 'disabled')),
    locale            text NOT NULL CHECK (locale IN ('fr', 'en')),
    password_hash     text NOT NULL CHECK (password_hash <> ''),
    password_changed_at timestamptz NOT NULL,
    email_verified_at timestamptz,
    created_at        timestamptz NOT NULL
);

CREATE INDEX merchant_users_merchant_idx ON merchant_users (merchant_id);

CREATE TABLE invitations (
    id          text PRIMARY KEY CHECK (id <> ''),
    merchant_id text NOT NULL REFERENCES merchants (id),
    email       text NOT NULL CHECK (email <> '' AND email = lower(email)),
    role        text NOT NULL
                CHECK (role IN ('owner', 'admin', 'developer', 'finance', 'support')),
    locale      text NOT NULL CHECK (locale IN ('fr', 'en')),
    token_hash  bytea NOT NULL UNIQUE CHECK (octet_length(token_hash) = 32),
    invited_by  text NOT NULL CHECK (invited_by <> ''),
    expires_at  timestamptz NOT NULL,
    used_at     timestamptz,
    revoked_at  timestamptz,
    created_at  timestamptz NOT NULL
);

-- At most one open invitation per person and merchant.
CREATE UNIQUE INDEX invitations_open_idx ON invitations (merchant_id, email)
    WHERE used_at IS NULL AND revoked_at IS NULL;

CREATE TABLE email_tokens (
    id         text PRIMARY KEY CHECK (id <> ''),
    user_id    text NOT NULL REFERENCES merchant_users (id),
    purpose    text NOT NULL CHECK (purpose IN ('verify_email', 'reset_password')),
    token_hash bytea NOT NULL UNIQUE CHECK (octet_length(token_hash) = 32),
    expires_at timestamptz NOT NULL,
    used_at    timestamptz,
    created_at timestamptz NOT NULL
);

-- At most one open token per user and purpose: a new request replaces the old.
CREATE UNIQUE INDEX email_tokens_open_idx ON email_tokens (user_id, purpose)
    WHERE used_at IS NULL;

-- +goose Down
DROP TABLE email_tokens;
DROP TABLE invitations;
DROP TABLE merchant_users;
