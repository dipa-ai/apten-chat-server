package user

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
	"github.com/jackc/pgx/v5/pgtype"
)

func ctxWithClaims(userID int64, isAdmin bool) context.Context {
	claims := &auth.Claims{UserID: userID, IsAdmin: isAdmin}
	return context.WithValue(context.Background(), auth.ClaimsKey, claims)
}

func TestRegisterHandler_MissingFields(t *testing.T) {
	h := NewHandler(nil, &testutil.MockQuerier{})

	tests := []struct {
		name string
		body string
	}{
		{"empty code", `{"code":"","username":"u","display_name":"d","password":"123456"}`},
		{"empty username", `{"code":"c","username":"","display_name":"d","password":"123456"}`},
		{"empty display_name", `{"code":"c","username":"u","display_name":"","password":"123456"}`},
		{"empty password", `{"code":"c","username":"u","display_name":"d","password":""}`},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodPost, "/api/register", strings.NewReader(tt.body))
			rec := httptest.NewRecorder()
			h.Register(rec, req)

			if rec.Code != http.StatusBadRequest {
				t.Errorf("status = %d, want %d", rec.Code, http.StatusBadRequest)
			}
		})
	}
}

func TestRegisterHandler_ShortPassword(t *testing.T) {
	h := NewHandler(nil, &testutil.MockQuerier{})

	body := `{"code":"c","username":"u","display_name":"d","password":"12345"}`
	req := httptest.NewRequest(http.MethodPost, "/api/register", strings.NewReader(body))
	rec := httptest.NewRecorder()
	h.Register(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusBadRequest)
	}
	var resp map[string]string
	json.NewDecoder(rec.Body).Decode(&resp)
	if !strings.Contains(resp["error"], "6 characters") {
		t.Errorf("error = %q, want password length message", resp["error"])
	}
}

func TestRegisterHandler_InvalidJSON(t *testing.T) {
	h := NewHandler(nil, &testutil.MockQuerier{})

	req := httptest.NewRequest(http.MethodPost, "/api/register", strings.NewReader("not-json"))
	rec := httptest.NewRecorder()
	h.Register(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusBadRequest)
	}
}

func TestLoginHandler_InvalidJSON(t *testing.T) {
	h := NewHandler(nil, &testutil.MockQuerier{})

	req := httptest.NewRequest(http.MethodPost, "/api/login", strings.NewReader("{invalid"))
	rec := httptest.NewRecorder()
	h.Login(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusBadRequest)
	}
}

func TestRefreshHandler_InvalidJSON(t *testing.T) {
	h := NewHandler(nil, &testutil.MockQuerier{})

	req := httptest.NewRequest(http.MethodPost, "/api/refresh", strings.NewReader("{bad"))
	rec := httptest.NewRecorder()
	h.Refresh(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusBadRequest)
	}
}

func TestGetMeHandler_Success(t *testing.T) {
	mock := &testutil.MockQuerier{
		GetUserByIDFunc: func(ctx context.Context, id int64) (dbq.User, error) {
			return dbq.User{
				ID:          id,
				Username:    "alice",
				DisplayName: "Alice",
				IsAdmin:     pgtype.Bool{Bool: true, Valid: true},
			}, nil
		},
	}
	h := NewHandler(nil, mock)

	req := httptest.NewRequest(http.MethodGet, "/api/users/me", nil)
	req = req.WithContext(ctxWithClaims(1, true))
	rec := httptest.NewRecorder()
	h.GetMe(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
	}
	var resp userPublic
	json.NewDecoder(rec.Body).Decode(&resp)
	if resp.Username != "alice" {
		t.Errorf("Username = %q, want alice", resp.Username)
	}
	if !resp.IsAdmin {
		t.Error("expected IsAdmin=true")
	}
}

func TestListUsersHandler_Success(t *testing.T) {
	mock := &testutil.MockQuerier{
		ListUsersFunc: func(ctx context.Context) ([]dbq.ListUsersRow, error) {
			return []dbq.ListUsersRow{
				{ID: 1, Username: "alice"},
				{ID: 2, Username: "bob"},
			}, nil
		},
	}
	h := NewHandler(nil, mock)

	req := httptest.NewRequest(http.MethodGet, "/api/users", nil)
	req = req.WithContext(ctxWithClaims(1, true))
	rec := httptest.NewRecorder()
	h.ListUsers(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusOK)
	}
}

func TestWriteJSON(t *testing.T) {
	rec := httptest.NewRecorder()
	writeJSON(rec, http.StatusCreated, map[string]string{"key": "value"})

	if rec.Code != http.StatusCreated {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusCreated)
	}
	if ct := rec.Header().Get("Content-Type"); ct != "application/json" {
		t.Errorf("Content-Type = %q, want application/json", ct)
	}
}
