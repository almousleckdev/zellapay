-- name: InsertUser :exec
INSERT INTO merchant_users (id, merchant_id, email, name, role, status, locale,
                            password_hash, password_changed_at,
                            email_verified_at, created_at)
VALUES (@id, @merchant_id, @email, @name, @role, @status, @locale,
        @password_hash, @password_changed_at, @email_verified_at, @created_at);

-- name: GetUserByEmail :one
SELECT * FROM merchant_users WHERE email = @email;

-- name: GetUser :one
SELECT * FROM merchant_users WHERE id = @id;

-- name: SetPassword :exec
UPDATE merchant_users
SET password_hash = @password_hash, password_changed_at = @changed_at
WHERE id = @id;

-- name: Rehash :exec
UPDATE merchant_users SET password_hash = @password_hash
WHERE id = @id AND password_hash = @old_hash;

-- name: MarkEmailVerified :exec
UPDATE merchant_users SET email_verified_at = @at
WHERE id = @id AND email_verified_at IS NULL;

-- name: RevokeOpenInvitations :exec
UPDATE invitations SET revoked_at = @at
WHERE merchant_id = @merchant_id AND email = @email
  AND used_at IS NULL AND revoked_at IS NULL;

-- name: InsertInvitation :exec
INSERT INTO invitations (id, merchant_id, email, role, locale, token_hash,
                         invited_by, expires_at, created_at)
VALUES (@id, @merchant_id, @email, @role, @locale, @token_hash,
        @invited_by, @expires_at, @created_at);

-- name: GetInvitationByHash :one
SELECT * FROM invitations
WHERE token_hash = @token_hash AND used_at IS NULL AND revoked_at IS NULL;

-- name: UseInvitation :one
UPDATE invitations SET used_at = @at
WHERE token_hash = @token_hash AND used_at IS NULL
  AND revoked_at IS NULL AND expires_at > @at
RETURNING id;

-- name: ReplaceEmailToken :exec
DELETE FROM email_tokens
WHERE user_id = @user_id AND purpose = @purpose AND used_at IS NULL;

-- name: InsertEmailToken :exec
INSERT INTO email_tokens (id, user_id, purpose, token_hash, expires_at, created_at)
VALUES (@id, @user_id, @purpose, @token_hash, @expires_at, @created_at);

-- name: GetEmailTokenByHash :one
SELECT * FROM email_tokens WHERE token_hash = @token_hash AND purpose = @purpose;

-- name: UseEmailToken :one
UPDATE email_tokens SET used_at = @at
WHERE token_hash = @token_hash AND purpose = @purpose
  AND used_at IS NULL AND expires_at > @at
RETURNING user_id;
