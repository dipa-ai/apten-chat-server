-- name: CreateAttachment :one
INSERT INTO attachments (message_id, file_name, file_size, mime_type, storage_path, thumbnail_path)
VALUES ($1, $2, $3, $4, $5, $6)
RETURNING id, message_id, file_name, file_size, mime_type, storage_path, thumbnail_path, created_at;

-- name: GetAttachmentByID :one
SELECT id, message_id, file_name, file_size, mime_type, storage_path, thumbnail_path, created_at
FROM attachments WHERE id = $1;

-- name: ListAttachmentsByMessage :many
SELECT id, message_id, file_name, file_size, mime_type, storage_path, thumbnail_path, created_at
FROM attachments WHERE message_id = $1;

-- name: DeleteAttachmentsByMessage :exec
DELETE FROM attachments WHERE message_id = $1;
