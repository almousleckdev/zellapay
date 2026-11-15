-- name: LockAuditHead :one
SELECT seq, hash FROM audit_head WHERE id = 1 FOR UPDATE;

-- name: GetAuditHead :one
SELECT seq, hash FROM audit_head WHERE id = 1;

-- name: InsertAuditEntry :exec
INSERT INTO audit_log
    (seq, at, actor_kind, actor_id, action, target_kind, target_id, reason,
     change, ip, user_agent, request_id, prev_hash, hash)
VALUES
    (@seq, @at, @actor_kind, @actor_id, @action, @target_kind, @target_id,
     @reason, @change, @ip, @user_agent, @request_id, @prev_hash, @hash);

-- name: AdvanceAuditHead :exec
UPDATE audit_head SET seq = @seq, hash = @hash WHERE id = 1;

-- name: ListAuditAfter :many
SELECT seq, at, actor_kind, actor_id, action, target_kind, target_id, reason,
       change, ip, user_agent, request_id, prev_hash, hash
  FROM audit_log
 WHERE seq > @after_seq
 ORDER BY seq
 LIMIT @batch_size;

-- name: ListAuditNewestFirst :many
SELECT seq, at, actor_kind, actor_id, action, target_kind, target_id, reason,
       change, ip, user_agent, request_id, prev_hash, hash
  FROM audit_log
 WHERE (@before_seq::bigint = 0 OR seq < @before_seq::bigint)
   AND (sqlc.narg('actor_kind')::text IS NULL OR actor_kind = sqlc.narg('actor_kind')::text)
   AND (sqlc.narg('actor_id')::text IS NULL OR actor_id = sqlc.narg('actor_id')::text)
   AND (sqlc.narg('target_kind')::text IS NULL OR target_kind = sqlc.narg('target_kind')::text)
   AND (sqlc.narg('target_id')::text IS NULL OR target_id = sqlc.narg('target_id')::text)
   AND (sqlc.narg('action')::text IS NULL OR action = sqlc.narg('action')::text)
 ORDER BY seq DESC
 LIMIT @batch_size;

-- name: GetAuditVerification :one
SELECT verified_seq, verified_hash, broken_seq, broken_reason
  FROM audit_verification WHERE id = 1;

-- name: SaveAuditVerification :exec
UPDATE audit_verification
   SET verified_seq = @verified_seq, verified_hash = @verified_hash,
       verified_at = @verified_at, broken_seq = @broken_seq,
       broken_reason = @broken_reason
 WHERE id = 1;
