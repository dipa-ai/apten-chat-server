-- +goose Up
CREATE TABLE push_subscriptions (
    id          BIGSERIAL PRIMARY KEY,
    user_id     BIGINT REFERENCES users(id) ON DELETE CASCADE NOT NULL,
    endpoint    TEXT UNIQUE NOT NULL,
    p256dh_key  TEXT NOT NULL,
    auth_key    TEXT NOT NULL,
    user_agent  VARCHAR(255),
    created_at  TIMESTAMPTZ DEFAULT now()
);

CREATE INDEX idx_push_subscriptions_user ON push_subscriptions(user_id);

-- +goose Down
DROP INDEX IF EXISTS idx_push_subscriptions_user;
DROP TABLE IF EXISTS push_subscriptions;
