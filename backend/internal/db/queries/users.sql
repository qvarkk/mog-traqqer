-- name: CreateUser :one
INSERT INTO users (id, email, password_hash)
VALUES ($1, $2, $3)
RETURNING *;

-- name: GetUser :one
SELECT * FROM users
WHERE id = $1;

-- name: GetUserByEmail :one
SELECT * FROM users
WHERE email = $1;

-- name: DeleteUser :execrows
DELETE FROM users
WHERE id = $1;
