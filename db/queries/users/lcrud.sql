-- name: GetUser :one
SELECT * FROM users
WHERE id = $1 LIMIT 1;

-- name: ListUsers :many
SELECT * FROM users
ORDER BY name;

-- name: CreateUser :one
INSERT INTO users (
  id, name, email
) VALUES (
  $1, $2, $3
)
RETURNING *;

-- name: UpdateUser :one
UPDATE users SET
  name = $2,
  email = $3
WHERE id = $1
RETURNING *;

-- name: DeleteUser :execrows
DELETE FROM users
WHERE id = $1;
