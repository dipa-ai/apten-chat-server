-- +goose Up
CREATE TABLE message_reads (
    chat_id         BIGINT NOT NULL,
    user_id         BIGINT NOT NULL,
    last_read_msg_id BIGINT REFERENCES messages(id) NOT NULL,
    read_at         TIMESTAMPTZ DEFAULT now(),
    PRIMARY KEY (chat_id, user_id),
    FOREIGN KEY (chat_id, user_id) REFERENCES chat_members(chat_id, user_id)
);

-- +goose Down
DROP TABLE IF EXISTS message_reads;
