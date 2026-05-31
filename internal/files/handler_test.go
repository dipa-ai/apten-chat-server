package files

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/apten-chat/messenger/internal/auth"
	"github.com/apten-chat/messenger/internal/chat"
	"github.com/apten-chat/messenger/internal/db/dbq"
	"github.com/apten-chat/messenger/internal/testutil"
	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5/pgtype"
)

// reqCtx builds a request context carrying auth claims for userID and a chi
// route with the given URL params populated.
func reqCtx(userID int64, params map[string]string) context.Context {
	claims := &auth.Claims{UserID: userID}
	ctx := context.WithValue(context.Background(), auth.ClaimsKey, claims)
	rctx := chi.NewRouteContext()
	for k, v := range params {
		rctx.URLParams.Add(k, v)
	}
	return context.WithValue(ctx, chi.RouteCtxKey, rctx)
}

// newForbiddenHandler wires a handler where attachment 55 belongs to chat 7 but
// the requesting user is not a member of that chat.
func newForbiddenHandler() *Handler {
	mock := &testutil.MockQuerier{
		GetAttachmentAccessContextFunc: func(ctx context.Context, id int64) (dbq.GetAttachmentAccessContextRow, error) {
			return dbq.GetAttachmentAccessContextRow{ID: id, ChatID: 7}, nil
		},
		IsChatMemberFunc: func(ctx context.Context, arg dbq.IsChatMemberParams) (bool, error) {
			return false, nil
		},
	}
	svc := NewService(nil, mock, 10*1024*1024)
	chatSvc := chat.NewServiceWithDeps(mock, nil, nil)
	return NewHandler(svc, chatSvc, nil)
}

func TestDownload_Forbidden(t *testing.T) {
	h := newForbiddenHandler()

	req := httptest.NewRequest(http.MethodGet, "/api/files/55", nil).
		WithContext(reqCtx(10, map[string]string{"fileID": "55"}))
	rr := httptest.NewRecorder()
	h.Download(rr, req)

	if rr.Code != http.StatusForbidden {
		t.Fatalf("status = %d, want %d", rr.Code, http.StatusForbidden)
	}
}

func TestThumbnail_Forbidden(t *testing.T) {
	h := newForbiddenHandler()

	req := httptest.NewRequest(http.MethodGet, "/api/files/55/thumb", nil).
		WithContext(reqCtx(10, map[string]string{"fileID": "55"}))
	rr := httptest.NewRecorder()
	h.Thumbnail(rr, req)

	if rr.Code != http.StatusForbidden {
		t.Fatalf("status = %d, want %d", rr.Code, http.StatusForbidden)
	}
}

func TestUpload_ForbiddenNonMember(t *testing.T) {
	mock := &testutil.MockQuerier{
		IsChatMemberFunc: func(ctx context.Context, arg dbq.IsChatMemberParams) (bool, error) {
			return false, nil
		},
	}
	svc := NewService(nil, mock, 10*1024*1024)
	h := NewHandler(svc, chat.NewServiceWithDeps(mock, nil, nil), nil)

	req := httptest.NewRequest(http.MethodPost, "/api/chats/7/upload", nil).
		WithContext(reqCtx(10, map[string]string{"id": "7"}))
	rr := httptest.NewRecorder()
	h.Upload(rr, req)

	if rr.Code != http.StatusForbidden {
		t.Fatalf("status = %d, want %d", rr.Code, http.StatusForbidden)
	}
}

type fakeBroadcaster struct {
	userIDs   []int64
	eventType string
	payload   any
	calls     int
}

func (f *fakeBroadcaster) SendJSON(userIDs []int64, eventType string, payload any) {
	f.calls++
	f.userIDs = userIDs
	f.eventType = eventType
	f.payload = payload
}

func TestBroadcastUpload_NotifiesAllMembers(t *testing.T) {
	mock := &testutil.MockQuerier{
		ListChatMembersFunc: func(ctx context.Context, chatID int64) ([]dbq.ListChatMembersRow, error) {
			return []dbq.ListChatMembersRow{{ID: 10}, {ID: 11}}, nil
		},
		GetUserByIDFunc: func(ctx context.Context, id int64) (dbq.User, error) {
			return dbq.User{ID: 10, DisplayName: "Alex"}, nil
		},
	}
	fb := &fakeBroadcaster{}
	svc := NewService(nil, mock, 10*1024*1024)
	h := NewHandler(svc, chat.NewServiceWithDeps(mock, nil, nil), fb)

	result := &UploadResult{
		Message: dbq.Message{ID: 101, ChatID: 7, SenderID: 10},
		Attachment: dbq.Attachment{
			ID:            55,
			MessageID:     101,
			FileName:      "photo.jpg",
			FileSize:      12345,
			MimeType:      "image/jpeg",
			StoragePath:   "chats/7/2026/05/photo.jpg",
			ThumbnailPath: pgtype.Text{String: "chats/7/2026/05/photo_thumb.jpg", Valid: true},
		},
	}

	h.broadcastUpload(context.Background(), 7, 10, result)

	if fb.calls != 1 {
		t.Fatalf("broadcast calls = %d, want 1", fb.calls)
	}
	if fb.eventType != "message.new" {
		t.Errorf("event type = %q, want message.new", fb.eventType)
	}
	if len(fb.userIDs) != 2 || fb.userIDs[0] != 10 || fb.userIDs[1] != 11 {
		t.Errorf("userIDs = %v, want [10 11]", fb.userIDs)
	}

	payload, ok := fb.payload.(map[string]any)
	if !ok {
		t.Fatalf("payload type = %T, want map[string]any", fb.payload)
	}
	if payload["sender_name"] != "Alex" {
		t.Errorf("sender_name = %v, want Alex", payload["sender_name"])
	}
	if payload["id"] != int64(101) {
		t.Errorf("message id = %v, want 101", payload["id"])
	}
	atts, ok := payload["attachments"].([]map[string]any)
	if !ok {
		t.Fatalf("attachments type = %T, want []map[string]any", payload["attachments"])
	}
	if len(atts) != 1 {
		t.Fatalf("attachments len = %d, want 1", len(atts))
	}
	if atts[0]["id"] != int64(55) {
		t.Errorf("attachment id = %v, want 55", atts[0]["id"])
	}
	if atts[0]["file_name"] != "photo.jpg" {
		t.Errorf("attachment file_name = %v, want photo.jpg", atts[0]["file_name"])
	}
}
