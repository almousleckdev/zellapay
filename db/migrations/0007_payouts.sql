-- +goose Up
-- Payouts: money sent from a merchant's balance to a phone number, plus the
-- settlement marker that makes collected funds withdrawable.

ALTER TABLE merchants
    ADD COLUMN payout_fee_bps integer NOT NULL DEFAULT 0
    CHECK (payout_fee_bps BETWEEN 0 AND 10000);

ALTER TABLE collections ADD COLUMN settled_at timestamptz;

CREATE INDEX collections_unsettled_idx
    ON collections (completed_at)
    WHERE status = 'succeeded' AND settled_at IS NULL;

CREATE TABLE payouts (
    id                     text PRIMARY KEY CHECK (id <> ''),
    merchant_id            text NOT NULL REFERENCES merchants (id),
    idempotency_key        text NOT NULL CHECK (idempotency_key <> ''),
    merchant_reference     text NOT NULL CHECK (merchant_reference <> ''),
    status                 text NOT NULL CHECK (status IN
        ('created', 'held', 'pending', 'ambiguous', 'succeeded', 'failed', 'rejected')),
    amount                 bigint NOT NULL CHECK (amount > 0),
    currency               char(3) NOT NULL CHECK (currency ~ '^[A-Z]{3}$'),
    fee_amount             bigint NOT NULL CHECK (fee_amount >= 0),
    recipient_phone_enc    text NOT NULL CHECK (recipient_phone_enc <> ''),
    recipient_phone_idx    bytea NOT NULL CHECK (length(recipient_phone_idx) = 32),
    recipient_phone_masked text NOT NULL CHECK (recipient_phone_masked <> ''),
    description            text NOT NULL DEFAULT '',
    operator               text NOT NULL DEFAULT '',
    operator_ref           text NOT NULL DEFAULT '',
    failure_code           text NOT NULL DEFAULT '',
    created_at             timestamptz NOT NULL,
    updated_at             timestamptz NOT NULL,
    completed_at           timestamptz,
    next_check_at          timestamptz,
    UNIQUE (merchant_id, idempotency_key)
);

CREATE INDEX payouts_merchant_idx ON payouts (merchant_id, created_at DESC, id DESC);
CREATE INDEX payouts_due_idx ON payouts (next_check_at)
    WHERE status IN ('created', 'held', 'pending', 'ambiguous');

CREATE TABLE payout_events (
    id          bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    payout_id   text NOT NULL REFERENCES payouts (id),
    type        text NOT NULL CHECK (type <> ''),
    from_status text NOT NULL,
    to_status   text NOT NULL,
    detail      text NOT NULL DEFAULT '',
    created_at  timestamptz NOT NULL
);

CREATE INDEX payout_events_payout_idx ON payout_events (payout_id, id);

CREATE TABLE payout_outbox (
    id           bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    payout_id    text NOT NULL REFERENCES payouts (id),
    event_type   text NOT NULL CHECK (event_type IN ('payout.succeeded', 'payout.failed')),
    created_at   timestamptz NOT NULL,
    published_at timestamptz,
    UNIQUE (payout_id, event_type)
);

CREATE INDEX payout_outbox_unpublished_idx
    ON payout_outbox (id) WHERE published_at IS NULL;

-- +goose StatementBegin
CREATE FUNCTION payouts_guard_terminal() RETURNS trigger
LANGUAGE plpgsql AS $$
BEGIN
    IF OLD.status IN ('succeeded', 'failed', 'rejected')
       AND NEW.status IS DISTINCT FROM OLD.status THEN
        RAISE EXCEPTION 'payout % is final (%)', OLD.id, OLD.status
            USING ERRCODE = 'integrity_constraint_violation';
    END IF;
    RETURN NEW;
END;
$$;
-- +goose StatementEnd

CREATE TRIGGER payouts_terminal_guard
    BEFORE UPDATE ON payouts
    FOR EACH ROW EXECUTE FUNCTION payouts_guard_terminal();

-- +goose StatementBegin
CREATE FUNCTION payouts_reject_change() RETURNS trigger
LANGUAGE plpgsql AS $$
BEGIN
    RAISE EXCEPTION '% is append-only', TG_TABLE_NAME
        USING ERRCODE = 'integrity_constraint_violation';
END;
$$;
-- +goose StatementEnd

CREATE TRIGGER payout_events_immutable
    BEFORE UPDATE OR DELETE ON payout_events
    FOR EACH ROW EXECUTE FUNCTION payouts_reject_change();

-- +goose StatementBegin
CREATE FUNCTION payout_outbox_guard() RETURNS trigger
LANGUAGE plpgsql AS $$
BEGIN
    IF TG_OP = 'DELETE' THEN
        RAISE EXCEPTION 'outbox rows are never deleted'
            USING ERRCODE = 'integrity_constraint_violation';
    END IF;
    IF NEW.id IS DISTINCT FROM OLD.id
       OR NEW.payout_id IS DISTINCT FROM OLD.payout_id
       OR NEW.event_type IS DISTINCT FROM OLD.event_type
       OR NEW.created_at IS DISTINCT FROM OLD.created_at
       OR OLD.published_at IS NOT NULL THEN
        RAISE EXCEPTION 'outbox rows may only be marked published once'
            USING ERRCODE = 'integrity_constraint_violation';
    END IF;
    RETURN NEW;
END;
$$;
-- +goose StatementEnd

CREATE TRIGGER payout_outbox_immutable
    BEFORE UPDATE OR DELETE ON payout_outbox
    FOR EACH ROW EXECUTE FUNCTION payout_outbox_guard();
