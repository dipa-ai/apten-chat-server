package testutil

import (
	"context"

	"github.com/apten-chat/messenger/internal/db/dbq"
	"github.com/jackc/pgx/v5/pgtype"
)

// MockQuerier implements dbq.Querier with configurable function fields.
// Methods without a configured function return zero values.
type MockQuerier struct {
	AddChatMemberFunc              func(ctx context.Context, arg dbq.AddChatMemberParams) error
	CountUsersFunc                 func(ctx context.Context) (int64, error)
	CreateAttachmentFunc           func(ctx context.Context, arg dbq.CreateAttachmentParams) (dbq.Attachment, error)
	CreateChatFunc                 func(ctx context.Context, arg dbq.CreateChatParams) (dbq.Chat, error)
	CreateInviteFunc               func(ctx context.Context, arg dbq.CreateInviteParams) (dbq.Invite, error)
	CreateMessageFunc              func(ctx context.Context, arg dbq.CreateMessageParams) (dbq.Message, error)
	CreatePushSubscriptionFunc     func(ctx context.Context, arg dbq.CreatePushSubscriptionParams) (dbq.PushSubscription, error)
	CreateRefreshTokenFunc         func(ctx context.Context, arg dbq.CreateRefreshTokenParams) error
	CreateUserFunc                 func(ctx context.Context, arg dbq.CreateUserParams) (dbq.CreateUserRow, error)
	DeleteAttachmentsByMessageFunc func(ctx context.Context, messageID int64) error
	DeleteExpiredRefreshTokensFunc func(ctx context.Context) error
	DeleteInviteFunc               func(ctx context.Context, id int64) error
	DeletePushSubscriptionFunc     func(ctx context.Context, arg dbq.DeletePushSubscriptionParams) error
	DeletePushSubscriptionByEndpointFunc func(ctx context.Context, endpoint string) error
	DeleteRefreshTokenFunc         func(ctx context.Context, tokenHash string) error
	DeleteUserRefreshTokensFunc    func(ctx context.Context, userID pgtype.Int8) error
	FindDirectChatFunc             func(ctx context.Context, arg dbq.FindDirectChatParams) (int64, error)
	GetAttachmentAccessContextFunc func(ctx context.Context, id int64) (dbq.GetAttachmentAccessContextRow, error)
	GetAttachmentByIDFunc          func(ctx context.Context, id int64) (dbq.Attachment, error)
	GetChatByIDFunc                func(ctx context.Context, id int64) (dbq.Chat, error)
	GetInviteByCodeFunc            func(ctx context.Context, code string) (dbq.Invite, error)
	GetMessageByIDFunc             func(ctx context.Context, id int64) (dbq.GetMessageByIDRow, error)
	GetMessageReadFunc             func(ctx context.Context, arg dbq.GetMessageReadParams) (dbq.MessageRead, error)
	GetRefreshTokenFunc            func(ctx context.Context, tokenHash string) (dbq.RefreshToken, error)
	GetUserByIDFunc                func(ctx context.Context, id int64) (dbq.User, error)
	GetUserByUsernameFunc          func(ctx context.Context, username string) (dbq.User, error)
	IsChatMemberFunc               func(ctx context.Context, arg dbq.IsChatMemberParams) (bool, error)
	ListAttachmentsByMessageFunc   func(ctx context.Context, messageID int64) ([]dbq.Attachment, error)
	ListAttachmentsByMessageIDsFunc func(ctx context.Context, messageIDs []int64) ([]dbq.Attachment, error)
	ListChatMembersFunc            func(ctx context.Context, chatID int64) ([]dbq.ListChatMembersRow, error)
	ListChatsByUserFunc            func(ctx context.Context, userID int64) ([]dbq.ListChatsByUserRow, error)
	ListInvitesFunc                func(ctx context.Context) ([]dbq.Invite, error)
	ListMessageReadsByChatFunc     func(ctx context.Context, chatID int64) ([]dbq.MessageRead, error)
	ListMessagesByChatBeforeFunc   func(ctx context.Context, arg dbq.ListMessagesByChatBeforeParams) ([]dbq.ListMessagesByChatBeforeRow, error)
	ListMessagesByChatLatestFunc   func(ctx context.Context, arg dbq.ListMessagesByChatLatestParams) ([]dbq.ListMessagesByChatLatestRow, error)
	ListPushSubscriptionsByUserFunc func(ctx context.Context, userID int64) ([]dbq.PushSubscription, error)
	ListUsersFunc                  func(ctx context.Context) ([]dbq.ListUsersRow, error)
	MarkInviteUsedFunc             func(ctx context.Context, arg dbq.MarkInviteUsedParams) error
	RemoveChatMemberFunc           func(ctx context.Context, arg dbq.RemoveChatMemberParams) error
	SoftDeleteMessageFunc          func(ctx context.Context, id int64) error
	UpdateChatFunc                 func(ctx context.Context, arg dbq.UpdateChatParams) (dbq.Chat, error)
	UpdateLastSeenFunc             func(ctx context.Context, id int64) error
	UpdateMessageContentFunc       func(ctx context.Context, arg dbq.UpdateMessageContentParams) (dbq.Message, error)
	UpdateUserFunc                 func(ctx context.Context, arg dbq.UpdateUserParams) (dbq.UpdateUserRow, error)
	UpsertMessageReadFunc          func(ctx context.Context, arg dbq.UpsertMessageReadParams) error
}

