package user

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/apten-chat/messenger/internal/auth"
	"github.com/apten-chat/messenger/internal/db/dbq"
	"github.com/apten-chat/messenger/internal/testutil"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
)

func newTestService(q dbq.Querier, txQ dbq.Querier) *Service {
	tx := &testutil.MockTx{}
	return NewServiceWithDeps(
		q, // queries
		func(ctx context.Context) (pgx.Tx, error) { return tx, nil },
		func(t pgx.Tx) dbq.Querier { return txQ },
		"test-secret",
		15*time.Minute,
		720*time.Hour,
	)
}

func TestLogin_Success(t *testing.T) {
	hash, _ := auth.HashPassword("password123")
	mock := &testutil.MockQuerier{
		GetUserByUsernameFunc: func(ctx context.Context, username string) (dbq.User, error) {
			if username != "alice" {
				t.Errorf("username = %q, want alice", username)
			}
			return dbq.User{
				ID:           1,
				Username:     "alice",
				PasswordHash: hash,
				IsAdmin:      pgtype.Bool{Bool: false, Valid: true},
			}, nil
		},
		CreateRefreshTokenFunc: func(ctx context.Context, arg dbq.CreateRefreshTokenParams) error {
			if !arg.UserID.Valid || arg.UserID.Int64 != 1 {
				t.Errorf("UserID = %v, want 1", arg.UserID)
			}
			return nil
		},
	}

	svc := newTestService(mock, mock)
	tokens, err := svc.Login(context.Background(), "alice", "password123")
	if err != nil {
		t.Fatalf("Login: %v", err)
	}
	if tokens.AccessToken == "" {
		t.Error("AccessToken should not be empty")
	}
	if tokens.RefreshToken == "" {
		t.Error("RefreshToken should not be empty")
	}

	// Verify the access token is parseable.
	claims, err := auth.ParseAccessToken("test-secret", tokens.AccessToken)
	if err != nil {
		t.Fatalf("ParseAccessToken: %v", err)
	}
	if claims.UserID != 1 {
		t.Errorf("UserID = %d, want 1", claims.UserID)
	}
	if claims.IsAdmin {
		t.Error("expected IsAdmin=false")
	}
}

func TestLogin_WrongPassword(t *testing.T) {
	hash, _ := auth.HashPassword("correct-password")
	mock := &testutil.MockQuerier{
		GetUserByUsernameFunc: func(ctx context.Context, username string) (dbq.User, error) {
			return dbq.User{PasswordHash: hash}, nil
		},
	}

	svc := newTestService(mock, mock)
	_, err := svc.Login(context.Background(), "alice", "wrong-password")
	if !errors.Is(err, ErrInvalidCredentials) {
		t.Errorf("err = %v, want ErrInvalidCredentials", err)
	}
}

func TestLogin_UserNotFound(t *testing.T) {
	mock := &testutil.MockQuerier{
		GetUserByUsernameFunc: func(ctx context.Context, username string) (dbq.User, error) {
			return dbq.User{}, pgx.ErrNoRows
		},
	}

	svc := newTestService(mock, mock)
	_, err := svc.Login(context.Background(), "nobody", "pass")
	if !errors.Is(err, ErrInvalidCredentials) {
		t.Errorf("err = %v, want ErrInvalidCredentials", err)
	}
}

