package chat

import (
	"testing"

	"github.com/apten-chat/messenger/internal/db/dbq"
	"github.com/jackc/pgx/v5/pgtype"
)

func TestChatListItemDTO_DirectChat(t *testing.T) {
	row := dbq.ListChatsByUserRow{
		ID:                  1,
		Type:                "direct",
		DirectDisplayName:   "Bob",
		DirectAvatarUrl:     pgtype.Text{String: "https://x/bob.png", Valid: true},
		LastMessageID:       pgtype.Int8{Int64: 10, Valid: true},
		LastMessageContent:  pgtype.Text{String: "hi", Valid: true},
		LastMessageSenderID: pgtype.Int8{Int64: 2, Valid: true},
		UnreadCount:         3,
	}
	dto := chatListItemDTO(row)

	if dto.DisplayName != "Bob" {
		t.Errorf("DisplayName = %q, want Bob", dto.DisplayName)
	}
	if dto.DisplayAvatarURL == nil || *dto.DisplayAvatarURL != "https://x/bob.png" {
		t.Errorf("DisplayAvatarURL = %v, want bob.png", dto.DisplayAvatarURL)
	}
	if dto.LastMessageContent == nil || *dto.LastMessageContent != "hi" {
		t.Errorf("LastMessageContent = %v, want hi", dto.LastMessageContent)
	}
	if dto.UnreadCount != 3 {
		t.Errorf("UnreadCount = %d, want 3", dto.UnreadCount)
	}
}

func TestChatListItemDTO_GroupChat(t *testing.T) {
	row := dbq.ListChatsByUserRow{
		ID:        2,
		Type:      "group",
		Name:      pgtype.Text{String: "Team", Valid: true},
		AvatarUrl: pgtype.Text{String: "https://x/team.png", Valid: true},
		// direct_display_name is empty for groups (COALESCE'd to '').
		DirectDisplayName: "",
	}
	dto := chatListItemDTO(row)

	if dto.DisplayName != "Team" {
		t.Errorf("DisplayName = %q, want Team", dto.DisplayName)
	}
	if dto.DisplayAvatarURL == nil || *dto.DisplayAvatarURL != "https://x/team.png" {
		t.Errorf("DisplayAvatarURL = %v, want team.png", dto.DisplayAvatarURL)
	}
}

func TestChatListItemDTO_GroupChat_NoName(t *testing.T) {
	row := dbq.ListChatsByUserRow{ID: 3, Type: "group"}
	dto := chatListItemDTO(row)

	if dto.DisplayName != "Group Chat" {
		t.Errorf("DisplayName = %q, want Group Chat", dto.DisplayName)
	}
}

// A chat with no messages must produce nil last-message pointers (not a panic
// or zero value), since the generated row carries SQL NULLs there.
func TestChatListItemDTO_NoMessages(t *testing.T) {
	row := dbq.ListChatsByUserRow{ID: 4, Type: "direct", DirectDisplayName: "Carol"}
	dto := chatListItemDTO(row)

	if dto.LastMessageID != nil {
		t.Errorf("LastMessageID = %v, want nil", dto.LastMessageID)
	}
	if dto.LastMessageContent != nil {
		t.Errorf("LastMessageContent = %v, want nil", dto.LastMessageContent)
	}
	if dto.LastMessageDeletedAt != nil {
		t.Errorf("LastMessageDeletedAt = %v, want nil", dto.LastMessageDeletedAt)
	}
}
