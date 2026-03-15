package chat

import (
	"context"
	"errors"
	"testing"

	"github.com/apten-chat/messenger/internal/db/dbq"
	"github.com/apten-chat/messenger/internal/testutil"
	"github.com/jackc/pgx/v5"
)

func newTestService(queries dbq.Querier, txQueries dbq.Querier) *Service {
	tx := &testutil.MockTx{}
	return NewServiceWithDeps(
		queries,
		func(ctx context.Context) (pgx.Tx, error) { return tx, nil },
		func(t pgx.Tx) dbq.Querier { return txQueries },
	)
}

func TestCreate_GroupChat(t *testing.T) {
	var createdChat dbq.CreateChatParams
	var addedMembers []dbq.AddChatMemberParams

	txMock := &testutil.MockQuerier{
		CreateChatFunc: func(ctx context.Context, arg dbq.CreateChatParams) (dbq.Chat, error) {
			createdChat = arg
			return dbq.Chat{ID: 1, Type: arg.Type, Name: arg.Name}, nil
		},
		AddChatMemberFunc: func(ctx context.Context, arg dbq.AddChatMemberParams) error {
			addedMembers = append(addedMembers, arg)
			return nil
		},
	}

	svc := newTestService(&testutil.MockQuerier{}, txMock)
	chatResult, err := svc.Create(context.Background(), 10, CreateRequest{
		Type:      "group",
		Name:      strPtr("Test Group"),
		MemberIDs: []int64{20, 30},
	})
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	if chatResult.ID != 1 {
		t.Errorf("chat.ID = %d, want 1", chatResult.ID)
	}
	if createdChat.Type != "group" {
		t.Errorf("Type = %q, want group", createdChat.Type)
	}
	if !createdChat.Name.Valid || createdChat.Name.String != "Test Group" {
		t.Errorf("Name = %v, want 'Test Group'", createdChat.Name)
	}
	// Creator (10) as admin + members 20, 30.
	if len(addedMembers) != 3 {
		t.Fatalf("added %d members, want 3", len(addedMembers))
	}
	if addedMembers[0].Role.String != "admin" {
		t.Error("creator should be admin")
	}
	if addedMembers[1].Role.String != "member" || addedMembers[2].Role.String != "member" {
		t.Error("other members should have 'member' role")
	}
}

func TestCreate_DirectChat_AlreadyExists(t *testing.T) {
	mock := &testutil.MockQuerier{
		FindDirectChatFunc: func(ctx context.Context, arg dbq.FindDirectChatParams) (int64, error) {
			return 5, nil // Found existing chat.
		},
	}

	svc := newTestService(mock, &testutil.MockQuerier{})
	_, err := svc.Create(context.Background(), 10, CreateRequest{
		Type:      "direct",
		MemberIDs: []int64{20},
	})
	if !errors.Is(err, ErrDirectChatExists) {
		t.Errorf("err = %v, want ErrDirectChatExists", err)
	}
}

func TestCreate_DirectChat_New(t *testing.T) {
	mock := &testutil.MockQuerier{
		FindDirectChatFunc: func(ctx context.Context, arg dbq.FindDirectChatParams) (int64, error) {
			return 0, pgx.ErrNoRows // No existing chat.
		},
	}
	txMock := &testutil.MockQuerier{
		CreateChatFunc: func(ctx context.Context, arg dbq.CreateChatParams) (dbq.Chat, error) {
			return dbq.Chat{ID: 2, Type: "direct"}, nil
		},
		AddChatMemberFunc: func(ctx context.Context, arg dbq.AddChatMemberParams) error {
			return nil
		},
	}

	svc := newTestService(mock, txMock)
	chat, err := svc.Create(context.Background(), 10, CreateRequest{
		Type:      "direct",
		MemberIDs: []int64{20},
	})
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	if chat.ID != 2 {
		t.Errorf("chat.ID = %d, want 2", chat.ID)
	}
}

func TestCreate_SkipsDuplicateCreator(t *testing.T) {
	var addedMembers []dbq.AddChatMemberParams
	txMock := &testutil.MockQuerier{
		CreateChatFunc: func(ctx context.Context, arg dbq.CreateChatParams) (dbq.Chat, error) {
			return dbq.Chat{ID: 1}, nil
		},
		AddChatMemberFunc: func(ctx context.Context, arg dbq.AddChatMemberParams) error {
			addedMembers = append(addedMembers, arg)
			return nil
		},
	}

	svc := newTestService(&testutil.MockQuerier{}, txMock)
	_, err := svc.Create(context.Background(), 10, CreateRequest{
		Type:      "group",
		MemberIDs: []int64{10, 20}, // Creator is also in MemberIDs.
	})
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	// Should add creator once (as admin) + member 20. Creator (10) in MemberIDs should be skipped.
	if len(addedMembers) != 2 {
		t.Errorf("added %d members, want 2", len(addedMembers))
	}
}

func TestEnsureMember_OK(t *testing.T) {
	mock := &testutil.MockQuerier{
		IsChatMemberFunc: func(ctx context.Context, arg dbq.IsChatMemberParams) (bool, error) {
			return true, nil
		},
	}

	svc := newTestService(mock, mock)
	err := svc.EnsureMember(context.Background(), 1, 1)
	if err != nil {
		t.Errorf("EnsureMember: %v", err)
	}
}

func TestEnsureMember_NotMember(t *testing.T) {
	mock := &testutil.MockQuerier{
		IsChatMemberFunc: func(ctx context.Context, arg dbq.IsChatMemberParams) (bool, error) {
			return false, nil
		},
	}

	svc := newTestService(mock, mock)
	err := svc.EnsureMember(context.Background(), 1, 99)
	if !errors.Is(err, ErrNotMember) {
		t.Errorf("err = %v, want ErrNotMember", err)
	}
}

func TestGetMemberIDs(t *testing.T) {
	mock := &testutil.MockQuerier{
		ListChatMembersFunc: func(ctx context.Context, chatID int64) ([]dbq.ListChatMembersRow, error) {
			return []dbq.ListChatMembersRow{
				{ID: 1},
				{ID: 2},
				{ID: 3},
			}, nil
		},
	}

	svc := newTestService(mock, mock)
	ids, err := svc.GetMemberIDs(context.Background(), 1)
	if err != nil {
		t.Fatalf("GetMemberIDs: %v", err)
	}
	if len(ids) != 3 || ids[0] != 1 || ids[1] != 2 || ids[2] != 3 {
		t.Errorf("ids = %v, want [1 2 3]", ids)
	}
}

func strPtr(s string) *string { return &s }
