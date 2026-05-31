package ws

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/apten-chat/messenger/internal/auth"
)

func newTestHandler(jwtSecret string, allowedOrigins []string) *Handler {
	return NewHandler(NewHub(), nil, nil, nil, jwtSecret, allowedOrigins)
}

func TestServeHTTP_RejectsDisallowedOrigin(t *testing.T) {
	h := newTestHandler("secret", []string{"http://localhost:3000"})

	req := httptest.NewRequest(http.MethodGet, "/api/ws", nil)
	req.Header.Set("Origin", "http://evil.example.com")
	req.Header.Set("Sec-WebSocket-Protocol", wsTokenPrefix+"whatever")
	rec := httptest.NewRecorder()

	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusForbidden {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusForbidden)
	}
}

func TestServeHTTP_MissingToken(t *testing.T) {
	h := newTestHandler("secret", nil) // no origin allowlist

	req := httptest.NewRequest(http.MethodGet, "/api/ws", nil)
	rec := httptest.NewRecorder()

	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusUnauthorized)
	}
}

func TestServeHTTP_InvalidToken(t *testing.T) {
	h := newTestHandler("secret", nil)

	req := httptest.NewRequest(http.MethodGet, "/api/ws", nil)
	req.Header.Set("Sec-WebSocket-Protocol", wsTokenPrefix+"not-a-real-jwt")
	rec := httptest.NewRecorder()

	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusUnauthorized)
	}
}

// A request whose Origin is allowed and whose subprotocol carries a valid token
// must pass both gates. We can't complete the WebSocket upgrade with httptest
// (no hijackable conn), but reaching websocket.Accept means auth succeeded — so
// the response must NOT be a 401/403.
func TestServeHTTP_AllowsValidOriginAndToken(t *testing.T) {
	secret := "secret"
	token, err := auth.GenerateAccessToken(secret, 42, false, time.Hour)
	if err != nil {
		t.Fatalf("generate token: %v", err)
	}
	h := newTestHandler(secret, []string{"http://localhost:3000"})

	req := httptest.NewRequest(http.MethodGet, "/api/ws", nil)
	req.Header.Set("Origin", "http://localhost:3000")
	req.Header.Set("Sec-WebSocket-Protocol", wsTokenPrefix+token)
	rec := httptest.NewRecorder()

	h.ServeHTTP(rec, req)

	if rec.Code == http.StatusUnauthorized || rec.Code == http.StatusForbidden {
		t.Fatalf("auth gate rejected a valid request: status = %d", rec.Code)
	}
}

func TestProtocolFromRequest(t *testing.T) {
	tests := []struct {
		name   string
		header string
		want   string
	}{
		{"single", wsTokenPrefix + "abc.def.ghi", wsTokenPrefix + "abc.def.ghi"},
		{"comma-separated", "json, " + wsTokenPrefix + "tok", wsTokenPrefix + "tok"},
		{"none", "some-other-proto", ""},
		{"empty", "", ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, "/api/ws", nil)
			if tt.header != "" {
				req.Header.Set("Sec-WebSocket-Protocol", tt.header)
			}
			if got := protocolFromRequest(req); got != tt.want {
				t.Errorf("protocolFromRequest = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestOriginAllowed(t *testing.T) {
	allowed := []string{"http://localhost:3000", " https://chat.example.com "}
	cases := map[string]bool{
		"http://localhost:3000":     true,
		"https://chat.example.com":  true, // trimmed match
		"http://evil.example.com":   false,
		"":                          false,
		"http://localhost:3000/foo": false,
	}
	for origin, want := range cases {
		if got := originAllowed(origin, allowed); got != want {
			t.Errorf("originAllowed(%q) = %v, want %v", origin, got, want)
		}
	}
}
