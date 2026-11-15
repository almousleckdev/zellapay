-- name: InsertSession :exec
INSERT INTO sessions (id, user_id, token_hash, created_at, last_seen_at,
                      expires_at, ip, user_agent)
VALUES (@id, @user_id, @token_hash, @created_at, @created_at,
        @expires_at, @ip, @user_agent);

-- name: GetSessionByHash :one
SELECT * FROM sessions WHERE token_hash = @token_hash;

-- name: TouchSession :exec
UPDATE sessions SET last_seen_at = @at WHERE id = @id AND revoked_at IS NULL;

-- name: SetSessionMFA :execrows
UPDATE sessions SET mfa_at = @at WHERE id = @id AND revoked_at IS NULL;

-- name: RevokeSession :execrows
UPDATE sessions SET revoked_at = @at
WHERE id = @id AND user_id = @user_id AND revoked_at IS NULL;

-- name: RevokeUserSessions :execrows
UPDATE sessions SET revoked_at = @at
WHERE user_id = @user_id AND revoked_at IS NULL
  AND (@keep_id::text = '' OR id <> @keep_id::text);

-- name: ListOpenSessions :many
SELECT * FROM sessions
WHERE user_id = @user_id AND revoked_at IS NULL AND expires_at > @at
ORDER BY last_seen_at DESC;

-- name: UpsertMFACredential :execrows
INSERT INTO mfa_credentials (user_id, secret_enc, created_at)
VALUES (@user_id, @secret_enc, @created_at)
ON CONFLICT (user_id) DO UPDATE
SET secret_enc = EXCLUDED.secret_enc, created_at = EXCLUDED.created_at
WHERE mfa_credentials.confirmed_at IS NULL;

-- name: GetMFACredential :one
SELECT * FROM mfa_credentials WHERE user_id = @user_id;

-- name: ConfirmMFACredential :execrows
UPDATE mfa_credentials SET confirmed_at = @at, last_step = @step
WHERE user_id = @user_id AND confirmed_at IS NULL;

-- name: AdvanceMFAStep :execrows
UPDATE mfa_credentials SET last_step = @step
WHERE user_id = @user_id AND confirmed_at IS NOT NULL AND last_step < @step;

-- name: DeleteRecoveryCodes :exec
DELETE FROM recovery_codes WHERE user_id = @user_id;

-- name: InsertRecoveryCode :exec
INSERT INTO recovery_codes (id, user_id, code_hash, created_at)
VALUES (@id, @user_id, @code_hash, @created_at);

-- name: UseRecoveryCode :execrows
UPDATE recovery_codes SET used_at = @at
WHERE user_id = @user_id AND code_hash = @code_hash AND used_at IS NULL;

-- name: CountRecoveryCodes :one
SELECT count(*) FROM recovery_codes WHERE user_id = @user_id AND used_at IS NULL;
