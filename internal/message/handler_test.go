package message

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/apten-chat/messenger/internal/auth"
	"github.com/apten-chat/messenger/internal/chat"
	"github.com/apten-chat/messenger/internal/db/dbq"
	"github.com/apten-chat/messenger/internal/testutil"
	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5/pgtype"
)

// captureBroadcaster records the most recent SendJSON call so tests can
// assert on the event type and payload that members would receive.
type captureBroadcaster struct {
	userIDs   []int64
	eventType string
	payload   any
	calls     int
}

func (c *captureBroadcaster) SendJSON(userIDs []int64, eventType string, payload any) {
	c.calls++
	c.userIDs = userIDs
	c.eventType = eventType
	c.payload = payload
}

// Deleting a message must soft-delete it (SoftDeleteMessage) and broadcast a
// message.deleted event carrying the message ID and chat ID to the members.
func TestDeleteMessage_BroadcastsDeletion(t *testing.T) {
	softDeleted := int64(0)
	mock := &testutil.MockQuerier{
		IsChatMemberFunc: func(ctx context.Context, arg dbq.IsChatMemberParams) (bool, error) {
			return true, nil
		},
		GetMessageByIDFunc: func(ctx context.Context, id int64) (dbq.GetMessageByIDRow, error) {
			return dbq.GetMessageByIDRow{ID: 123, ChatID: 7, SenderID: 10}, nil
		},
		SoftDeleteMessageFunc: func(ctx context.Context, id int64) error {
			softDeleted = id
			return nil
		},
		ListChatMembersFunc: func(ctx context.Context, chatID int64) ([]dbq.ListChatMembersRow, error) {
			return []dbq.ListChatMembersRow{{ID: 10}, {ID: 11}}, nil
		},
	}
	bc := &captureBroadcaster{}
	h := NewHandler(nil, chat.NewServiceWithDeps(mock, nil, nil), mock, bc)

	req := httptest.NewRequest(http.MethodDelete, "/api/chats/7/messages/123", nil).
		WithContext(reqCtx(10, map[string]string{"id": "7", "mid": "123"}))
	rec := httptest.NewRecorder()
	h.DeleteMessage(rec, req)

	if rec.Code != http.StatusNoContent {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusNoContent)
	}
	if softDeleted != 123 {
		t.Errorf("SoftDeleteMessage called with id=%d, want 123", softDeleted)
	}
	if bc.calls != 1 {
		t.Fatalf("broadcaster called %d times, want 1", bc.calls)
	}
	if bc.eventType != "message.deleted" {
		t.Errorf("event type = %q, want message.deleted", bc.eventType)
	}
	p, ok := bc.payload.(map[string]any)
	if !ok {
		t.Fatalf("payload type = %T, want map[string]any", bc.payload)
	}
	if p["id"] != int64(123) {
		t.Errorf("payload id = %v, want 123", p["id"])
	}
	if p["chat_id"] != int64(7) {
		t.Errorf("payload chat_id = %v, want 7", p["chat_id"])
	}
}

func TestWriteJSON(t *testing.T) {
	rec := httptest.NewRecorder()
	writeJSON(rec, http.StatusOK, map[string]string{"status": "ok"})

	if rec.Code != http.StatusOK {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusOK)
	}
	if ct := rec.Header().Get("Content-Type"); ct != "application/json" {
		t.Errorf("Content-Type = %q, want application/json", ct)
	}
}

// reqCtx builds a request context with auth claims and chi URL params.
func reqCtx(userID int64, params map[string]string) context.Context {
	claims := &auth.Claims{UserID: userID}
	ctx := context.WithValue(context.Background(), auth.ClaimsKey, claims)
	rctx := chi.NewRouteContext()
	for k, v := range params {
		rctx.URLParams.Add(k, v)
	}
	return context.WithValue(ctx, chi.RouteCtxKey, rctx)
}

