package ws

import (
	"encoding/json"
	"log"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/apten-chat/messenger/internal/auth"
	"github.com/apten-chat/messenger/internal/chat"
	"github.com/apten-chat/messenger/internal/db/dbq"
	"github.com/apten-chat/messenger/internal/message"
	"github.com/coder/websocket"
)

// wsTokenPrefix marks the WebSocket subprotocol that carries the access token.
// Browsers cannot set Authorization headers on WebSocket handshakes, so the
// token rides in Sec-WebSocket-Protocol as "apten-chat.jwt.<token>" instead of
// the URL (where it would leak into logs and history).
const wsTokenPrefix = "apten-chat.jwt."

type typingKey struct {
	UserID int64
	ChatID int64
}

type Handler struct {
	hub            *Hub
	chatService    *chat.Service
	messageService *message.Service
	queries        dbq.Querier
	jwtSecret      string
	allowedOrigins []string

	typingTimers map[typingKey]*time.Timer
	typingMu     sync.Mutex
}

func NewHandler(hub *Hub, chatService *chat.Service, messageService *message.Service, queries dbq.Querier, jwtSecret string, allowedOrigins []string) *Handler {
	return &Handler{
		hub:            hub,
		chatService:    chatService,
		messageService: messageService,
		queries:        queries,
		jwtSecret:      jwtSecret,
		allowedOrigins: allowedOrigins,
		typingTimers:   make(map[typingKey]*time.Timer),
	}
}

func (h *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	// Enforce the origin allowlist (when configured) before upgrading.
	if len(h.allowedOrigins) > 0 && !originAllowed(r.Header.Get("Origin"), h.allowedOrigins) {
		// Log the rejected origin: a 403 here is almost always a misconfigured
		// WS_ALLOWED_ORIGINS that doesn't match the deployed scheme+host.
		log.Printf("ws: rejected disallowed origin %q (allowed: %v)", r.Header.Get("Origin"), h.allowedOrigins)
		http.Error(w, "origin not allowed", http.StatusForbidden)
		return
	}

	protocol := protocolFromRequest(r)
	if protocol == "" {
		http.Error(w, "missing token", http.StatusUnauthorized)
		return
	}
	tokenStr := strings.TrimPrefix(protocol, wsTokenPrefix)

	claims, err := auth.ParseAccessToken(h.jwtSecret, tokenStr)
	if err != nil {
		http.Error(w, "invalid token", http.StatusUnauthorized)
		return
	}

	// Echo the negotiated subprotocol back so the browser handshake succeeds.
	// Origin is enforced above, so the built-in same-origin check is disabled
	// (it would otherwise reject legitimate reverse-proxied deployments).
	conn, err := websocket.Accept(w, r, &websocket.AcceptOptions{
		InsecureSkipVerify: true,
		Subprotocols:       []string{protocol},
	})
	if err != nil {
		log.Printf("ws: accept error: %v", err)
		return
	}

	client := NewClient(h.hub, conn, claims.UserID)
	h.hub.Register <- client

	go client.WritePump()
	go client.ReadPump(h.handleEvent)
}

func (h *Handler) handleEvent(client *Client, evt Event) {
	switch evt.Type {
	case "message.send":
		h.handleMessageSend(client, evt.Payload)
	case "typing.start":
		h.handleTyping(client, evt.Payload, true)
	case "typing.stop":
		h.handleTyping(client, evt.Payload, false)
	case "message.read":
		h.handleMessageRead(client, evt.Payload)
	default:
		log.Printf("ws: unknown event type %q from user %d", evt.Type, client.UserID)
	}
}

