package chat

import (
	"time"

	"github.com/apten-chat/messenger/internal/db/dbq"
	"github.com/jackc/pgx/v5/pgtype"
)

// ChatListItemDTO is the JSON contract for an entry in the chat list. It carries
// everything the frontend needs to render a row without per-chat detail fetches:
// a resolved display name/avatar (group name or direct counterpart), the last
// message preview, the effective sort timestamp, and the unread count.
type ChatListItemDTO struct {
	ID                           int64      `json:"id"`
	Type                         string     `json:"type"`
	Name                         *string    `json:"name"`
	AvatarURL                    *string    `json:"avatar_url"`
	DisplayName                  string     `json:"display_name"`
	DisplayAvatarURL             *string    `json:"display_avatar_url"`
	CreatedBy                    *int64     `json:"created_by"`
	CreatedAt                    time.Time  `json:"created_at"`
	UpdatedAt                    time.Time  `json:"updated_at"`
	LastMessageID                *int64     `json:"last_message_id"`
	LastMessageContent           *string    `json:"last_message_content"`
	LastMessageSenderID          *int64     `json:"last_message_sender_id"`
	LastMessageSenderDisplayName *string    `json:"last_message_sender_display_name"`
	LastMessageDeletedAt         *time.Time `json:"last_message_deleted_at"`
	UnreadCount                  int64      `json:"unread_count"`
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

// chatListItemDTO maps a generated row to the JSON contract, resolving the
// display name/avatar from the chat type (group name vs. direct counterpart).
func chatListItemDTO(row dbq.ListChatsByUserRow) ChatListItemDTO {
	displayName := row.DirectDisplayName
	var displayAvatar *string
	if row.Type == "group" {
		if row.Name.Valid && row.Name.String != "" {
			displayName = row.Name.String
		} else {
			displayName = "Group Chat"
		}
		displayAvatar = textPtr(row.AvatarUrl)
	} else {
		// Direct chat: prefer the counterpart's name/avatar, falling back to
		// any name/avatar stored on the chat itself.
		if displayName == "" && row.Name.Valid {
			displayName = row.Name.String
		}
		if row.DirectAvatarUrl.Valid {
			displayAvatar = textPtr(row.DirectAvatarUrl)
		} else {
			displayAvatar = textPtr(row.AvatarUrl)
		}
	}

	return ChatListItemDTO{
		ID:                           row.ID,
		Type:                         row.Type,
		Name:                         textPtr(row.Name),
		AvatarURL:                    textPtr(row.AvatarUrl),
		DisplayName:                  displayName,
		DisplayAvatarURL:             displayAvatar,
		CreatedBy:                    int8Ptr(row.CreatedBy),
		CreatedAt:                    row.CreatedAt.Time,
		UpdatedAt:                    row.UpdatedAt.Time,
		LastMessageID:                int8Ptr(row.LastMessageID),
		LastMessageContent:           textPtr(row.LastMessageContent),
		LastMessageSenderID:          int8Ptr(row.LastMessageSenderID),
		LastMessageSenderDisplayName: textPtr(row.LastMessageSenderDisplayName),
		LastMessageDeletedAt:         timestamptzPtr(row.LastMessageDeletedAt),
		UnreadCount:                  row.UnreadCount,
	}
}
