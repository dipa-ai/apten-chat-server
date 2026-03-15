package auth

import "testing"

func TestHashAndCheckPassword(t *testing.T) {
	password := "my-secret-password"
	hash, err := HashPassword(password)
	if err != nil {
		t.Fatalf("HashPassword: %v", err)
	}
	if hash == "" {
		t.Fatal("expected non-empty hash")
	}
	if hash == password {
		t.Fatal("hash should not equal plaintext")
	}
	if !CheckPassword(hash, password) {
		t.Error("CheckPassword should return true for correct password")
	}
}

func TestCheckPassword_Wrong(t *testing.T) {
	hash, _ := HashPassword("correct-password")
	if CheckPassword(hash, "wrong-password") {
		t.Error("CheckPassword should return false for wrong password")
	}
}

func TestHashPassword_Unique(t *testing.T) {
	h1, _ := HashPassword("same-password")
	h2, _ := HashPassword("same-password")
	if h1 == h2 {
		t.Error("bcrypt hashes should include random salt and differ")
	}
}
