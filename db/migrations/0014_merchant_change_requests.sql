-- +goose Up
-- A change to a merchant that a second staff member must approve: going live
-- and fee changes. The approver can never be the requester, enforced here.
CREATE TABLE merchant_change_requests (
    id              text PRIMARY KEY CHECK (id <> ''),
    merchant_id     text NOT NULL REFERENCES merchants (id),
    kind            text NOT NULL CHECK (kind IN ('activate', 'fees')),
    fee_bps         integer CHECK (fee_bps BETWEEN 0 AND 10000),
    payout_fee_bps  integer CHECK (payout_fee_bps BETWEEN 0 AND 10000),
    reason          text NOT NULL CHECK (length(btrim(reason)) >= 5),
    requested_by    text NOT NULL CHECK (requested_by <> ''),
    requested_at    timestamptz NOT NULL,
    expires_at      timestamptz NOT NULL,
    status          text NOT NULL DEFAULT 'pending'
                    CHECK (status IN ('pending', 'approved', 'rejected', 'expired')),
    decided_by      text,
    decided_at      timestamptz,
    decision_reason text,
    CHECK (kind <> 'fees' OR (fee_bps IS NOT NULL AND payout_fee_bps IS NOT NULL)),
    CHECK ((status IN ('approved', 'rejected')) = (decided_by IS NOT NULL)),
    CHECK (decided_by IS NULL OR decided_by <> requested_by)
);

-- One open request of each kind per merchant.
CREATE UNIQUE INDEX merchant_change_requests_open
    ON merchant_change_requests (merchant_id, kind) WHERE status = 'pending';

CREATE INDEX merchant_change_requests_by_status
    ON merchant_change_requests (status, requested_at DESC);

-- +goose Down
DROP TABLE merchant_change_requests;