var _ dbq.Querier = (*MockQuerier)(nil)

func (m *MockQuerier) AddChatMember(ctx context.Context, arg dbq.AddChatMemberParams) error {
	if m.AddChatMemberFunc != nil {
		return m.AddChatMemberFunc(ctx, arg)
	}
	return nil
}

func (m *MockQuerier) CountUsers(ctx context.Context) (int64, error) {
	if m.CountUsersFunc != nil {
		return m.CountUsersFunc(ctx)
	}
	return 0, nil
}

func (m *MockQuerier) CreateAttachment(ctx context.Context, arg dbq.CreateAttachmentParams) (dbq.Attachment, error) {
	if m.CreateAttachmentFunc != nil {
		return m.CreateAttachmentFunc(ctx, arg)
	}
	return dbq.Attachment{}, nil
}

func (m *MockQuerier) CreateChat(ctx context.Context, arg dbq.CreateChatParams) (dbq.Chat, error) {
	if m.CreateChatFunc != nil {
		return m.CreateChatFunc(ctx, arg)
	}
	return dbq.Chat{}, nil
}

func (m *MockQuerier) CreateInvite(ctx context.Context, arg dbq.CreateInviteParams) (dbq.Invite, error) {
	if m.CreateInviteFunc != nil {
		return m.CreateInviteFunc(ctx, arg)
	}
	return dbq.Invite{}, nil
}

func (m *MockQuerier) CreateMessage(ctx context.Context, arg dbq.CreateMessageParams) (dbq.Message, error) {
	if m.CreateMessageFunc != nil {
		return m.CreateMessageFunc(ctx, arg)
	}
	return dbq.Message{}, nil
}

func (m *MockQuerier) CreatePushSubscription(ctx context.Context, arg dbq.CreatePushSubscriptionParams) (dbq.PushSubscription, error) {
	if m.CreatePushSubscriptionFunc != nil {
		return m.CreatePushSubscriptionFunc(ctx, arg)
	}
	return dbq.PushSubscription{}, nil
}

func (m *MockQuerier) CreateRefreshToken(ctx context.Context, arg dbq.CreateRefreshTokenParams) error {
	if m.CreateRefreshTokenFunc != nil {
		return m.CreateRefreshTokenFunc(ctx, arg)
	}
	return nil
}

func (m *MockQuerier) CreateUser(ctx context.Context, arg dbq.CreateUserParams) (dbq.CreateUserRow, error) {
	if m.CreateUserFunc != nil {
		return m.CreateUserFunc(ctx, arg)
	}
	return dbq.CreateUserRow{}, nil
}

func (m *MockQuerier) DeleteAttachmentsByMessage(ctx context.Context, messageID int64) error {
	if m.DeleteAttachmentsByMessageFunc != nil {
		return m.DeleteAttachmentsByMessageFunc(ctx, messageID)
	}
	return nil
}

func (m *MockQuerier) DeleteExpiredRefreshTokens(ctx context.Context) error {
	if m.DeleteExpiredRefreshTokensFunc != nil {
		return m.DeleteExpiredRefreshTokensFunc(ctx)
	}
	return nil
}

func (m *MockQuerier) DeleteInvite(ctx context.Context, id int64) error {
	if m.DeleteInviteFunc != nil {
		return m.DeleteInviteFunc(ctx, id)
	}
	return nil
}

func (m *MockQuerier) DeletePushSubscription(ctx context.Context, arg dbq.DeletePushSubscriptionParams) error {
	if m.DeletePushSubscriptionFunc != nil {
		return m.DeletePushSubscriptionFunc(ctx, arg)
	}
	return nil
}

func (m *MockQuerier) DeletePushSubscriptionByEndpoint(ctx context.Context, endpoint string) error {
	if m.DeletePushSubscriptionByEndpointFunc != nil {
		return m.DeletePushSubscriptionByEndpointFunc(ctx, endpoint)
	}
	return nil
}

