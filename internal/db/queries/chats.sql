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
-- The last message is resolved via a regular LEFT JOIN (not LATERAL): sqlc
-- correctly infers nullable columns for a regular LEFT JOIN, but treats
-- LATERAL-joined NOT NULL columns as non-null, which would fail to scan NULL
-- for chats that have no messages. The direct counterpart and unread count use
-- scalar subqueries, which sqlc always types as nullable.
SELECT
    c.id,
    c.type,
    c.name,
    c.avatar_url,
    c.created_by,
    c.created_at,
    COALESCE(lm.created_at, c.created_at)::timestamptz AS updated_at,
    lm.id AS last_message_id,
    lm.content AS last_message_content,
    lm.sender_id AS last_message_sender_id,
    lmu.display_name AS last_message_sender_display_name,
    lm.deleted_at AS last_message_deleted_at,
    COALESCE((
        SELECT u.display_name
        FROM chat_members cm2
        JOIN users u ON u.id = cm2.user_id
        WHERE cm2.chat_id = c.id AND cm2.user_id <> $1 AND c.type = 'direct'
        ORDER BY cm2.joined_at
        LIMIT 1
    ), '')::text AS direct_display_name,
    (
        SELECT u.avatar_url
        FROM chat_members cm2
        JOIN users u ON u.id = cm2.user_id
        WHERE cm2.chat_id = c.id AND cm2.user_id <> $1 AND c.type = 'direct'
        ORDER BY cm2.joined_at
        LIMIT 1
    ) AS direct_avatar_url,
    COALESCE((
        SELECT count(*)
        FROM messages um
        WHERE um.chat_id = c.id
          AND um.sender_id <> $1
          AND um.deleted_at IS NULL
          AND um.id > COALESCE((
              SELECT mr.last_read_msg_id
              FROM message_reads mr
              WHERE mr.chat_id = c.id AND mr.user_id = $1
          ), 0)
    ), 0)::bigint AS unread_count
FROM chat_members cm
JOIN chats c ON c.id = cm.chat_id
LEFT JOIN messages lm ON lm.id = (
    SELECT m.id FROM messages m WHERE m.chat_id = c.id ORDER BY m.created_at DESC, m.id DESC LIMIT 1
)
LEFT JOIN users lmu ON lmu.id = lm.sender_id
WHERE cm.user_id = $1
ORDER BY COALESCE(lm.created_at, c.created_at) DESC, c.id DESC;
