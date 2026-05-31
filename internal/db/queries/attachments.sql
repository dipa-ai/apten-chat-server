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

-- name: GetAttachmentAccessContext :one
SELECT
  a.id,
  a.message_id,
  a.file_name,
  a.file_size,
  a.mime_type,
  a.storage_path,
  a.thumbnail_path,
  a.created_at,
  m.chat_id
FROM attachments a
JOIN messages m ON m.id = a.message_id
WHERE a.id = $1;

-- name: ListAttachmentsByMessageIDs :many
SELECT
  id,
  message_id,
  file_name,
  file_size,
  mime_type,
  storage_path,
  thumbnail_path,
  created_at
FROM attachments
WHERE message_id = ANY($1::bigint[])
ORDER BY message_id, id;
