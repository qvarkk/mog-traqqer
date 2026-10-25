-- name: CreateUser :one
INSERT INTO users (email, display_name)
VALUES ($1, $2)
RETURNING *;

-- name: GetUser :one
SELECT * FROM users
WHERE id = $1;

-- name: GetUserByEmail :one
SELECT * FROM users
WHERE email = $1;

-- name: ListUsers :many
SELECT * FROM users
ORDER BY created_at DESC
LIMIT $1 OFFSET $2;

-- name: UpdateUserDisplayName :one
UPDATE users
SET display_name = $2,
    updated_at   = now()
WHERE id = $1
RETURNING *;

-- name: DeleteUser :execrows
DELETE FROM users
WHERE id = $1;
