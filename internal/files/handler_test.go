package files

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"

	"github.com/apten-chat/messenger/internal/auth"
	"github.com/apten-chat/messenger/internal/chat"
	"github.com/apten-chat/messenger/internal/db/dbq"
	"github.com/apten-chat/messenger/internal/testutil"
	"github.com/go-chi/chi/v5"
)

// ctxWithFile builds a request context carrying auth claims for userID and a
// chi route with the {fileID} URL param populated.
func ctxWithFile(userID, fileID int64) context.Context {
	claims := &auth.Claims{UserID: userID}
	ctx := context.WithValue(context.Background(), auth.ClaimsKey, claims)
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("fileID", strconv.FormatInt(fileID, 10))
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
	return NewHandler(svc, chatSvc)
}

func TestDownload_Forbidden(t *testing.T) {
	h := newForbiddenHandler()

	req := httptest.NewRequest(http.MethodGet, "/api/files/55", nil).WithContext(ctxWithFile(10, 55))
	rr := httptest.NewRecorder()
	h.Download(rr, req)

	if rr.Code != http.StatusForbidden {
		t.Fatalf("status = %d, want %d", rr.Code, http.StatusForbidden)
	}
}

func TestThumbnail_Forbidden(t *testing.T) {
	h := newForbiddenHandler()

	req := httptest.NewRequest(http.MethodGet, "/api/files/55/thumb", nil).WithContext(ctxWithFile(10, 55))
	rr := httptest.NewRecorder()
	h.Thumbnail(rr, req)

	if rr.Code != http.StatusForbidden {
		t.Fatalf("status = %d, want %d", rr.Code, http.StatusForbidden)
	}
}
