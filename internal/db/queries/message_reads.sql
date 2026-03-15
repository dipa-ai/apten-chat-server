-- name: UpsertMessageRead :exec
INSERT INTO message_reads (chat_id, user_id, last_read_msg_id, read_at)
VALUES ($1, $2, $3, now())
ON CONFLICT (chat_id, user_id) DO UPDATE
SET last_read_msg_id = GREATEST(message_reads.last_read_msg_id, EXCLUDED.last_read_msg_id),
    read_at = now();

-- name: GetMessageRead :one
SELECT chat_id, user_id, last_read_msg_id, read_at
FROM message_reads
WHERE chat_id = $1 AND user_id = $2;

-- name: ListMessageReadsByChat :many
SELECT chat_id, user_id, last_read_msg_id, read_at
FROM message_reads
WHERE chat_id = $1;
