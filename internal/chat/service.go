package chat

import (
	"context"
	"errors"

	"github.com/apten-chat/messenger/internal/db/dbq"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"
)

var (
	ErrDirectChatExists = errors.New("direct chat already exists")
	ErrNotMember        = errors.New("not a chat member")
	ErrChatNotFound     = errors.New("chat not found")
)

type Service struct {
	queries dbq.Querier
	beginTx func(ctx context.Context) (pgx.Tx, error)
	withTx  func(tx pgx.Tx) dbq.Querier
}

func NewService(pool *pgxpool.Pool, queries *dbq.Queries) *Service {
	return &Service{
		queries: queries,
		beginTx: func(ctx context.Context) (pgx.Tx, error) { return pool.Begin(ctx) },
		withTx:  func(tx pgx.Tx) dbq.Querier { return queries.WithTx(tx) },
	}
}

// NewServiceWithDeps creates a Service with explicit dependencies (for testing).
func NewServiceWithDeps(queries dbq.Querier, beginTx func(ctx context.Context) (pgx.Tx, error), withTx func(tx pgx.Tx) dbq.Querier) *Service {
	return &Service{queries: queries, beginTx: beginTx, withTx: withTx}
}

type CreateRequest struct {
	Type      string  `json:"type"`
	Name      *string `json:"name,omitempty"`
	MemberIDs []int64 `json:"member_ids"`
}

func (s *Service) Create(ctx context.Context, creatorID int64, req CreateRequest) (dbq.Chat, error) {
	if req.Type == "direct" && len(req.MemberIDs) == 1 {
		otherID := req.MemberIDs[0]
		_, err := s.queries.FindDirectChat(ctx, dbq.FindDirectChatParams{
			UserID:   creatorID,
			UserID_2: otherID,
		})
		if err == nil {
			return dbq.Chat{}, ErrDirectChatExists
		}
		if !errors.Is(err, pgx.ErrNoRows) {
			return dbq.Chat{}, err
		}
	}

	tx, err := s.beginTx(ctx)
	if err != nil {
		return dbq.Chat{}, err
	}
	defer tx.Rollback(ctx)

	qtx := s.withTx(tx)

	var name pgtype.Text
	if req.Name != nil {
		name = pgtype.Text{String: *req.Name, Valid: true}
	}

	chat, err := qtx.CreateChat(ctx, dbq.CreateChatParams{
		Type:      req.Type,
		Name:      name,
		CreatedBy: pgtype.Int8{Int64: creatorID, Valid: true},
	})
	if err != nil {
		return dbq.Chat{}, err
	}

	// Add creator as admin.
	if err := qtx.AddChatMember(ctx, dbq.AddChatMemberParams{
		ChatID: chat.ID,
		UserID: creatorID,
		Role:   pgtype.Text{String: "admin", Valid: true},
	}); err != nil {
		return dbq.Chat{}, err
	}

	// Add other members.
	for _, uid := range req.MemberIDs {
		if uid == creatorID {
			continue
		}
		if err := qtx.AddChatMember(ctx, dbq.AddChatMemberParams{
			ChatID: chat.ID,
			UserID: uid,
			Role:   pgtype.Text{String: "member", Valid: true},
		}); err != nil {
			return dbq.Chat{}, err
		}
	}

	if err := tx.Commit(ctx); err != nil {
		return dbq.Chat{}, err
	}

	return chat, nil
}

func (s *Service) GetMemberIDs(ctx context.Context, chatID int64) ([]int64, error) {
	members, err := s.queries.ListChatMembers(ctx, chatID)
	if err != nil {
		return nil, err
	}
	ids := make([]int64, len(members))
	for i, m := range members {
		ids[i] = m.ID
	}
	return ids, nil
}

func (s *Service) EnsureMember(ctx context.Context, chatID, userID int64) error {
	ok, err := s.queries.IsChatMember(ctx, dbq.IsChatMemberParams{
		ChatID: chatID,
		UserID: userID,
	})
	if err != nil {
		return err
	}
	if !ok {
		return ErrNotMember
	}
	return nil
}
