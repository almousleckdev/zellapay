-- +goose Up
-- Webhooks: merchant endpoints, events, deliveries and an attempt log.

CREATE TABLE webhook_endpoints (
    id          text PRIMARY KEY CHECK (id <> ''),
    merchant_id text NOT NULL REFERENCES merchants (id),
    url         text NOT NULL CHECK (url <> '' AND length(url) <= 2048),
    secret_enc  text NOT NULL CHECK (secret_enc <> ''),
    events      text[] NOT NULL CHECK (cardinality(events) > 0),
    description text NOT NULL DEFAULT '',
    status      text NOT NULL DEFAULT 'active' CHECK (status IN ('active', 'disabled')),
    created_at  timestamptz NOT NULL,
    disabled_at timestamptz
);

CREATE INDEX webhook_endpoints_merchant_idx
    ON webhook_endpoints (merchant_id) WHERE status = 'active';

CREATE TABLE webhook_events (
    id          text PRIMARY KEY CHECK (id <> ''),
    merchant_id text NOT NULL REFERENCES merchants (id),
    type        text NOT NULL CHECK (type <> ''),
    object_id   text NOT NULL CHECK (object_id <> ''),
    payload     bytea NOT NULL,
    created_at  timestamptz NOT NULL,
    UNIQUE (type, object_id)
);

CREATE TABLE webhook_deliveries (
    id              bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    event_id        text NOT NULL REFERENCES webhook_events (id),
    endpoint_id     text NOT NULL REFERENCES webhook_endpoints (id),
    status          text NOT NULL CHECK (status IN ('pending', 'succeeded', 'dead')),
    attempts        integer NOT NULL DEFAULT 0 CHECK (attempts >= 0),
    next_attempt_at timestamptz,
    retry_until     timestamptz NOT NULL,
    last_status     integer NOT NULL DEFAULT 0,
    last_error      text NOT NULL DEFAULT '',
    created_at      timestamptz NOT NULL,
    completed_at    timestamptz,
    UNIQUE (event_id, endpoint_id)
);

CREATE INDEX webhook_deliveries_due_idx
    ON webhook_deliveries (next_attempt_at) WHERE status = 'pending';

CREATE TABLE webhook_attempts (
    id          bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    delivery_id bigint NOT NULL REFERENCES webhook_deliveries (id),
    attempt     integer NOT NULL,
    at          timestamptz NOT NULL,
    status_code integer NOT NULL DEFAULT 0,
    error       text NOT NULL DEFAULT '',
    duration_ms integer NOT NULL DEFAULT 0
);

CREATE INDEX webhook_attempts_delivery_idx ON webhook_attempts (delivery_id, id);

-- +goose StatementBegin
CREATE FUNCTION webhooks_reject_change() RETURNS trigger
LANGUAGE plpgsql AS $$
BEGIN
    RAISE EXCEPTION '% is append-only', TG_TABLE_NAME
        USING ERRCODE = 'integrity_constraint_violation';
END;
$$;
-- +goose StatementEnd

CREATE TRIGGER webhook_events_immutable
    BEFORE UPDATE OR DELETE ON webhook_events
    FOR EACH ROW EXECUTE FUNCTION webhooks_reject_change();
CREATE TRIGGER webhook_attempts_immutable
    BEFORE UPDATE OR DELETE ON webhook_attempts
    FOR EACH ROW EXECUTE FUNCTION webhooks_reject_change();
