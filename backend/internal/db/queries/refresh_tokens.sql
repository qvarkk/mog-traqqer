-- name: InsertRefreshToken :exec
INSERT INTO refresh_tokens (jti, user_id, device_info, expires_at)
VALUES ($1, $2, jsonb_build_object('device', $3::text), $4);

-- name: GetRefreshToken :one
SELECT * FROM refresh_tokens
WHERE jti = $1;

-- name: RevokeRefreshToken :execrows
UPDATE refresh_tokens
SET is_revoked = true
WHERE jti = $1 AND is_revoked = false;

-- name: RevokeUserTokens :execrows
UPDATE refresh_tokens
SET is_revoked = true
WHERE user_id = $1 AND is_revoked = false;