func TestRegister_Success(t *testing.T) {
	mock := &testutil.MockQuerier{
		GetInviteByCodeFunc: func(ctx context.Context, code string) (dbq.Invite, error) {
			return dbq.Invite{
				ID:        1,
				Code:      code,
				ExpiresAt: pgtype.Timestamptz{Time: time.Now().Add(time.Hour), Valid: true},
			}, nil
		},
		CountUsersFunc: func(ctx context.Context) (int64, error) {
			return 0, nil // First user → admin.
		},
		CreateRefreshTokenFunc: func(ctx context.Context, arg dbq.CreateRefreshTokenParams) error {
			return nil
		},
	}

	txMock := &testutil.MockQuerier{
		CreateUserFunc: func(ctx context.Context, arg dbq.CreateUserParams) (dbq.CreateUserRow, error) {
			if arg.Username != "bob" {
				t.Errorf("username = %q, want bob", arg.Username)
			}
			if !arg.IsAdmin.Bool {
				t.Error("first user should be admin")
			}
			return dbq.CreateUserRow{ID: 1, IsAdmin: arg.IsAdmin}, nil
		},
		MarkInviteUsedFunc: func(ctx context.Context, arg dbq.MarkInviteUsedParams) error {
			if arg.UsedBy.Int64 != 1 {
				t.Errorf("UsedBy = %d, want 1", arg.UsedBy.Int64)
			}
			return nil
		},
	}

	svc := newTestService(mock, txMock)
	tokens, err := svc.Register(context.Background(), "invite-code", "bob", "Bob", "password123")
	if err != nil {
		t.Fatalf("Register: %v", err)
	}
	if tokens.AccessToken == "" || tokens.RefreshToken == "" {
		t.Error("tokens should not be empty")
	}
}

func TestRegister_InvalidInvite(t *testing.T) {
	mock := &testutil.MockQuerier{
		GetInviteByCodeFunc: func(ctx context.Context, code string) (dbq.Invite, error) {
			return dbq.Invite{}, pgx.ErrNoRows
		},
	}

	svc := newTestService(mock, mock)
	_, err := svc.Register(context.Background(), "bad-code", "bob", "Bob", "password123")
	if !errors.Is(err, ErrInviteInvalid) {
		t.Errorf("err = %v, want ErrInviteInvalid", err)
	}
}

func TestRegister_ExpiredInvite(t *testing.T) {
	mock := &testutil.MockQuerier{
		GetInviteByCodeFunc: func(ctx context.Context, code string) (dbq.Invite, error) {
			return dbq.Invite{
				ExpiresAt: pgtype.Timestamptz{Time: time.Now().Add(-time.Hour), Valid: true},
			}, nil
		},
	}

	svc := newTestService(mock, mock)
	_, err := svc.Register(context.Background(), "expired", "bob", "Bob", "password123")
	if !errors.Is(err, ErrInviteInvalid) {
		t.Errorf("err = %v, want ErrInviteInvalid", err)
	}
}

func TestRegister_UsedInvite(t *testing.T) {
	mock := &testutil.MockQuerier{
		GetInviteByCodeFunc: func(ctx context.Context, code string) (dbq.Invite, error) {
			return dbq.Invite{
				UsedBy:    pgtype.Int8{Int64: 99, Valid: true},
				ExpiresAt: pgtype.Timestamptz{Time: time.Now().Add(time.Hour), Valid: true},
			}, nil
		},
	}

	svc := newTestService(mock, mock)
	_, err := svc.Register(context.Background(), "used", "bob", "Bob", "password123")
	if !errors.Is(err, ErrInviteInvalid) {
		t.Errorf("err = %v, want ErrInviteInvalid", err)
	}
}

func TestRegister_UsernameTaken(t *testing.T) {
	mock := &testutil.MockQuerier{
		GetInviteByCodeFunc: func(ctx context.Context, code string) (dbq.Invite, error) {
			return dbq.Invite{
				ID:        1,
				ExpiresAt: pgtype.Timestamptz{Time: time.Now().Add(time.Hour), Valid: true},
			}, nil
		},
		CountUsersFunc: func(ctx context.Context) (int64, error) {
			return 5, nil
		},
	}
	txMock := &testutil.MockQuerier{
		CreateUserFunc: func(ctx context.Context, arg dbq.CreateUserParams) (dbq.CreateUserRow, error) {
			return dbq.CreateUserRow{}, errors.New("unique constraint violation")
		},
	}

	svc := newTestService(mock, txMock)
	_, err := svc.Register(context.Background(), "code", "taken", "Taken", "password123")
	if !errors.Is(err, ErrUsernameTaken) {
		t.Errorf("err = %v, want ErrUsernameTaken", err)
	}
}

