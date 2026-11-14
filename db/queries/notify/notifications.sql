-- name: InsertNotification :execrows
INSERT INTO notifications
    (id, channel, template, locale, recipient_masked, payload_enc, dedupe_key,
     status, next_attempt_at, retry_until, created_at)
VALUES
    (@id, @channel, @template, @locale, @recipient_masked, @payload_enc,
     sqlc.narg('dedupe_key'), 'pending', @created_at, @retry_until, @created_at)
ON CONFLICT (dedupe_key) DO NOTHING;

-- name: ClaimDueNotifications :many
WITH due AS (
    SELECT n.id FROM notifications n
     WHERE n.status = 'pending' AND n.next_attempt_at <= @now
     ORDER BY n.next_attempt_at
     LIMIT @batch_size
       FOR UPDATE OF n SKIP LOCKED
)
UPDATE notifications n
   SET next_attempt_at = @lease_until
  FROM due
 WHERE n.id = due.id
RETURNING n.id, n.channel, n.template, n.locale, n.payload_enc, n.attempts,
          n.retry_until;

-- name: RecordNotification :exec
UPDATE notifications
   SET status = @status,
       attempts = @attempts,
       next_attempt_at = COALESCE(sqlc.narg('next_attempt_at'), next_attempt_at),
       last_error = @last_error,
       sent_at = sqlc.narg('sent_at'),
       payload_enc = CASE WHEN @status::text <> 'pending' THEN '' ELSE payload_enc END
 WHERE id = @id;

-- name: GetNotification :one
SELECT id, channel, template, recipient_masked, status, attempts, last_error,
       created_at, sent_at, (payload_enc = '') AS payload_erased
  FROM notifications
 WHERE id = @id;
