-- name: InsertKey :one
INSERT INTO idempotency_keys
    (scope, key, fingerprint, state, locked_until, created_at, expires_at)
VALUES
    (@scope, @key, @fingerprint, 'in_progress', @locked_until, @now,
     @expires_at)
ON CONFLICT (scope, key) DO NOTHING
RETURNING key;

-- name: TakeOverKey :one
UPDATE idempotency_keys
   SET fingerprint = @fingerprint,
       state = 'in_progress',
       locked_until = @locked_until,
       created_at = @now,
       expires_at = @expires_at,
       response_status = NULL,
       response_content_type = NULL,
       response_body = NULL
 WHERE scope = @scope
   AND key = @key
   AND (expires_at <= @now
        OR (state = 'in_progress'
            AND locked_until <= @now
            AND fingerprint = @fingerprint))
RETURNING key;

-- name: GetKey :one
SELECT fingerprint, state, response_status, response_content_type,
       response_body
  FROM idempotency_keys
 WHERE scope = @scope AND key = @key;

-- name: CompleteKey :execrows
UPDATE idempotency_keys
   SET state = 'complete',
       response_status = @response_status,
       response_content_type = @response_content_type,
       response_body = @response_body
 WHERE scope = @scope
   AND key = @key
   AND fingerprint = @fingerprint
   AND state = 'in_progress';

-- name: ReleaseKey :execrows
DELETE FROM idempotency_keys
 WHERE scope = @scope
   AND key = @key
   AND fingerprint = @fingerprint
   AND state = 'in_progress';

-- name: PurgeExpiredKeys :execrows
DELETE FROM idempotency_keys AS d
 WHERE (d.scope, d.key) IN (
     SELECT k.scope, k.key
       FROM idempotency_keys AS k
      WHERE k.expires_at <= @now
      ORDER BY k.expires_at
      LIMIT @batch_size
 );