func TestRegister_SecondUser_NotAdmin(t *testing.T) {
	mock := &testutil.MockQuerier{
		GetInviteByCodeFunc: func(ctx context.Context, code string) (dbq.Invite, error) {
			return dbq.Invite{
				ID:        1,
				ExpiresAt: pgtype.Timestamptz{Time: time.Now().Add(time.Hour), Valid: true},
			}, nil
		},
		CountUsersFunc: func(ctx context.Context) (int64, error) {
			return 1, nil // Not the first user.
		},
		CreateRefreshTokenFunc: func(ctx context.Context, arg dbq.CreateRefreshTokenParams) error {
			return nil
		},
	}
	txMock := &testutil.MockQuerier{
		CreateUserFunc: func(ctx context.Context, arg dbq.CreateUserParams) (dbq.CreateUserRow, error) {
			if arg.IsAdmin.Bool {
				t.Error("second user should not be admin")
			}
			return dbq.CreateUserRow{ID: 2, IsAdmin: arg.IsAdmin}, nil
		},
		MarkInviteUsedFunc: func(ctx context.Context, arg dbq.MarkInviteUsedParams) error {
			return nil
		},
	}

	svc := newTestService(mock, txMock)
	_, err := svc.Register(context.Background(), "code", "bob2", "Bob2", "password123")
	if err != nil {
		t.Fatalf("Register: %v", err)
	}
}

func TestRefreshToken_Success(t *testing.T) {
	rawToken := "abc123"
	tokenHash := auth.HashToken(rawToken)

	mock := &testutil.MockQuerier{
		GetRefreshTokenFunc: func(ctx context.Context, hash string) (dbq.RefreshToken, error) {
			if hash != tokenHash {
				t.Errorf("hash = %q, want %q", hash, tokenHash)
			}
			return dbq.RefreshToken{
				UserID:    pgtype.Int8{Int64: 1, Valid: true},
				TokenHash: hash,
				ExpiresAt: pgtype.Timestamptz{Time: time.Now().Add(time.Hour), Valid: true},
			}, nil
		},
		DeleteRefreshTokenFunc: func(ctx context.Context, hash string) error {
			return nil
		},
		GetUserByIDFunc: func(ctx context.Context, id int64) (dbq.User, error) {
			return dbq.User{ID: 1, IsAdmin: pgtype.Bool{Bool: true, Valid: true}}, nil
		},
		CreateRefreshTokenFunc: func(ctx context.Context, arg dbq.CreateRefreshTokenParams) error {
			return nil
		},
	}

	svc := newTestService(mock, mock)
	tokens, err := svc.RefreshToken(context.Background(), rawToken)
	if err != nil {
		t.Fatalf("RefreshToken: %v", err)
	}
	if tokens.AccessToken == "" || tokens.RefreshToken == "" {
		t.Error("tokens should not be empty")
	}
}

func TestRefreshToken_NotFound(t *testing.T) {
	mock := &testutil.MockQuerier{
		GetRefreshTokenFunc: func(ctx context.Context, hash string) (dbq.RefreshToken, error) {
			return dbq.RefreshToken{}, pgx.ErrNoRows
		},
	}

	svc := newTestService(mock, mock)
	_, err := svc.RefreshToken(context.Background(), "nonexistent")
	if !errors.Is(err, ErrInvalidRefreshToken) {
		t.Errorf("err = %v, want ErrInvalidRefreshToken", err)
	}
}

func TestRefreshToken_Expired(t *testing.T) {
	mock := &testutil.MockQuerier{
		GetRefreshTokenFunc: func(ctx context.Context, hash string) (dbq.RefreshToken, error) {
			return dbq.RefreshToken{
				UserID:    pgtype.Int8{Int64: 1, Valid: true},
				ExpiresAt: pgtype.Timestamptz{Time: time.Now().Add(-time.Hour), Valid: true},
			}, nil
		},
	}

	svc := newTestService(mock, mock)
	_, err := svc.RefreshToken(context.Background(), "expired-token")
	if !errors.Is(err, ErrInvalidRefreshToken) {
		t.Errorf("err = %v, want ErrInvalidRefreshToken", err)
	}
}