func (h *Handler) handleMessageSend(client *Client, payload json.RawMessage) {
	var p MessageSendPayload
	if err := json.Unmarshal(payload, &p); err != nil {
		log.Printf("ws: message send unmarshal error: %v", err)
		return
	}

	if err := h.chatService.EnsureMember(client.ctx, p.ChatID, client.UserID); err != nil {
		client.sendError(p.ClientID, "forbidden")
		return
	}

	msg, err := h.messageService.Send(client.ctx, p.ChatID, client.UserID, p.Content, p.ReplyToID)
	if err != nil {
		log.Printf("ws: message send error: %v", err)
		client.sendError(p.ClientID, "send_failed")
		return
	}

	// Get sender info for the broadcast.
	fullMsg, err := h.messageService.GetByID(client.ctx, msg.ID)
	if err != nil {
		log.Printf("ws: get message error: %v", err)
		client.sendError(p.ClientID, "internal")
		return
	}

	memberIDs, err := h.chatService.GetMemberIDs(client.ctx, p.ChatID)
	if err != nil {
		client.sendError(p.ClientID, "internal")
		return
	}

	var content *string
	if fullMsg.Content.Valid {
		content = &fullMsg.Content.String
	}

	newEvt, _ := NewEvent("message.new", MessageNewPayload{
		ID:          fullMsg.ID,
		ChatID:      fullMsg.ChatID,
		SenderID:    fullMsg.SenderID,
		SenderName:  fullMsg.SenderDisplayName,
		Content:     content,
		Attachments: []AttachmentPayload{},
		CreatedAt:   fullMsg.CreatedAt.Time,
		ClientID:    p.ClientID,
	})
	h.hub.Send(memberIDs, newEvt)

	// Send ack to sender.
	ackEvt, _ := NewEvent("message.ack", MessageAckPayload{
		ClientID:  p.ClientID,
		MessageID: msg.ID,
	})
	h.hub.Send([]int64{client.UserID}, ackEvt)
}

func (h *Handler) handleTyping(client *Client, payload json.RawMessage, isTyping bool) {
	var p TypingPayload
	if err := json.Unmarshal(payload, &p); err != nil {
		return
	}

	key := typingKey{UserID: client.UserID, ChatID: p.ChatID}

	h.typingMu.Lock()
	if timer, ok := h.typingTimers[key]; ok {
		timer.Stop()
		delete(h.typingTimers, key)
	}
	h.typingMu.Unlock()

	memberIDs, err := h.chatService.GetMemberIDs(client.ctx, p.ChatID)
	if err != nil {
		return
	}

	others := make([]int64, 0, len(memberIDs)-1)
	for _, id := range memberIDs {
		if id != client.UserID {
			others = append(others, id)
		}
	}

	h.broadcastTyping(others, p.ChatID, client.UserID, isTyping)

	// Auto-expire typing after 5s.
	if isTyping {
		h.typingMu.Lock()
		h.typingTimers[key] = time.AfterFunc(5*time.Second, func() {
			h.broadcastTyping(others, p.ChatID, client.UserID, false)
			h.typingMu.Lock()
			delete(h.typingTimers, key)
			h.typingMu.Unlock()
		})
		h.typingMu.Unlock()
	}
}

func (h *Handler) handleMessageRead(client *Client, payload json.RawMessage) {
	var p MessageReadPayload
	if err := json.Unmarshal(payload, &p); err != nil {
		return
	}

	if err := h.chatService.EnsureMember(client.ctx, p.ChatID, client.UserID); err != nil {
		return
	}

	if err := h.queries.UpsertMessageRead(client.ctx, dbq.UpsertMessageReadParams{
		ChatID:        p.ChatID,
		UserID:        client.UserID,
		LastReadMsgID: p.LastMessageID,
	}); err != nil {
		log.Printf("ws: upsert message read error: %v", err)
		return
	}

	memberIDs, err := h.chatService.GetMemberIDs(client.ctx, p.ChatID)
	if err != nil {
		return
	}

	evt, _ := NewEvent("message.read_update", MessageReadUpdatePayload{
		ChatID:        p.ChatID,
		UserID:        client.UserID,
		LastReadMsgID: p.LastMessageID,
	})
	h.hub.Send(memberIDs, evt)
}

func (h *Handler) broadcastTyping(userIDs []int64, chatID, userID int64, isTyping bool) {
	evt, _ := NewEvent("typing.update", TypingUpdatePayload{
		ChatID:   chatID,
		UserID:   userID,
		IsTyping: isTyping,
	})
	h.hub.Send(userIDs, evt)
}

// protocolFromRequest returns the "apten-chat.jwt.<token>" subprotocol offered
// by the client, or "" if none was offered. The Sec-WebSocket-Protocol header
// may appear multiple times and/or as a single comma-separated value.
func protocolFromRequest(r *http.Request) string {
	for _, header := range r.Header.Values("Sec-WebSocket-Protocol") {
		for _, p := range strings.Split(header, ",") {
			p = strings.TrimSpace(p)
			if strings.HasPrefix(p, wsTokenPrefix) {
				return p
			}
		}
	}
	return ""
}

// originAllowed reports whether origin exactly matches one of the configured
// allowed origins.
func originAllowed(origin string, allowed []string) bool {
	if origin == "" {
		return false
	}
	for _, item := range allowed {
		if origin == strings.TrimSpace(item) {
			return true
		}
	}
	return false
}