func (m *MockQuerier) DeleteRefreshToken(ctx context.Context, tokenHash string) error {
	if m.DeleteRefreshTokenFunc != nil {
		return m.DeleteRefreshTokenFunc(ctx, tokenHash)
	}
	return nil
}

func (m *MockQuerier) DeleteUserRefreshTokens(ctx context.Context, userID pgtype.Int8) error {
	if m.DeleteUserRefreshTokensFunc != nil {
		return m.DeleteUserRefreshTokensFunc(ctx, userID)
	}
	return nil
}

func (m *MockQuerier) FindDirectChat(ctx context.Context, arg dbq.FindDirectChatParams) (int64, error) {
	if m.FindDirectChatFunc != nil {
		return m.FindDirectChatFunc(ctx, arg)
	}
	return 0, nil
}

func (m *MockQuerier) GetAttachmentAccessContext(ctx context.Context, id int64) (dbq.GetAttachmentAccessContextRow, error) {
	if m.GetAttachmentAccessContextFunc != nil {
		return m.GetAttachmentAccessContextFunc(ctx, id)
	}
	return dbq.GetAttachmentAccessContextRow{}, nil
}

func (m *MockQuerier) GetAttachmentByID(ctx context.Context, id int64) (dbq.Attachment, error) {
	if m.GetAttachmentByIDFunc != nil {
		return m.GetAttachmentByIDFunc(ctx, id)
	}
	return dbq.Attachment{}, nil
}

func (m *MockQuerier) GetChatByID(ctx context.Context, id int64) (dbq.Chat, error) {
	if m.GetChatByIDFunc != nil {
		return m.GetChatByIDFunc(ctx, id)
	}
	return dbq.Chat{}, nil
}

func (m *MockQuerier) GetInviteByCode(ctx context.Context, code string) (dbq.Invite, error) {
	if m.GetInviteByCodeFunc != nil {
		return m.GetInviteByCodeFunc(ctx, code)
	}
	return dbq.Invite{}, nil
}

func (m *MockQuerier) GetMessageByID(ctx context.Context, id int64) (dbq.GetMessageByIDRow, error) {
	if m.GetMessageByIDFunc != nil {
		return m.GetMessageByIDFunc(ctx, id)
	}
	return dbq.GetMessageByIDRow{}, nil
}

func (m *MockQuerier) GetMessageRead(ctx context.Context, arg dbq.GetMessageReadParams) (dbq.MessageRead, error) {
	if m.GetMessageReadFunc != nil {
		return m.GetMessageReadFunc(ctx, arg)
	}
	return dbq.MessageRead{}, nil
}

func (m *MockQuerier) GetRefreshToken(ctx context.Context, tokenHash string) (dbq.RefreshToken, error) {
	if m.GetRefreshTokenFunc != nil {
		return m.GetRefreshTokenFunc(ctx, tokenHash)
	}
	return dbq.RefreshToken{}, nil
}

func (m *MockQuerier) GetUserByID(ctx context.Context, id int64) (dbq.User, error) {
	if m.GetUserByIDFunc != nil {
		return m.GetUserByIDFunc(ctx, id)
	}
	return dbq.User{}, nil
}

func (m *MockQuerier) GetUserByUsername(ctx context.Context, username string) (dbq.User, error) {
	if m.GetUserByUsernameFunc != nil {
		return m.GetUserByUsernameFunc(ctx, username)
	}
	return dbq.User{}, nil
}

func (m *MockQuerier) IsChatMember(ctx context.Context, arg dbq.IsChatMemberParams) (bool, error) {
	if m.IsChatMemberFunc != nil {
		return m.IsChatMemberFunc(ctx, arg)
	}
	return false, nil
}

func (m *MockQuerier) ListAttachmentsByMessage(ctx context.Context, messageID int64) ([]dbq.Attachment, error) {
	if m.ListAttachmentsByMessageFunc != nil {
		return m.ListAttachmentsByMessageFunc(ctx, messageID)
	}
	return []dbq.Attachment{}, nil
}

func (m *MockQuerier) ListAttachmentsByMessageIDs(ctx context.Context, messageIDs []int64) ([]dbq.Attachment, error) {
	if m.ListAttachmentsByMessageIDsFunc != nil {
		return m.ListAttachmentsByMessageIDsFunc(ctx, messageIDs)
	}
	return []dbq.Attachment{}, nil
}

func (m *MockQuerier) ListChatMembers(ctx context.Context, chatID int64) ([]dbq.ListChatMembersRow, error) {
	if m.ListChatMembersFunc != nil {
		return m.ListChatMembersFunc(ctx, chatID)
	}
	return []dbq.ListChatMembersRow{}, nil
}

