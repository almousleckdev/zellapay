-- name: ExpireStaleChangeRequests :exec
UPDATE merchant_change_requests
   SET status = 'expired'
 WHERE merchant_id = @merchant_id AND kind = @kind
   AND status = 'pending' AND expires_at <= @now;

-- name: InsertChangeRequest :one
INSERT INTO merchant_change_requests
    (id, merchant_id, kind, fee_bps, payout_fee_bps, reason,
     requested_by, requested_at, expires_at)
VALUES
    (@id, @merchant_id, @kind, @fee_bps, @payout_fee_bps, @reason,
     @requested_by, @requested_at, @expires_at)
RETURNING id, merchant_id, kind, fee_bps, payout_fee_bps, reason, requested_by,
          requested_at, expires_at, status, decided_by, decided_at, decision_reason;

-- name: GetChangeRequestForUpdate :one
SELECT id, merchant_id, kind, fee_bps, payout_fee_bps, reason, requested_by,
       requested_at, expires_at, status, decided_by, decided_at, decision_reason
  FROM merchant_change_requests
 WHERE id = @id
   FOR UPDATE;

-- name: DecideChangeRequest :exec
UPDATE merchant_change_requests
   SET status = @status, decided_by = @decided_by, decided_at = @decided_at,
       decision_reason = @decision_reason
 WHERE id = @id;

-- name: ListChangeRequests :many
SELECT r.id, r.merchant_id, m.name AS merchant_name, r.kind, r.fee_bps,
       r.payout_fee_bps, r.reason, r.requested_by, r.requested_at, r.expires_at,
       r.status, r.decided_by, r.decided_at, r.decision_reason
  FROM merchant_change_requests r
  JOIN merchants m ON m.id = r.merchant_id
 WHERE (@status::text = '' OR r.status = @status)
   AND (@merchant_id::text = '' OR r.merchant_id = @merchant_id)
 ORDER BY r.requested_at DESC, r.id DESC
 LIMIT @row_limit;

-- name: CountPendingChangeRequests :one
SELECT count(*)::int
  FROM merchant_change_requests
 WHERE status = 'pending' AND expires_at > @now;
