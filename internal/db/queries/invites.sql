-- name: CreateInvite :one
INSERT INTO invites (code, created_by, expires_at)
VALUES ($1, $2, $3)
RETURNING id, code, created_by, used_by, expires_at, used_at, created_at;

-- name: GetInviteByCode :one
SELECT id, code, created_by, used_by, expires_at, used_at, created_at
FROM invites WHERE code = $1;

-- name: MarkInviteUsed :exec
UPDATE invites SET used_by = $1, used_at = now() WHERE id = $2;

-- name: ListInvites :many
SELECT id, code, created_by, used_by, expires_at, used_at, created_at
FROM invites ORDER BY created_at DESC;

-- name: DeleteInvite :exec
DELETE FROM invites WHERE id = $1;
