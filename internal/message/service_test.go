package message

import (
	"context"
	"errors"
	"testing"

	"github.com/apten-chat/messenger/internal/db/dbq"
	"github.com/apten-chat/messenger/internal/testutil"
	"github.com/jackc/pgx/v5/pgtype"
)

func TestSend_Success(t *testing.T) {
	mock := &testutil.MockQuerier{
		CreateMessageFunc: func(ctx context.Context, arg dbq.CreateMessageParams) (dbq.Message, error) {
			if arg.ChatID != 10 {
				t.Errorf("ChatID = %d, want 10", arg.ChatID)
			}
			if arg.SenderID != 1 {
				t.Errorf("SenderID = %d, want 1", arg.SenderID)
			}
			if arg.Content.String != "hello" {
				t.Errorf("Content = %q, want hello", arg.Content.String)
			}
			return dbq.Message{ID: 100, ChatID: 10, SenderID: 1, Content: arg.Content}, nil
		},
	}

	svc := NewService(mock)
	msg, err := svc.Send(context.Background(), 10, 1, "hello", nil)
	if err != nil {
		t.Fatalf("Send: %v", err)
	}
	if msg.ID != 100 {
		t.Errorf("msg.ID = %d, want 100", msg.ID)
	}
}

func TestSend_WithReply(t *testing.T) {
	var gotReplyID pgtype.Int8
	mock := &testutil.MockQuerier{
		CreateMessageFunc: func(ctx context.Context, arg dbq.CreateMessageParams) (dbq.Message, error) {
			gotReplyID = arg.ReplyToID
			return dbq.Message{ID: 101}, nil
		},
	}

	svc := NewService(mock)
	replyTo := int64(50)
	_, err := svc.Send(context.Background(), 10, 1, "reply", &replyTo)
	if err != nil {
		t.Fatalf("Send: %v", err)
	}
	if !gotReplyID.Valid || gotReplyID.Int64 != 50 {
		t.Errorf("ReplyToID = %v, want 50", gotReplyID)
	}
}

func TestSend_EmptyContent(t *testing.T) {
	mock := &testutil.MockQuerier{
		CreateMessageFunc: func(ctx context.Context, arg dbq.CreateMessageParams) (dbq.Message, error) {
			if arg.Content.Valid {
				t.Error("Content should not be valid for empty string")
			}
			return dbq.Message{ID: 102}, nil
		},
	}

	svc := NewService(mock)
	_, err := svc.Send(context.Background(), 10, 1, "", nil)
	if err != nil {
		t.Fatalf("Send: %v", err)
	}
}

func TestSend_DBError(t *testing.T) {
	dbErr := errors.New("db error")
	mock := &testutil.MockQuerier{
		CreateMessageFunc: func(ctx context.Context, arg dbq.CreateMessageParams) (dbq.Message, error) {
			return dbq.Message{}, dbErr
		},
	}

	svc := NewService(mock)
	_, err := svc.Send(context.Background(), 10, 1, "hello", nil)
	if !errors.Is(err, dbErr) {
		t.Errorf("err = %v, want %v", err, dbErr)
	}
}

func TestGetByID_Success(t *testing.T) {
	mock := &testutil.MockQuerier{
		GetMessageByIDFunc: func(ctx context.Context, id int64) (dbq.GetMessageByIDRow, error) {
			return dbq.GetMessageByIDRow{ID: id, ChatID: 10}, nil
		},
	}

	svc := NewService(mock)
	msg, err := svc.GetByID(context.Background(), 42)
	if err != nil {
		t.Fatalf("GetByID: %v", err)
	}
	if msg.ID != 42 {
		t.Errorf("ID = %d, want 42", msg.ID)
	}
}

func TestGetByID_NotFound(t *testing.T) {
	notFound := errors.New("not found")
	mock := &testutil.MockQuerier{
		GetMessageByIDFunc: func(ctx context.Context, id int64) (dbq.GetMessageByIDRow, error) {
			return dbq.GetMessageByIDRow{}, notFound
		},
	}

	svc := NewService(mock)
	_, err := svc.GetByID(context.Background(), 42)
	if !errors.Is(err, notFound) {
		t.Errorf("err = %v, want %v", err, notFound)
	}
}
