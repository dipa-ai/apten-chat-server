package auth

import (
	"testing"
	"time"
)

func TestGenerateAndParseAccessToken(t *testing.T) {
	secret := "test-secret-key"
	userID := int64(42)
	isAdmin := true
	ttl := 15 * time.Minute

	token, err := GenerateAccessToken(secret, userID, isAdmin, ttl)
	if err != nil {
		t.Fatalf("GenerateAccessToken: %v", err)
	}
	if token == "" {
		t.Fatal("expected non-empty token")
	}

	claims, err := ParseAccessToken(secret, token)
	if err != nil {
		t.Fatalf("ParseAccessToken: %v", err)
	}
	if claims.UserID != userID {
		t.Errorf("UserID = %d, want %d", claims.UserID, userID)
	}
	if claims.IsAdmin != isAdmin {
		t.Errorf("IsAdmin = %v, want %v", claims.IsAdmin, isAdmin)
	}
}

func TestParseAccessToken_WrongSecret(t *testing.T) {
	token, _ := GenerateAccessToken("secret-1", 1, false, time.Hour)
	_, err := ParseAccessToken("secret-2", token)
	if err == nil {
		t.Fatal("expected error for wrong secret")
	}
}

func TestParseAccessToken_Expired(t *testing.T) {
	token, _ := GenerateAccessToken("secret", 1, false, -time.Hour)
	_, err := ParseAccessToken("secret", token)
	if err == nil {
		t.Fatal("expected error for expired token")
	}
}

func TestParseAccessToken_InvalidString(t *testing.T) {
	_, err := ParseAccessToken("secret", "not-a-jwt")
	if err == nil {
		t.Fatal("expected error for invalid token")
	}
}

func TestParseAccessToken_NonAdmin(t *testing.T) {
	token, _ := GenerateAccessToken("secret", 99, false, time.Hour)
	claims, err := ParseAccessToken("secret", token)
	if err != nil {
		t.Fatalf("ParseAccessToken: %v", err)
	}
	if claims.IsAdmin {
		t.Error("expected IsAdmin=false")
	}
}

func TestGenerateRefreshToken(t *testing.T) {
	raw, hash, err := GenerateRefreshToken()
	if err != nil {
		t.Fatalf("GenerateRefreshToken: %v", err)
	}
	if len(raw) != 64 { // 32 bytes hex-encoded
		t.Errorf("raw token length = %d, want 64", len(raw))
	}
	if len(hash) != 64 {
		t.Errorf("hash length = %d, want 64", len(hash))
	}
	if raw == hash {
		t.Error("raw and hash should differ")
	}
}

func TestGenerateRefreshToken_Unique(t *testing.T) {
	raw1, _, _ := GenerateRefreshToken()
	raw2, _, _ := GenerateRefreshToken()
	if raw1 == raw2 {
		t.Error("two generated tokens should not be equal")
	}
}

func TestHashToken_Deterministic(t *testing.T) {
	token := "test-token-123"
	h1 := HashToken(token)
	h2 := HashToken(token)
	if h1 != h2 {
		t.Error("HashToken should be deterministic")
	}
}

func TestHashToken_DifferentInputs(t *testing.T) {
	h1 := HashToken("token-a")
	h2 := HashToken("token-b")
	if h1 == h2 {
		t.Error("different inputs should produce different hashes")
	}
}
