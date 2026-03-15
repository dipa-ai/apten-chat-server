-- +goose Up
CREATE TABLE chats (
    id          BIGSERIAL PRIMARY KEY,
    type        VARCHAR(20) NOT NULL CHECK (type IN ('direct', 'group')),
    name        VARCHAR(100),
    avatar_url  VARCHAR(500),
    created_by  BIGINT REFERENCES users(id),
    created_at  TIMESTAMPTZ DEFAULT now()
);

CREATE TABLE chat_members (
    chat_id     BIGINT REFERENCES chats(id) ON DELETE CASCADE,
    user_id     BIGINT REFERENCES users(id) ON DELETE CASCADE,
    role        VARCHAR(20) DEFAULT 'member' CHECK (role IN ('admin', 'member')),
    joined_at   TIMESTAMPTZ DEFAULT now(),
    PRIMARY KEY (chat_id, user_id)
);

CREATE TABLE messages (
    id          BIGSERIAL PRIMARY KEY,
    chat_id     BIGINT REFERENCES chats(id) ON DELETE CASCADE NOT NULL,
    sender_id   BIGINT REFERENCES users(id) NOT NULL,
    content     TEXT,
    reply_to_id BIGINT REFERENCES messages(id),
    created_at  TIMESTAMPTZ DEFAULT now(),
    updated_at  TIMESTAMPTZ
);

CREATE INDEX idx_messages_chat_created ON messages(chat_id, created_at DESC);

-- +goose Down
DROP INDEX IF EXISTS idx_messages_chat_created;
DROP TABLE IF EXISTS messages;
DROP TABLE IF EXISTS chat_members;
DROP TABLE IF EXISTS chats;
