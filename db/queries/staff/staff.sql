-- name: InsertStaffUser :exec
INSERT INTO staff_users (id, email, name, role, status, password_hash,
                         password_changed_at, created_by, created_at)
VALUES (@id, @email, @name, @role, @status, @password_hash,
        @password_changed_at, @created_by, @created_at);

-- name: GetStaffByEmail :one
SELECT * FROM staff_users WHERE email = @email;

-- name: GetStaff :one
SELECT * FROM staff_users WHERE id = @id;

-- name: Rehash :exec
UPDATE staff_users SET password_hash = @password_hash
WHERE id = @id AND password_hash = @old_hash;

-- name: InsertCredential :exec
INSERT INTO staff_credentials (id, user_id, credential_id, label, data, created_at)
VALUES (@id, @user_id, @credential_id, @label, @data, @created_at);

-- name: ListCredentials :many
SELECT * FROM staff_credentials WHERE user_id = @user_id ORDER BY created_at, id;

-- name: UpdateCredential :exec
UPDATE staff_credentials SET data = @data, last_used_at = @at
WHERE credential_id = @credential_id AND user_id = @user_id;

-- name: InsertStaffSession :exec
INSERT INTO staff_sessions (id, user_id, token_hash, created_at, last_seen_at,
                            expires_at, ip, user_agent)
VALUES (@id, @user_id, @token_hash, @created_at, @created_at,
        @expires_at, @ip, @user_agent);

-- name: GetStaffSessionByHash :one
SELECT * FROM staff_sessions WHERE token_hash = @token_hash;

-- name: TouchStaffSession :exec
UPDATE staff_sessions SET last_seen_at = @at WHERE id = @id AND revoked_at IS NULL;

-- name: SetStaffSessionMFA :execrows
UPDATE staff_sessions SET mfa_at = @at WHERE id = @id AND revoked_at IS NULL;

-- name: RevokeStaffSession :execrows
UPDATE staff_sessions SET revoked_at = @at
WHERE id = @id AND user_id = @user_id AND revoked_at IS NULL;

-- name: SaveChallenge :exec
INSERT INTO staff_challenges (session_id, kind, data, expires_at)
VALUES (@session_id, @kind, @data, @expires_at)
ON CONFLICT (session_id, kind) DO UPDATE
SET data = EXCLUDED.data, expires_at = EXCLUDED.expires_at;

-- name: TakeChallenge :one
DELETE FROM staff_challenges
WHERE session_id = @session_id AND kind = @kind AND expires_at > @at
RETURNING data;
