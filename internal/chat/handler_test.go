package chat

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/apten-chat/messenger/internal/auth"
	"github.com/apten-chat/messenger/internal/db/dbq"
	"github.com/apten-chat/messenger/internal/testutil"
)

func ctxWithClaims(userID int64, isAdmin bool) context.Context {
	claims := &auth.Claims{UserID: userID, IsAdmin: isAdmin}
	return context.WithValue(context.Background(), auth.ClaimsKey, claims)
}

func TestCreateHandler_InvalidJSON(t *testing.T) {
	h := NewHandler(nil, &testutil.MockQuerier{})

	req := httptest.NewRequest(http.MethodPost, "/api/chats", strings.NewReader("not-json"))
	req = req.WithContext(ctxWithClaims(1, false))
	rec := httptest.NewRecorder()
	h.Create(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusBadRequest)
	}
}

func TestCreateHandler_InvalidType(t *testing.T) {
	h := NewHandler(nil, &testutil.MockQuerier{})

	body := `{"type":"invalid","member_ids":[2]}`
	req := httptest.NewRequest(http.MethodPost, "/api/chats", strings.NewReader(body))
	req = req.WithContext(ctxWithClaims(1, false))
	rec := httptest.NewRecorder()
	h.Create(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusBadRequest)
	}
	var resp map[string]string
	json.NewDecoder(rec.Body).Decode(&resp)
	if !strings.Contains(resp["error"], "direct") {
		t.Errorf("error = %q, want type validation message", resp["error"])
	}
}

func TestListHandler_Success(t *testing.T) {
	mock := &testutil.MockQuerier{
		ListChatsByUserFunc: func(ctx context.Context, userID int64) ([]dbq.ListChatsByUserRow, error) {
			return []dbq.ListChatsByUserRow{}, nil
		},
	}
	h := NewHandler(nil, mock)

	req := httptest.NewRequest(http.MethodGet, "/api/chats", nil)
	req = req.WithContext(ctxWithClaims(1, false))
	rec := httptest.NewRecorder()
	h.List(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusOK)
	}
}

func TestWriteJSON(t *testing.T) {
	rec := httptest.NewRecorder()
	writeJSON(rec, http.StatusOK, map[string]int{"count": 5})

	if rec.Code != http.StatusOK {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusOK)
	}
	if ct := rec.Header().Get("Content-Type"); ct != "application/json" {
		t.Errorf("Content-Type = %q, want application/json", ct)
	}
}
