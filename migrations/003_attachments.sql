-- +goose Up
CREATE TABLE attachments (
    id             BIGSERIAL PRIMARY KEY,
    message_id     BIGINT REFERENCES messages(id) ON DELETE CASCADE NOT NULL,
    file_name      VARCHAR(255) NOT NULL,
    file_size      BIGINT NOT NULL,
    mime_type      VARCHAR(100) NOT NULL,
    storage_path   VARCHAR(500) NOT NULL,
    thumbnail_path VARCHAR(500),
    created_at     TIMESTAMPTZ DEFAULT now()
);

CREATE INDEX idx_attachments_message ON attachments(message_id);

-- +goose Down
DROP INDEX IF EXISTS idx_attachments_message;
DROP TABLE IF EXISTS attachments;
