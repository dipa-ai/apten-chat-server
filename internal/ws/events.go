package ws

import (
	"encoding/json"
	"time"
)

// Event is the envelope for all WebSocket messages.
type Event struct {
	Type    string          `json:"type"`
	Payload json.RawMessage `json:"payload"`
}

// Client → Server payloads

type MessageSendPayload struct {
	ChatID    int64  `json:"chat_id"`
	Content   string `json:"content"`
	ReplyToID *int64 `json:"reply_to_id,omitempty"`
	ClientID  string `json:"client_id"`
}

type TypingPayload struct {
	ChatID int64 `json:"chat_id"`
}

type MessageReadPayload struct {
	ChatID        int64 `json:"chat_id"`
	LastMessageID int64 `json:"last_message_id"`
}

// Server → Client payloads

// AttachmentPayload is the concrete shape of an attachment carried by a
// message.new event. It mirrors the frontend Attachment type.
type AttachmentPayload struct {
	ID            int64     `json:"id"`
	MessageID     int64     `json:"message_id"`
	FileName      string    `json:"file_name"`
	FileSize      int64     `json:"file_size"`
	MimeType      string    `json:"mime_type"`
	StoragePath   string    `json:"storage_path"`
	ThumbnailPath *string   `json:"thumbnail_path"`
	CreatedAt     time.Time `json:"created_at"`
}

type MessageNewPayload struct {
	ID          int64               `json:"id"`
	ChatID      int64               `json:"chat_id"`
	SenderID    int64               `json:"sender_id"`
	SenderName  string              `json:"sender_name"`
	Content     *string             `json:"content"`
	ReplyToID   *int64              `json:"reply_to_id,omitempty"`
	Attachments []AttachmentPayload `json:"attachments"`
	CreatedAt   time.Time           `json:"created_at"`
	ClientID    string              `json:"client_id,omitempty"`
}

type MessageAckPayload struct {
	ClientID  string `json:"client_id"`
	MessageID int64  `json:"message_id"`
}

// MessageErrorPayload is sent back to the sender when a message.send
// event cannot be delivered (rate limit, permissions, persistence error).
type MessageErrorPayload struct {
	ClientID string `json:"client_id"`
	Reason   string `json:"reason"`
}

type TypingUpdatePayload struct {
	ChatID   int64 `json:"chat_id"`
	UserID   int64 `json:"user_id"`
	IsTyping bool  `json:"is_typing"`
}

type PresenceUpdatePayload struct {
	UserID int64  `json:"user_id"`
	Status string `json:"status"` // "online" | "offline"
}

type MessageReadUpdatePayload struct {
	ChatID        int64 `json:"chat_id"`
	UserID        int64 `json:"user_id"`
	LastReadMsgID int64 `json:"last_read_msg_id"`
}

type MessageUpdatedPayload struct {
	ID        int64     `json:"id"`
	ChatID    int64     `json:"chat_id"`
	Content   *string   `json:"content"`
	UpdatedAt time.Time `json:"updated_at"`
}

type MessageDeletedPayload struct {
	ID     int64 `json:"id"`
	ChatID int64 `json:"chat_id"`
}

func NewEvent(typ string, payload any) (Event, error) {
	data, err := json.Marshal(payload)
	if err != nil {
		return Event{}, err
	}
	return Event{Type: typ, Payload: data}, nil
}
