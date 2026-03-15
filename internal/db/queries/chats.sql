-- name: CreateChat :one
INSERT INTO chats (type, name, avatar_url, created_by)
VALUES ($1, $2, $3, $4)
RETURNING id, type, name, avatar_url, created_by, created_at;

-- name: GetChatByID :one
SELECT id, type, name, avatar_url, created_by, created_at
FROM chats WHERE id = $1;

-- name: UpdateChat :one
UPDATE chats
SET name = COALESCE(sqlc.narg('name'), name),
    avatar_url = COALESCE(sqlc.narg('avatar_url'), avatar_url)
WHERE id = $1
RETURNING id, type, name, avatar_url, created_by, created_at;

-- name: AddChatMember :exec
INSERT INTO chat_members (chat_id, user_id, role)
VALUES ($1, $2, $3)
ON CONFLICT DO NOTHING;

-- name: RemoveChatMember :exec
DELETE FROM chat_members WHERE chat_id = $1 AND user_id = $2;

-- name: ListChatMembers :many
SELECT u.id, u.username, u.display_name, u.avatar_url, u.last_seen_at, cm.role, cm.joined_at
FROM chat_members cm
JOIN users u ON u.id = cm.user_id
WHERE cm.chat_id = $1
ORDER BY cm.joined_at;

-- name: IsChatMember :one
SELECT EXISTS(
    SELECT 1 FROM chat_members WHERE chat_id = $1 AND user_id = $2
) AS is_member;

-- name: FindDirectChat :one
SELECT cm1.chat_id
FROM chat_members cm1
JOIN chat_members cm2 ON cm1.chat_id = cm2.chat_id
JOIN chats c ON c.id = cm1.chat_id
WHERE cm1.user_id = $1 AND cm2.user_id = $2 AND c.type = 'direct';

-- name: ListChatsByUser :many
SELECT c.id, c.type, c.name, c.avatar_url, c.created_by, c.created_at,
       lm.id AS last_message_id, lm.content AS last_message_content,
       lm.sender_id AS last_message_sender_id, lm.created_at AS last_message_at
FROM chat_members cm
JOIN chats c ON c.id = cm.chat_id
LEFT JOIN messages lm ON lm.id = (
    SELECT id FROM messages WHERE chat_id = c.id ORDER BY created_at DESC LIMIT 1
)
WHERE cm.user_id = $1
ORDER BY COALESCE(lm.created_at, c.created_at) DESC;
