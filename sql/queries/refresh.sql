-- name: CreateRefreshToken :one
INSERT INTO refresh_tokens (token, created_at, updated_at, user_id, expires_at)
VALUES (
    $1,
    NOW(),
    NOW(),
    $2,
    $3
)
RETURNING *;


-- name: GetUserFromRefreshToken :one
SELECT * FROM refresh_tokens
WHERE token = $1;

-- name: UpdateRevokedAt :one
UPDATE refresh_tokens 
SET updated_at = NOW(), revoked_at = NOW() 
WHERE token = $1
RETURNING *;

-- name: UpdateUserLogin :one
UPDATE users
SET email = $2, hashed_password = $3
WHERE id = $1
RETURNING *;