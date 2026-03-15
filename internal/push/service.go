package push

import (
	"context"
	"encoding/json"
	"log"
	"net/http"

	"github.com/SherClockHolmes/webpush-go"
	"github.com/apten-chat/messenger/internal/db/dbq"
)

// HubChecker checks if a user has an active WebSocket connection.
type HubChecker interface {
	IsOnline(userID int64) bool
}

type Service struct {
	queries    *dbq.Queries
	hubChecker HubChecker
	vapidPub   string
	vapidPriv  string
	contact    string
}

func NewService(queries *dbq.Queries, hubChecker HubChecker, vapidPub, vapidPriv, contact string) *Service {
	return &Service{
		queries:    queries,
		hubChecker: hubChecker,
		vapidPub:   vapidPub,
		vapidPriv:  vapidPriv,
		contact:    contact,
	}
}

type NotificationPayload struct {
	Title     string `json:"title"`
	Body      string `json:"body"`
	ChatID    int64  `json:"chat_id"`
	MessageID int64  `json:"message_id"`
}

// SendNotification sends push to a user only if they have no active WS connection.
func (s *Service) SendNotification(ctx context.Context, userID int64, payload NotificationPayload) {
	if s.vapidPub == "" || s.vapidPriv == "" {
		return // Push not configured.
	}
	if s.hubChecker.IsOnline(userID) {
		return // User is online, no need for push.
	}

	subs, err := s.queries.ListPushSubscriptionsByUser(ctx, userID)
	if err != nil || len(subs) == 0 {
		return
	}

	data, err := json.Marshal(payload)
	if err != nil {
		return
	}

	for _, sub := range subs {
		subscription := &webpush.Subscription{
			Endpoint: sub.Endpoint,
			Keys: webpush.Keys{
				P256dh: sub.P256dhKey,
				Auth:   sub.AuthKey,
			},
		}

		resp, err := webpush.SendNotification(data, subscription, &webpush.Options{
			VAPIDPublicKey:  s.vapidPub,
			VAPIDPrivateKey: s.vapidPriv,
			Subscriber:      s.contact,
		})
		if err != nil {
			log.Printf("push: send error for user %d: %v", userID, err)
			continue
		}
		resp.Body.Close()

		// 410 Gone: browser revoked permission.
		if resp.StatusCode == http.StatusGone {
			s.queries.DeletePushSubscriptionByEndpoint(ctx, sub.Endpoint)
		}
	}
}

func (s *Service) VAPIDPublicKey() string {
	return s.vapidPub
}
