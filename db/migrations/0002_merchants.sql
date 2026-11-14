-- +goose Up
-- Merchants and their API keys. Keys are revoked, never deleted.

CREATE TABLE merchants (
    id               text PRIMARY KEY CHECK (id <> ''),
    name             text NOT NULL CHECK (name <> ''),
    status           text NOT NULL DEFAULT 'pending'
                     CHECK (status IN ('pending', 'active', 'suspended')),
    kyc_status       text NOT NULL DEFAULT 'none'
                     CHECK (kyc_status IN
                            ('none', 'submitted', 'verified', 'rejected')),
    default_currency char(3) NOT NULL CHECK (default_currency ~ '^[A-Z]{3}$'),
    created_at       timestamptz NOT NULL,
    updated_at       timestamptz NOT NULL
);

CREATE TABLE merchant_api_keys (
    id           text PRIMARY KEY CHECK (id <> ''),
    merchant_id  text NOT NULL REFERENCES merchants (id),
    lookup_id    text NOT NULL UNIQUE CHECK (lookup_id <> ''),
    secret_hash  bytea NOT NULL CHECK (length(secret_hash) = 32),
    mode         text NOT NULL CHECK (mode IN ('test', 'live')),
    scopes       text[] NOT NULL CHECK (cardinality(scopes) > 0),
    ip_allowlist cidr[] NOT NULL DEFAULT '{}',
    created_at   timestamptz NOT NULL,
    expires_at   timestamptz,
    revoked_at   timestamptz
);

CREATE INDEX merchant_api_keys_merchant_idx
    ON merchant_api_keys (merchant_id);

-- +goose StatementBegin
CREATE FUNCTION merchant_api_keys_reject_delete() RETURNS trigger
LANGUAGE plpgsql AS $$
BEGIN
    RAISE EXCEPTION 'api keys are revoked, never deleted'
        USING ERRCODE = 'integrity_constraint_violation';
END;
$$;
-- +goose StatementEnd

CREATE TRIGGER merchant_api_keys_no_delete
    BEFORE DELETE ON merchant_api_keys
    FOR EACH ROW EXECUTE FUNCTION merchant_api_keys_reject_delete();
