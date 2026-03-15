-- name: CreateUser :one
INSERT INTO users (username, display_name, password_hash, is_admin)
VALUES ($1, $2, $3, $4)
RETURNING id, username, display_name, avatar_url, is_admin, created_at, last_seen_at;

-- name: GetUserByID :one
SELECT id, username, display_name, password_hash, avatar_url, is_admin, created_at, last_seen_at
FROM users WHERE id = $1;

-- name: GetUserByUsername :one
SELECT id, username, display_name, password_hash, avatar_url, is_admin, created_at, last_seen_at
FROM users WHERE username = $1;

-- name: ListUsers :many
SELECT id, username, display_name, avatar_url, is_admin, created_at, last_seen_at
FROM users ORDER BY username;

-- name: UpdateUser :one
UPDATE users
SET display_name = COALESCE(sqlc.narg('display_name'), display_name),
    avatar_url = COALESCE(sqlc.narg('avatar_url'), avatar_url)
WHERE id = $1
RETURNING id, username, display_name, avatar_url, is_admin, created_at, last_seen_at;

-- name: UpdateLastSeen :exec
UPDATE users SET last_seen_at = now() WHERE id = $1;

-- name: CountUsers :one
SELECT count(*) FROM users;
