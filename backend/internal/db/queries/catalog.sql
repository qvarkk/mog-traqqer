-- name: ListMuscleGroups :many
SELECT * FROM muscle_groups
WHERE deleted_at IS NULL
  AND (owner_id IS NULL OR owner_id = $1)
ORDER BY owner_id NULLS FIRST, name;

-- name: CreateMuscleGroup :one
INSERT INTO muscle_groups (owner_id, name)
VALUES ($1, $2)
RETURNING *;

-- name: GetMuscleGroup :one
SELECT * FROM muscle_groups
WHERE id = $1 AND deleted_at IS NULL;

-- name: UpdateMuscleGroup :one
UPDATE muscle_groups
SET name = $2
WHERE id = $1 AND deleted_at IS NULL
RETURNING *;

-- name: SoftDeleteMuscleGroup :execrows
UPDATE muscle_groups
SET deleted_at = now()
WHERE id = $1 AND deleted_at IS NULL;
