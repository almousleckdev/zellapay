-- name: LockMerchantForEndpoints :one
SELECT id FROM merchants WHERE id = @merchant_id FOR UPDATE;

-- name: CountActiveEndpoints :one
SELECT count(*)::int FROM webhook_endpoints
 WHERE merchant_id = @merchant_id AND status = 'active';

-- name: InsertEndpoint :exec
INSERT INTO webhook_endpoints
    (id, merchant_id, url, secret_enc, events, description, created_at)
VALUES
    (@id, @merchant_id, @url, @secret_enc, @events, @description, @created_at);

-- name: ListEndpoints :many
SELECT id, merchant_id, url, events, description, status, created_at
  FROM webhook_endpoints
 WHERE merchant_id = @merchant_id
 ORDER BY created_at, id;

-- name: EndpointsForEvent :many
SELECT id, merchant_id, url, events, description, status, created_at
  FROM webhook_endpoints
 WHERE merchant_id = @merchant_id
   AND status = 'active'
   AND @event_type::text = ANY(events)
 ORDER BY id;
