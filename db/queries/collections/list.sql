-- name: ListCollectionsFirstPage :many
SELECT id, merchant_id, idempotency_key, merchant_reference, status, amount,
       currency, fee_amount, customer_phone_idx, customer_phone_masked,
       description, operator, operator_ref, failure_code, created_at,
       updated_at, completed_at, next_check_at
  FROM collections
 WHERE merchant_id = @merchant_id
   AND (cardinality(@statuses::text[]) = 0 OR status = ANY(@statuses::text[]))
   AND (@reference::text = '' OR merchant_reference = @reference::text)
 ORDER BY created_at DESC, id DESC
 LIMIT @batch_size;

-- name: ListCollectionsAfter :many
SELECT c.id, c.merchant_id, c.idempotency_key, c.merchant_reference,
       c.status, c.amount, c.currency, c.fee_amount, c.customer_phone_idx,
       c.customer_phone_masked, c.description, c.operator, c.operator_ref,
       c.failure_code, c.created_at, c.updated_at, c.completed_at,
       c.next_check_at
  FROM collections c
  JOIN collections cursor ON cursor.id = @after_id
 WHERE c.merchant_id = @merchant_id
   AND cursor.merchant_id = @merchant_id
   AND (c.created_at, c.id) < (cursor.created_at, cursor.id)
   AND (cardinality(@statuses::text[]) = 0 OR c.status = ANY(@statuses::text[]))
   AND (@reference::text = '' OR c.merchant_reference = @reference::text)
 ORDER BY c.created_at DESC, c.id DESC
 LIMIT @batch_size;
