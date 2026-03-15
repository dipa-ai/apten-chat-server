-- name: CreatePushSubscription :one
INSERT INTO push_subscriptions (user_id, endpoint, p256dh_key, auth_key, user_agent)
VALUES ($1, $2, $3, $4, $5)
ON CONFLICT (endpoint) DO UPDATE
SET p256dh_key = EXCLUDED.p256dh_key,
    auth_key = EXCLUDED.auth_key,
    user_agent = EXCLUDED.user_agent
RETURNING id, user_id, endpoint, p256dh_key, auth_key, user_agent, created_at;

-- name: DeletePushSubscription :exec
DELETE FROM push_subscriptions WHERE user_id = $1 AND endpoint = $2;

-- name: DeletePushSubscriptionByEndpoint :exec
DELETE FROM push_subscriptions WHERE endpoint = $1;

-- name: ListPushSubscriptionsByUser :many
SELECT id, user_id, endpoint, p256dh_key, auth_key, user_agent, created_at
FROM push_subscriptions WHERE user_id = $1;
