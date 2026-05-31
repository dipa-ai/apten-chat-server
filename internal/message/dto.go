package message

import (
	"context"
	"time"

	"github.com/apten-chat/messenger/internal/db/dbq"
	"github.com/jackc/pgx/v5/pgtype"
)

// AttachmentDTO is the JSON contract for a message attachment. It mirrors the
// frontend Attachment type so REST, upload, and WebSocket payloads stay
// consistent.
type AttachmentDTO struct {
	ID            int64     `json:"id"`
	MessageID     int64     `json:"message_id"`
	FileName      string    `json:"file_name"`
	FileSize      int64     `json:"file_size"`
	MimeType      string    `json:"mime_type"`
	StoragePath   string    `json:"storage_path"`
	ThumbnailPath *string   `json:"thumbnail_path"`
	CreatedAt     time.Time `json:"created_at"`
}

// MessageDTO is the JSON contract for a message returned by the list/get
// endpoints. Attachments is always a (possibly empty) array, never null.
type MessageDTO struct {
	ID                int64           `json:"id"`
	ChatID            int64           `json:"chat_id"`
	SenderID          int64           `json:"sender_id"`
	SenderUsername    string          `json:"sender_username"`
	SenderDisplayName string          `json:"sender_display_name"`
	Content           *string         `json:"content"`
	ReplyToID         *int64          `json:"reply_to_id"`
	CreatedAt         time.Time       `json:"created_at"`
	UpdatedAt         *time.Time      `json:"updated_at"`
	DeletedAt         *time.Time      `json:"deleted_at"`
	Attachments       []AttachmentDTO `json:"attachments"`
}

func textPtr(t pgtype.Text) *string {
	if !t.Valid {
		return nil
	}
	s := t.String
	return &s
}

func int8Ptr(i pgtype.Int8) *int64 {
	if !i.Valid {
		return nil
	}
	v := i.Int64
	return &v
}

func timestamptzPtr(t pgtype.Timestamptz) *time.Time {
	if !t.Valid {
		return nil
	}
	v := t.Time
	return &v
}

func attachmentDTO(a dbq.Attachment) AttachmentDTO {
	return AttachmentDTO{
		ID:            a.ID,
		MessageID:     a.MessageID,
		FileName:      a.FileName,
		FileSize:      a.FileSize,
		MimeType:      a.MimeType,
		StoragePath:   a.StoragePath,
		ThumbnailPath: textPtr(a.ThumbnailPath),
		CreatedAt:     a.CreatedAt.Time,
	}
}

// buildMessageDTO assembles a MessageDTO from the scalar fields shared by every
// message row type (list-latest, list-before, get-by-id) plus its attachments.
// A nil atts slice is normalized to an empty slice so it serializes as [].
func buildMessageDTO(
	id, chatID, senderID int64,
	senderUsername, senderDisplayName string,
	content pgtype.Text,
	replyToID pgtype.Int8,
	createdAt, updatedAt, deletedAt pgtype.Timestamptz,
	atts []AttachmentDTO,
) MessageDTO {
	if atts == nil {
		atts = []AttachmentDTO{}
	}
	return MessageDTO{
		ID:                id,
		ChatID:            chatID,
		SenderID:          senderID,
		SenderUsername:    senderUsername,
		SenderDisplayName: senderDisplayName,
		Content:           textPtr(content),
		ReplyToID:         int8Ptr(replyToID),
		CreatedAt:         createdAt.Time,
		UpdatedAt:         timestamptzPtr(updatedAt),
		DeletedAt:         timestamptzPtr(deletedAt),
		Attachments:       atts,
	}
}

// attachmentsByMessageID batch-loads attachments for the given message IDs and
// groups them by message_id. Messages with no attachments are simply absent
// from the map (callers default to an empty slice).
func (h *Handler) attachmentsByMessageID(ctx context.Context, messageIDs []int64) (map[int64][]AttachmentDTO, error) {
	if len(messageIDs) == 0 {
		return map[int64][]AttachmentDTO{}, nil
	}
	rows, err := h.queries.ListAttachmentsByMessageIDs(ctx, messageIDs)
	if err != nil {
		return nil, err
	}
	result := make(map[int64][]AttachmentDTO, len(messageIDs))
	for _, row := range rows {
		result[row.MessageID] = append(result[row.MessageID], attachmentDTO(row))
	}
	return result, nil
}
