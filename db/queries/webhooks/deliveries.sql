-- name: EnqueueDelivery :exec
INSERT INTO webhook_deliveries
    (event_id, endpoint_id, status, next_attempt_at, retry_until, created_at)
VALUES
    (@event_id, @endpoint_id, 'pending', @at, @retry_until, @at)
ON CONFLICT (event_id, endpoint_id) DO NOTHING;

-- name: ClaimDueDeliveries :many
WITH due AS (
    SELECT w.id FROM webhook_deliveries w
      JOIN webhook_endpoints e ON e.id = w.endpoint_id AND e.status = 'active'
     WHERE w.status = 'pending' AND w.next_attempt_at <= @now
     ORDER BY w.next_attempt_at
     LIMIT @batch_size
       FOR UPDATE OF w SKIP LOCKED
)
UPDATE webhook_deliveries d
   SET next_attempt_at = @lease_until
  FROM due
 WHERE d.id = due.id
RETURNING d.id;

-- name: GetDeliveryWork :many
SELECT d.id, d.event_id, d.endpoint_id, d.status, d.attempts,
       d.next_attempt_at, d.retry_until, d.last_status, d.last_error,
       d.created_at, d.completed_at,
       ep.url, ep.secret_enc,
       ev.merchant_id, ev.type, ev.object_id, ev.payload,
       ev.created_at AS event_created_at
  FROM webhook_deliveries d
  JOIN webhook_endpoints ep ON ep.id = d.endpoint_id
  JOIN webhook_events ev ON ev.id = d.event_id
 WHERE d.id = ANY(@ids::bigint[])
 ORDER BY d.id;

-- name: InsertAttempt :exec
INSERT INTO webhook_attempts
    (delivery_id, attempt, at, status_code, error, duration_ms)
VALUES
    (@delivery_id, @attempt, @at, @status_code, @error, @duration_ms);

-- name: UpdateDelivery :exec
UPDATE webhook_deliveries
   SET status = @status,
       attempts = @attempts,
       next_attempt_at = @next_attempt_at,
       last_status = @last_status,
       last_error = @last_error,
       completed_at = @completed_at
 WHERE id = @id;

-- name: RearmDeliveries :execrows
UPDATE webhook_deliveries
   SET status = 'pending', next_attempt_at = @now, retry_until = @retry_until,
       completed_at = NULL
 WHERE event_id = @event_id;

-- name: ListDeliveries :many
SELECT id, event_id, endpoint_id, status, attempts, next_attempt_at,
       retry_until, last_status, last_error, created_at, completed_at
  FROM webhook_deliveries
 WHERE event_id = @event_id
 ORDER BY id;

-- name: ListAttempts :many
SELECT attempt, at, status_code, error, duration_ms
  FROM webhook_attempts
 WHERE delivery_id = @delivery_id
 ORDER BY id;
