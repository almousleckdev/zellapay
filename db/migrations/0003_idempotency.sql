-- +goose Up
-- API idempotency records, scoped per merchant. Rows expire and are purged.

CREATE TABLE idempotency_keys (
    scope                 text NOT NULL CHECK (scope <> ''),
    key                   text NOT NULL
                          CHECK (key <> '' AND length(key) <= 255),
    fingerprint           bytea NOT NULL CHECK (length(fingerprint) = 32),
    state                 text NOT NULL
                          CHECK (state IN ('in_progress', 'complete')),
    locked_until          timestamptz NOT NULL,
    response_status       integer,
    response_content_type text,
    response_body         bytea,
    created_at            timestamptz NOT NULL,
    expires_at            timestamptz NOT NULL,
    PRIMARY KEY (scope, key),
    CHECK ((state = 'complete') = (response_status IS NOT NULL))
);

CREATE INDEX idempotency_keys_expires_idx
    ON idempotency_keys (expires_at);
