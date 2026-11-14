-- name: InsertPayout :one
INSERT INTO payouts
    (id, merchant_id, idempotency_key, merchant_reference, status, amount,
     currency, fee_amount, recipient_phone_enc, recipient_phone_idx,
     recipient_phone_masked, description, operator, created_at, updated_at,
     next_check_at)
VALUES
    (@id, @merchant_id, @idempotency_key, @merchant_reference, 'created',
     @amount, @currency, @fee_amount, @recipient_phone_enc,
     @recipient_phone_idx, @recipient_phone_masked, @description, @operator,
     @now, @now, @next_check_at)
ON CONFLICT (merchant_id, idempotency_key) DO NOTHING
RETURNING id;

-- name: GetPayout :one
SELECT id, merchant_id, idempotency_key, merchant_reference, status, amount,
       currency, fee_amount, recipient_phone_idx, recipient_phone_masked,
       description, operator, operator_ref, failure_code, created_at,
       updated_at, completed_at, next_check_at
  FROM payouts
 WHERE id = @id;

-- name: GetPayoutByKey :one
SELECT id, merchant_id, idempotency_key, merchant_reference, status, amount,
       currency, fee_amount, recipient_phone_idx, recipient_phone_masked,
       description, operator, operator_ref, failure_code, created_at,
       updated_at, completed_at, next_check_at
  FROM payouts
 WHERE merchant_id = @merchant_id AND idempotency_key = @idempotency_key;

-- name: LockPayout :one
SELECT id, merchant_id, idempotency_key, merchant_reference, status, amount,
       currency, fee_amount, recipient_phone_idx, recipient_phone_masked,
       description, operator, operator_ref, failure_code, created_at,
       updated_at, completed_at, next_check_at
  FROM payouts
 WHERE id = @id
   FOR UPDATE;

-- name: UpdatePayout :exec
UPDATE payouts
   SET status = @status,
       operator = @operator,
       operator_ref = @operator_ref,
       failure_code = @failure_code,
       updated_at = @updated_at,
       completed_at = @completed_at,
       next_check_at = @next_check_at
 WHERE id = @id;

-- name: InsertPayoutEvent :exec
INSERT INTO payout_events
    (payout_id, type, from_status, to_status, detail, created_at)
VALUES
    (@payout_id, @type, @from_status, @to_status, @detail, @created_at);

-- name: ListDuePayouts :many
SELECT id, merchant_id, idempotency_key, merchant_reference, status, amount,
       currency, fee_amount, recipient_phone_idx, recipient_phone_masked,
       description, operator, operator_ref, failure_code, created_at,
       updated_at, completed_at, next_check_at
  FROM payouts
 WHERE status IN ('created', 'held', 'pending', 'ambiguous')
   AND next_check_at <= @now
 ORDER BY next_check_at
 LIMIT @batch_size;

-- name: ListPayoutEvents :many
SELECT type, from_status, to_status, detail, created_at
  FROM payout_events
 WHERE payout_id = @payout_id
 ORDER BY id;

-- name: ListPayoutsFirstPage :many
SELECT id, merchant_id, idempotency_key, merchant_reference, status, amount,
       currency, fee_amount, recipient_phone_idx, recipient_phone_masked,
       description, operator, operator_ref, failure_code, created_at,
       updated_at, completed_at, next_check_at
  FROM payouts
 WHERE merchant_id = @merchant_id
   AND (cardinality(@statuses::text[]) = 0 OR status = ANY(@statuses::text[]))
   AND (@reference::text = '' OR merchant_reference = @reference::text)
 ORDER BY created_at DESC, id DESC
 LIMIT @batch_size;

-- name: ListPayoutsAfter :many
SELECT c.id, c.merchant_id, c.idempotency_key, c.merchant_reference,
       c.status, c.amount, c.currency, c.fee_amount, c.recipient_phone_idx,
       c.recipient_phone_masked, c.description, c.operator, c.operator_ref,
       c.failure_code, c.created_at, c.updated_at, c.completed_at,
       c.next_check_at
  FROM payouts c
  JOIN payouts cursor ON cursor.id = @after_id
 WHERE c.merchant_id = @merchant_id
   AND cursor.merchant_id = @merchant_id
   AND (c.created_at, c.id) < (cursor.created_at, cursor.id)
   AND (cardinality(@statuses::text[]) = 0 OR c.status = ANY(@statuses::text[]))
   AND (@reference::text = '' OR c.merchant_reference = @reference::text)
 ORDER BY c.created_at DESC, c.id DESC
 LIMIT @batch_size;

-- name: InsertPayoutOutbox :exec
INSERT INTO payout_outbox (payout_id, event_type, created_at)
VALUES (@payout_id, @event_type, @created_at)
ON CONFLICT (payout_id, event_type) DO NOTHING;

-- name: ClaimPayoutOutbox :many
SELECT id, payout_id, event_type, created_at
  FROM payout_outbox
 WHERE published_at IS NULL
 ORDER BY id
 LIMIT @batch_size
   FOR UPDATE SKIP LOCKED;

-- name: MarkPayoutOutboxPublished :exec
UPDATE payout_outbox
   SET published_at = @published_at
 WHERE id = ANY(@ids::bigint[])
   AND published_at IS NULL;

