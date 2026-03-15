package invite

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/apten-chat/messenger/internal/db/dbq"
	"github.com/apten-chat/messenger/internal/testutil"
)

func TestCreate_Success(t *testing.T) {
	var gotParams dbq.CreateInviteParams
	mock := &testutil.MockQuerier{
		CreateInviteFunc: func(ctx context.Context, arg dbq.CreateInviteParams) (dbq.Invite, error) {
			gotParams = arg
			return dbq.Invite{ID: 1, Code: arg.Code}, nil
		},
	}

	svc := NewService(mock, 7*24*time.Hour)
	inv, err := svc.Create(context.Background(), 42)
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	if inv.ID != 1 {
		t.Errorf("ID = %d, want 1", inv.ID)
	}
	if gotParams.CreatedBy.Int64 != 42 {
		t.Errorf("CreatedBy = %d, want 42", gotParams.CreatedBy.Int64)
	}
	if len(gotParams.Code) != 64 { // 32 bytes hex
		t.Errorf("code length = %d, want 64", len(gotParams.Code))
	}
	if !gotParams.ExpiresAt.Valid {
		t.Error("ExpiresAt should be valid")
	}
}

func TestCreate_DBError(t *testing.T) {
	dbErr := errors.New("db error")
	mock := &testutil.MockQuerier{
		CreateInviteFunc: func(ctx context.Context, arg dbq.CreateInviteParams) (dbq.Invite, error) {
			return dbq.Invite{}, dbErr
		},
	}

	svc := NewService(mock, time.Hour)
	_, err := svc.Create(context.Background(), 1)
	if !errors.Is(err, dbErr) {
		t.Errorf("err = %v, want %v", err, dbErr)
	}
}

func TestList_Success(t *testing.T) {
	invites := []dbq.Invite{{ID: 1}, {ID: 2}}
	mock := &testutil.MockQuerier{
		ListInvitesFunc: func(ctx context.Context) ([]dbq.Invite, error) {
			return invites, nil
		},
	}

	svc := NewService(mock, time.Hour)
	got, err := svc.List(context.Background())
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(got) != 2 {
		t.Errorf("len = %d, want 2", len(got))
	}
}

func TestDelete_Success(t *testing.T) {
	var deletedID int64
	mock := &testutil.MockQuerier{
		DeleteInviteFunc: func(ctx context.Context, id int64) error {
			deletedID = id
			return nil
		},
	}

	svc := NewService(mock, time.Hour)
	err := svc.Delete(context.Background(), 5)
	if err != nil {
		t.Fatalf("Delete: %v", err)
	}
	if deletedID != 5 {
		t.Errorf("deletedID = %d, want 5", deletedID)
	}
}

func TestBootstrap_Success(t *testing.T) {
	var gotParams dbq.CreateInviteParams
	mock := &testutil.MockQuerier{
		CreateInviteFunc: func(ctx context.Context, arg dbq.CreateInviteParams) (dbq.Invite, error) {
			gotParams = arg
			return dbq.Invite{Code: arg.Code}, nil
		},
	}

	code, err := Bootstrap(context.Background(), mock, 24*time.Hour)
	if err != nil {
		t.Fatalf("Bootstrap: %v", err)
	}
	if len(code) != 64 {
		t.Errorf("code length = %d, want 64", len(code))
	}
	// Bootstrap invites have no CreatedBy.
	if gotParams.CreatedBy.Valid {
		t.Error("CreatedBy should not be valid for bootstrap invite")
	}
}
