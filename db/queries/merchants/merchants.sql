-- name: CreateMerchant :one
INSERT INTO merchants (id, name, default_currency, created_at, updated_at)
VALUES (@id, @name, @default_currency, @now, @now)
RETURNING id, name, status, kyc_status, default_currency, created_at;

-- name: GetMerchant :one
SELECT id, name, status, kyc_status, default_currency, created_at
  FROM merchants
 WHERE id = @id;

-- name: SetMerchantStatus :execrows
UPDATE merchants
   SET status = @status, updated_at = @now
 WHERE id = @id;

-- name: InsertAPIKey :exec
INSERT INTO merchant_api_keys
    (id, merchant_id, lookup_id, secret_hash, mode, scopes, ip_allowlist,
     created_at, expires_at)
VALUES
    (@id, @merchant_id, @lookup_id, @secret_hash, @mode, @scopes,
     @ip_allowlist, @created_at, @expires_at);

-- name: GetAPIKeyByLookup :one
SELECT k.id, k.merchant_id, k.lookup_id, k.secret_hash, k.mode, k.scopes,
       k.ip_allowlist, k.created_at, k.expires_at, k.revoked_at,
       m.status AS merchant_status
  FROM merchant_api_keys k
  JOIN merchants m ON m.id = k.merchant_id
 WHERE k.lookup_id = @lookup_id;

-- name: RevokeAPIKey :execrows
UPDATE merchant_api_keys
   SET revoked_at = @now
 WHERE id = @id AND merchant_id = @merchant_id AND revoked_at IS NULL;

-- name: SetAPIKeyExpiry :execrows
UPDATE merchant_api_keys
   SET expires_at = @expires_at
 WHERE id = @id AND merchant_id = @merchant_id AND revoked_at IS NULL;

-- name: ListAPIKeys :many
SELECT id, merchant_id, lookup_id, mode, scopes, ip_allowlist,
       created_at, expires_at, revoked_at
  FROM merchant_api_keys
 WHERE merchant_id = @merchant_id
 ORDER BY created_at, id;
