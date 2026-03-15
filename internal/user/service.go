package user

import (
	"context"
	"errors"
	"time"

	"github.com/apten-chat/messenger/internal/auth"
	"github.com/apten-chat/messenger/internal/db/dbq"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"
)

var (
	ErrInvalidCredentials  = errors.New("invalid credentials")
	ErrInviteInvalid       = errors.New("invite code is invalid, used, or expired")
	ErrUsernameTaken       = errors.New("username already taken")
	ErrInvalidRefreshToken = errors.New("invalid or expired refresh token")
)

type Service struct {
	pool       *pgxpool.Pool
	queries    *dbq.Queries
	jwtSecret  string
	accessTTL  time.Duration
	refreshTTL time.Duration
}

type TokenPair struct {
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token"`
}

func NewService(pool *pgxpool.Pool, queries *dbq.Queries, jwtSecret string, accessTTL, refreshTTL time.Duration) *Service {
	return &Service{
		pool:       pool,
		queries:    queries,
		jwtSecret:  jwtSecret,
		accessTTL:  accessTTL,
		refreshTTL: refreshTTL,
	}
}

func (s *Service) Register(ctx context.Context, code, username, displayName, password string) (*TokenPair, error) {
	invite, err := s.queries.GetInviteByCode(ctx, code)
	if err != nil {
		return nil, ErrInviteInvalid
	}
	if invite.UsedBy.Valid || invite.ExpiresAt.Time.Before(time.Now()) {
		return nil, ErrInviteInvalid
	}

	hash, err := auth.HashPassword(password)
	if err != nil {
		return nil, err
	}

	count, err := s.queries.CountUsers(ctx)
	if err != nil {
		return nil, err
	}
	isAdmin := count == 0

	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback(ctx)

	qtx := s.queries.WithTx(tx)

	user, err := qtx.CreateUser(ctx, dbq.CreateUserParams{
		Username:     username,
		DisplayName:  displayName,
		PasswordHash: hash,
		IsAdmin:      pgtype.Bool{Bool: isAdmin, Valid: true},
	})
	if err != nil {
		return nil, ErrUsernameTaken
	}

	if err := qtx.MarkInviteUsed(ctx, dbq.MarkInviteUsedParams{
		UsedBy: pgtype.Int8{Int64: user.ID, Valid: true},
		ID:     invite.ID,
	}); err != nil {
		return nil, err
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, err
	}

	return s.generateTokens(ctx, user.ID, user.IsAdmin.Bool)
}

func (s *Service) Login(ctx context.Context, username, password string) (*TokenPair, error) {
	user, err := s.queries.GetUserByUsername(ctx, username)
	if err != nil {
		return nil, ErrInvalidCredentials
	}
	if !auth.CheckPassword(user.PasswordHash, password) {
		return nil, ErrInvalidCredentials
	}
	return s.generateTokens(ctx, user.ID, user.IsAdmin.Bool)
}

func (s *Service) RefreshToken(ctx context.Context, rawToken string) (*TokenPair, error) {
	hash := auth.HashToken(rawToken)

	stored, err := s.queries.GetRefreshToken(ctx, hash)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrInvalidRefreshToken
		}
		return nil, err
	}
	if stored.ExpiresAt.Time.Before(time.Now()) {
		return nil, ErrInvalidRefreshToken
	}

	if err := s.queries.DeleteRefreshToken(ctx, hash); err != nil {
		return nil, err
	}

	user, err := s.queries.GetUserByID(ctx, stored.UserID.Int64)
	if err != nil {
		return nil, err
	}

	return s.generateTokens(ctx, user.ID, user.IsAdmin.Bool)
}

func (s *Service) generateTokens(ctx context.Context, userID int64, isAdmin bool) (*TokenPair, error) {
	accessToken, err := auth.GenerateAccessToken(s.jwtSecret, userID, isAdmin, s.accessTTL)
	if err != nil {
		return nil, err
	}

	rawRefresh, hashRefresh, err := auth.GenerateRefreshToken()
	if err != nil {
		return nil, err
	}

	if err := s.queries.CreateRefreshToken(ctx, dbq.CreateRefreshTokenParams{
		UserID:    pgtype.Int8{Int64: userID, Valid: true},
		TokenHash: hashRefresh,
		ExpiresAt: pgtype.Timestamptz{Time: time.Now().Add(s.refreshTTL), Valid: true},
	}); err != nil {
		return nil, err
	}

	return &TokenPair{
		AccessToken:  accessToken,
		RefreshToken: rawRefresh,
	}, nil
}
