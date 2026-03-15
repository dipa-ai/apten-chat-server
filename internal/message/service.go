package message

import (
	"context"

	"github.com/apten-chat/messenger/internal/db/dbq"
	"github.com/jackc/pgx/v5/pgtype"
)

type Service struct {
	queries *dbq.Queries
}

func NewService(queries *dbq.Queries) *Service {
	return &Service{queries: queries}
}

func (s *Service) Send(ctx context.Context, chatID, senderID int64, content string, replyToID *int64) (dbq.Message, error) {
	params := dbq.CreateMessageParams{
		ChatID:   chatID,
		SenderID: senderID,
		Content:  pgtype.Text{String: content, Valid: content != ""},
	}
	if replyToID != nil {
		params.ReplyToID = pgtype.Int8{Int64: *replyToID, Valid: true}
	}
	return s.queries.CreateMessage(ctx, params)
}

func (s *Service) GetByID(ctx context.Context, id int64) (dbq.GetMessageByIDRow, error) {
	return s.queries.GetMessageByID(ctx, id)
}
