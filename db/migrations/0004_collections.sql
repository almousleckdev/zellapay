-- +goose Up
-- Collections: requests for money from a customer's phone, and their events.

ALTER TABLE merchants
    ADD COLUMN fee_bps integer NOT NULL DEFAULT 0
    CHECK (fee_bps BETWEEN 0 AND 10000);

CREATE TABLE collections (
    id                    text PRIMARY KEY CHECK (id <> ''),
    merchant_id           text NOT NULL REFERENCES merchants (id),
    idempotency_key       text NOT NULL CHECK (idempotency_key <> ''),
    merchant_reference    text NOT NULL CHECK (merchant_reference <> ''),
    status                text NOT NULL CHECK (status IN
        ('created', 'pending', 'ambiguous', 'succeeded', 'failed', 'expired')),
    amount                bigint NOT NULL CHECK (amount > 0),
    currency              char(3) NOT NULL CHECK (currency ~ '^[A-Z]{3}$'),
    fee_amount            bigint NOT NULL CHECK (fee_amount >= 0),
    customer_phone_enc    text NOT NULL CHECK (customer_phone_enc <> ''),
    customer_phone_idx    bytea NOT NULL CHECK (length(customer_phone_idx) = 32),
    customer_phone_masked text NOT NULL CHECK (customer_phone_masked <> ''),
    description           text NOT NULL DEFAULT '',
    operator              text NOT NULL DEFAULT '',
    operator_ref          text NOT NULL DEFAULT '',
    failure_code          text NOT NULL DEFAULT '',
    created_at            timestamptz NOT NULL,
    updated_at            timestamptz NOT NULL,
    completed_at          timestamptz,
    next_check_at         timestamptz,
    UNIQUE (merchant_id, idempotency_key),
    CHECK (fee_amount <= amount)
);

CREATE INDEX collections_merchant_idx
    ON collections (merchant_id, created_at DESC, id DESC);
CREATE INDEX collections_due_idx
    ON collections (next_check_at)
    WHERE status IN ('created', 'pending', 'ambiguous');

CREATE TABLE collection_events (
    id            bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    collection_id text NOT NULL REFERENCES collections (id),
    type          text NOT NULL CHECK (type <> ''),
    from_status   text NOT NULL,
    to_status     text NOT NULL,
    detail        text NOT NULL DEFAULT '',
    created_at    timestamptz NOT NULL
);

CREATE INDEX collection_events_collection_idx
    ON collection_events (collection_id, id);

-- +goose StatementBegin
CREATE FUNCTION collections_guard_terminal() RETURNS trigger
LANGUAGE plpgsql AS $$
BEGIN
    IF OLD.status IN ('succeeded', 'failed', 'expired')
       AND NEW.status IS DISTINCT FROM OLD.status THEN
        RAISE EXCEPTION 'collection % is final (%)', OLD.id, OLD.status
            USING ERRCODE = 'integrity_constraint_violation';
    END IF;
    RETURN NEW;
END;
$$;
-- +goose StatementEnd

CREATE TRIGGER collections_terminal_guard
    BEFORE UPDATE ON collections
    FOR EACH ROW EXECUTE FUNCTION collections_guard_terminal();

-- +goose StatementBegin
CREATE FUNCTION collection_events_reject_change() RETURNS trigger
LANGUAGE plpgsql AS $$
BEGIN
    RAISE EXCEPTION 'collection events are append-only'
        USING ERRCODE = 'integrity_constraint_violation';
END;
$$;
-- +goose StatementEnd

CREATE TRIGGER collection_events_immutable
    BEFORE UPDATE OR DELETE ON collection_events
    FOR EACH ROW EXECUTE FUNCTION collection_events_reject_change();