func TestListMessages_WithAttachments(t *testing.T) {
	mock := &testutil.MockQuerier{
		IsChatMemberFunc: func(ctx context.Context, arg dbq.IsChatMemberParams) (bool, error) {
			return true, nil
		},
		ListMessagesByChatLatestFunc: func(ctx context.Context, arg dbq.ListMessagesByChatLatestParams) ([]dbq.ListMessagesByChatLatestRow, error) {
			return []dbq.ListMessagesByChatLatestRow{
				{ID: 101, ChatID: 7, SenderID: 10, SenderDisplayName: "Alex"},
				{ID: 102, ChatID: 7, SenderID: 11, SenderDisplayName: "Bob", Content: pgtype.Text{String: "hi", Valid: true}},
			}, nil
		},
		ListAttachmentsByMessageIDsFunc: func(ctx context.Context, ids []int64) ([]dbq.Attachment, error) {
			return []dbq.Attachment{
				{
					ID:            55,
					MessageID:     101,
					FileName:      "photo.jpg",
					FileSize:      12345,
					MimeType:      "image/jpeg",
					StoragePath:   "chats/7/2026/05/photo.jpg",
					ThumbnailPath: pgtype.Text{String: "chats/7/2026/05/photo_thumb.jpg", Valid: true},
				},
			}, nil
		},
	}
	h := NewHandler(nil, chat.NewServiceWithDeps(mock, nil, nil), mock, nil)

	req := httptest.NewRequest(http.MethodGet, "/api/chats/7/messages", nil).
		WithContext(reqCtx(10, map[string]string{"id": "7"}))
	rec := httptest.NewRecorder()
	h.ListMessages(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
	}

	var resp []map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode: %v (body=%s)", err, rec.Body.String())
	}
	if len(resp) != 2 {
		t.Fatalf("got %d messages, want 2", len(resp))
	}

	// Message 101: one attachment.
	atts0, ok := resp[0]["attachments"].([]any)
	if !ok {
		t.Fatalf("msg[0].attachments is not a JSON array: %T", resp[0]["attachments"])
	}
	if len(atts0) != 1 {
		t.Fatalf("msg[0] attachments = %d, want 1", len(atts0))
	}
	att := atts0[0].(map[string]any)
	if att["file_name"] != "photo.jpg" {
		t.Errorf("attachment file_name = %v, want photo.jpg", att["file_name"])
	}
	if att["id"].(float64) != 55 {
		t.Errorf("attachment id = %v, want 55", att["id"])
	}

	// Message 102: no attachments must serialize as [] (a JSON array), not null.
	atts1, ok := resp[1]["attachments"].([]any)
	if !ok {
		t.Fatalf("msg[1].attachments is not a JSON array (got %v / %T) — expected [] not null", resp[1]["attachments"], resp[1]["attachments"])
	}
	if len(atts1) != 0 {
		t.Errorf("msg[1] attachments = %d, want 0", len(atts1))
	}
}

func TestGetMessage_EmptyAttachmentsArray(t *testing.T) {
	mock := &testutil.MockQuerier{
		IsChatMemberFunc: func(ctx context.Context, arg dbq.IsChatMemberParams) (bool, error) {
			return true, nil
		},
		GetMessageByIDFunc: func(ctx context.Context, id int64) (dbq.GetMessageByIDRow, error) {
			return dbq.GetMessageByIDRow{
				ID:                200,
				ChatID:            7,
				SenderID:          10,
				SenderDisplayName: "Alex",
				Content:           pgtype.Text{String: "yo", Valid: true},
			}, nil
		},
		ListAttachmentsByMessageIDsFunc: func(ctx context.Context, ids []int64) ([]dbq.Attachment, error) {
			return []dbq.Attachment{}, nil
		},
	}
	h := NewHandler(nil, chat.NewServiceWithDeps(mock, nil, nil), mock, nil)

	req := httptest.NewRequest(http.MethodGet, "/api/chats/7/messages/200", nil).
		WithContext(reqCtx(10, map[string]string{"id": "7", "mid": "200"}))
	rec := httptest.NewRecorder()
	h.GetMessage(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
	}

	var resp map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	atts, ok := resp["attachments"].([]any)
	if !ok {
		t.Fatalf("attachments is not a JSON array (got %v / %T) — expected [] not null", resp["attachments"], resp["attachments"])
	}
	if len(atts) != 0 {
		t.Errorf("attachments = %d, want 0", len(atts))
	}
}

// A member of the URL's chat must not be able to read a message (or its
// attachment metadata) that belongs to a different chat.
func TestGetMessage_CrossChatDenied(t *testing.T) {
	attachmentsLoaded := false
	mock := &testutil.MockQuerier{
		IsChatMemberFunc: func(ctx context.Context, arg dbq.IsChatMemberParams) (bool, error) {
			return true, nil // user is a member of chat 7 (the URL chat)
		},
		GetMessageByIDFunc: func(ctx context.Context, id int64) (dbq.GetMessageByIDRow, error) {
			// Message 200 actually belongs to chat 99, not 7.
			return dbq.GetMessageByIDRow{
				ID:                200,
				ChatID:            99,
				SenderID:          42,
				SenderDisplayName: "Mallory",
				Content:           pgtype.Text{String: "secret", Valid: true},
			}, nil
		},
		ListAttachmentsByMessageIDsFunc: func(ctx context.Context, ids []int64) ([]dbq.Attachment, error) {
			attachmentsLoaded = true
			return []dbq.Attachment{{ID: 55, MessageID: 200, FileName: "secret.pdf"}}, nil
		},
	}
	h := NewHandler(nil, chat.NewServiceWithDeps(mock, nil, nil), mock, nil)

	req := httptest.NewRequest(http.MethodGet, "/api/chats/7/messages/200", nil).
		WithContext(reqCtx(10, map[string]string{"id": "7", "mid": "200"}))
	rec := httptest.NewRecorder()
	h.GetMessage(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusNotFound)
	}
	if attachmentsLoaded {
		t.Error("attachments were loaded for a cross-chat message; metadata may leak")
	}
	if body := rec.Body.String(); strings.Contains(body, "secret") || strings.Contains(body, "55") {
		t.Errorf("response leaked cross-chat content: %s", body)
	}
}
