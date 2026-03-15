package invite

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"time"

	"github.com/apten-chat/messenger/internal/db/dbq"
	"github.com/jackc/pgx/v5/pgtype"
)

type Service struct {
	queries   *dbq.Queries
	inviteTTL time.Duration
}

func NewService(queries *dbq.Queries, inviteTTL time.Duration) *Service {
	return &Service{queries: queries, inviteTTL: inviteTTL}
}

func (s *Service) Create(ctx context.Context, createdBy int64) (dbq.Invite, error) {
	code, err := generateCode()
	if err != nil {
		return dbq.Invite{}, err
	}

	return s.queries.CreateInvite(ctx, dbq.CreateInviteParams{
		Code:      code,
		CreatedBy: pgtype.Int8{Int64: createdBy, Valid: true},
		ExpiresAt: pgtype.Timestamptz{Time: time.Now().Add(s.inviteTTL), Valid: true},
	})
}

func (s *Service) List(ctx context.Context) ([]dbq.Invite, error) {
	return s.queries.ListInvites(ctx)
}

func (s *Service) Delete(ctx context.Context, id int64) error {
	return s.queries.DeleteInvite(ctx, id)
}

// Bootstrap creates a seed invite when no users exist (created_by = NULL).
func Bootstrap(ctx context.Context, queries *dbq.Queries, ttl time.Duration) (string, error) {
	code, err := generateCode()
	if err != nil {
		return "", err
	}
	_, err = queries.CreateInvite(ctx, dbq.CreateInviteParams{
		Code:      code,
		ExpiresAt: pgtype.Timestamptz{Time: time.Now().Add(ttl), Valid: true},
	})
	return code, err
}

func generateCode() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}