func (m *MockQuerier) ListChatsByUser(ctx context.Context, userID int64) ([]dbq.ListChatsByUserRow, error) {
	if m.ListChatsByUserFunc != nil {
		return m.ListChatsByUserFunc(ctx, userID)
	}
	return []dbq.ListChatsByUserRow{}, nil
}

func (m *MockQuerier) ListInvites(ctx context.Context) ([]dbq.Invite, error) {
	if m.ListInvitesFunc != nil {
		return m.ListInvitesFunc(ctx)
	}
	return []dbq.Invite{}, nil
}

func (m *MockQuerier) ListMessageReadsByChat(ctx context.Context, chatID int64) ([]dbq.MessageRead, error) {
	if m.ListMessageReadsByChatFunc != nil {
		return m.ListMessageReadsByChatFunc(ctx, chatID)
	}
	return []dbq.MessageRead{}, nil
}

func (m *MockQuerier) ListMessagesByChatBefore(ctx context.Context, arg dbq.ListMessagesByChatBeforeParams) ([]dbq.ListMessagesByChatBeforeRow, error) {
	if m.ListMessagesByChatBeforeFunc != nil {
		return m.ListMessagesByChatBeforeFunc(ctx, arg)
	}
	return []dbq.ListMessagesByChatBeforeRow{}, nil
}

func (m *MockQuerier) ListMessagesByChatLatest(ctx context.Context, arg dbq.ListMessagesByChatLatestParams) ([]dbq.ListMessagesByChatLatestRow, error) {
	if m.ListMessagesByChatLatestFunc != nil {
		return m.ListMessagesByChatLatestFunc(ctx, arg)
	}
	return []dbq.ListMessagesByChatLatestRow{}, nil
}

func (m *MockQuerier) ListPushSubscriptionsByUser(ctx context.Context, userID int64) ([]dbq.PushSubscription, error) {
	if m.ListPushSubscriptionsByUserFunc != nil {
		return m.ListPushSubscriptionsByUserFunc(ctx, userID)
	}
	return []dbq.PushSubscription{}, nil
}

func (m *MockQuerier) ListUsers(ctx context.Context) ([]dbq.ListUsersRow, error) {
	if m.ListUsersFunc != nil {
		return m.ListUsersFunc(ctx)
	}
	return []dbq.ListUsersRow{}, nil
}

func (m *MockQuerier) MarkInviteUsed(ctx context.Context, arg dbq.MarkInviteUsedParams) error {
	if m.MarkInviteUsedFunc != nil {
		return m.MarkInviteUsedFunc(ctx, arg)
	}
	return nil
}

func (m *MockQuerier) RemoveChatMember(ctx context.Context, arg dbq.RemoveChatMemberParams) error {
	if m.RemoveChatMemberFunc != nil {
		return m.RemoveChatMemberFunc(ctx, arg)
	}
	return nil
}

func (m *MockQuerier) SoftDeleteMessage(ctx context.Context, id int64) error {
	if m.SoftDeleteMessageFunc != nil {
		return m.SoftDeleteMessageFunc(ctx, id)
	}
	return nil
}

func (m *MockQuerier) UpdateChat(ctx context.Context, arg dbq.UpdateChatParams) (dbq.Chat, error) {
	if m.UpdateChatFunc != nil {
		return m.UpdateChatFunc(ctx, arg)
	}
	return dbq.Chat{}, nil
}

func (m *MockQuerier) UpdateLastSeen(ctx context.Context, id int64) error {
	if m.UpdateLastSeenFunc != nil {
		return m.UpdateLastSeenFunc(ctx, id)
	}
	return nil
}

func (m *MockQuerier) UpdateMessageContent(ctx context.Context, arg dbq.UpdateMessageContentParams) (dbq.Message, error) {
	if m.UpdateMessageContentFunc != nil {
		return m.UpdateMessageContentFunc(ctx, arg)
	}
	return dbq.Message{}, nil
}

func (m *MockQuerier) UpdateUser(ctx context.Context, arg dbq.UpdateUserParams) (dbq.UpdateUserRow, error) {
	if m.UpdateUserFunc != nil {
		return m.UpdateUserFunc(ctx, arg)
	}
	return dbq.UpdateUserRow{}, nil
}

func (m *MockQuerier) UpsertMessageRead(ctx context.Context, arg dbq.UpsertMessageReadParams) error {
	if m.UpsertMessageReadFunc != nil {
		return m.UpsertMessageReadFunc(ctx, arg)
	}
	return nil
}
