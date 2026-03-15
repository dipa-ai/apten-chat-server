package ws

import (
	"encoding/json"
	"log"
	"net/http"
	"sync"
	"time"

	"github.com/apten-chat/messenger/internal/auth"
	"github.com/apten-chat/messenger/internal/chat"
	"github.com/apten-chat/messenger/internal/db/dbq"
	"github.com/apten-chat/messenger/internal/message"
	"github.com/coder/websocket"
)

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

	typingTimers map[typingKey]*time.Timer
	typingMu     sync.Mutex
}

func NewHandler(hub *Hub, chatService *chat.Service, messageService *message.Service, queries dbq.Querier, jwtSecret string) *Handler {
	return &Handler{
		hub:            hub,
		chatService:    chatService,
		messageService: messageService,
		queries:        queries,
		jwtSecret:      jwtSecret,
		typingTimers:   make(map[typingKey]*time.Timer),
	}
}

func (h *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	tokenStr := r.URL.Query().Get("token")
	if tokenStr == "" {
		http.Error(w, "missing token", http.StatusUnauthorized)
		return
	}

	claims, err := auth.ParseAccessToken(h.jwtSecret, tokenStr)
	if err != nil {
		http.Error(w, "invalid token", http.StatusUnauthorized)
		return
	}

	conn, err := websocket.Accept(w, r, &websocket.AcceptOptions{
		InsecureSkipVerify: true, // Allow any origin in dev.
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
		return
	}

	if err := h.chatService.EnsureMember(client.ctx, p.ChatID, client.UserID); err != nil {
		return
	}

	msg, err := h.messageService.Send(client.ctx, p.ChatID, client.UserID, p.Content, p.ReplyToID)
	if err != nil {
		log.Printf("ws: message send error: %v", err)
		return
	}

	// Get sender info for the broadcast.
	fullMsg, err := h.messageService.GetByID(client.ctx, msg.ID)
	if err != nil {
		log.Printf("ws: get message error: %v", err)
		return
	}

	memberIDs, err := h.chatService.GetMemberIDs(client.ctx, p.ChatID)
	if err != nil {
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
		Attachments: []any{},
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
