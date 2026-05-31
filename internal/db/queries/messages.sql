-- name: CreateMessage :one
INSERT INTO messages (chat_id, sender_id, content, reply_to_id)
VALUES ($1, $2, $3, $4)
RETURNING id, chat_id, sender_id, content, reply_to_id, created_at, updated_at, deleted_at;

-- name: GetMessageByID :one
SELECT m.id, m.chat_id, m.sender_id, m.content, m.reply_to_id, m.created_at, m.updated_at, m.deleted_at,
       u.username AS sender_username, u.display_name AS sender_display_name
FROM messages m
JOIN users u ON u.id = m.sender_id
WHERE m.id = $1;

-- name: ListMessagesByChatBefore :many
SELECT m.id, m.chat_id, m.sender_id, m.content, m.reply_to_id, m.created_at, m.updated_at, m.deleted_at,
       u.username AS sender_username, u.display_name AS sender_display_name
FROM messages m
JOIN users u ON u.id = m.sender_id
WHERE m.chat_id = $1
  AND (m.created_at, m.id) < (
      SELECT msg.created_at, msg.id FROM messages msg WHERE msg.id = $2
  )
ORDER BY m.created_at DESC, m.id DESC
LIMIT $3;

-- name: ListMessagesByChatLatest :many
SELECT m.id, m.chat_id, m.sender_id, m.content, m.reply_to_id, m.created_at, m.updated_at, m.deleted_at,
       u.username AS sender_username, u.display_name AS sender_display_name
FROM messages m
JOIN users u ON u.id = m.sender_id
WHERE m.chat_id = $1
ORDER BY m.created_at DESC, m.id DESC
LIMIT $2;

-- name: UpdateMessageContent :one
UPDATE messages
SET content = $2, updated_at = now()
WHERE id = $1
RETURNING id, chat_id, sender_id, content, reply_to_id, created_at, updated_at, deleted_at;

-- name: SoftDeleteMessage :exec
UPDATE messages
SET content = NULL,
    deleted_at = now(),
    updated_at = now()
WHERE id = $1;
