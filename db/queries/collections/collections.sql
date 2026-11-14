-- name: InsertCollection :one
INSERT INTO collections
    (id, merchant_id, idempotency_key, merchant_reference, status, amount,
     currency, fee_amount, customer_phone_enc, customer_phone_idx,
     customer_phone_masked, description, operator, created_at, updated_at,
     next_check_at)
VALUES
    (@id, @merchant_id, @idempotency_key, @merchant_reference, 'created',
     @amount, @currency, @fee_amount, @customer_phone_enc,
     @customer_phone_idx, @customer_phone_masked, @description, @operator,
     @now, @now, @next_check_at)
ON CONFLICT (merchant_id, idempotency_key) DO NOTHING
RETURNING id;

-- name: GetCollection :one
SELECT id, merchant_id, idempotency_key, merchant_reference, status, amount,
       currency, fee_amount, customer_phone_idx, customer_phone_masked,
       description, operator, operator_ref, failure_code, created_at,
       updated_at, completed_at, next_check_at
  FROM collections
 WHERE id = @id;

-- name: GetCollectionByKey :one
SELECT id, merchant_id, idempotency_key, merchant_reference, status, amount,
       currency, fee_amount, customer_phone_idx, customer_phone_masked,
       description, operator, operator_ref, failure_code, created_at,
       updated_at, completed_at, next_check_at
  FROM collections
 WHERE merchant_id = @merchant_id AND idempotency_key = @idempotency_key;

-- name: LockCollection :one
SELECT id, merchant_id, idempotency_key, merchant_reference, status, amount,
       currency, fee_amount, customer_phone_idx, customer_phone_masked,
       description, operator, operator_ref, failure_code, created_at,
       updated_at, completed_at, next_check_at
  FROM collections
 WHERE id = @id
   FOR UPDATE;

-- name: UpdateCollection :exec
UPDATE collections
   SET status = @status,
       operator = @operator,
       operator_ref = @operator_ref,
       failure_code = @failure_code,
       updated_at = @updated_at,
       completed_at = @completed_at,
       next_check_at = @next_check_at
 WHERE id = @id;

-- name: InsertCollectionEvent :exec
INSERT INTO collection_events
    (collection_id, type, from_status, to_status, detail, created_at)
VALUES
    (@collection_id, @type, @from_status, @to_status, @detail, @created_at);

-- name: ListDueCollections :many
SELECT id, merchant_id, idempotency_key, merchant_reference, status, amount,
       currency, fee_amount, customer_phone_idx, customer_phone_masked,
       description, operator, operator_ref, failure_code, created_at,
       updated_at, completed_at, next_check_at
  FROM collections
 WHERE status IN ('created', 'pending', 'ambiguous')
   AND next_check_at <= @now
 ORDER BY next_check_at
 LIMIT @batch_size;

-- name: ListCollectionEvents :many
SELECT type, from_status, to_status, detail, created_at
  FROM collection_events
 WHERE collection_id = @collection_id
 ORDER BY id;
