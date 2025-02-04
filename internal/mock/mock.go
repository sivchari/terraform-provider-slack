// Code generated by MockGen. DO NOT EDIT.
// Source: provider.go
//
// Generated by this command:
//
//	mockgen -source=provider.go -package=mock -destination=./mock/mock.go
//

// Package mock is a generated GoMock package.
package mock

import (
	context "context"
	reflect "reflect"

	slack "github.com/slack-go/slack"
	gomock "go.uber.org/mock/gomock"
)

// MockAPIClient is a mock of APIClient interface.
type MockAPIClient struct {
	ctrl     *gomock.Controller
	recorder *MockAPIClientMockRecorder
}

// MockAPIClientMockRecorder is the mock recorder for MockAPIClient.
type MockAPIClientMockRecorder struct {
	mock *MockAPIClient
}

// NewMockAPIClient creates a new mock instance.
func NewMockAPIClient(ctrl *gomock.Controller) *MockAPIClient {
	mock := &MockAPIClient{ctrl: ctrl}
	mock.recorder = &MockAPIClientMockRecorder{mock}
	return mock
}

// EXPECT returns an object that allows the caller to indicate expected use.
func (m *MockAPIClient) EXPECT() *MockAPIClientMockRecorder {
	return m.recorder
}

// ArchiveConversationContext mocks base method.
func (m *MockAPIClient) ArchiveConversationContext(ctx context.Context, channelID string) error {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "ArchiveConversationContext", ctx, channelID)
	ret0, _ := ret[0].(error)
	return ret0
}

// ArchiveConversationContext indicates an expected call of ArchiveConversationContext.
func (mr *MockAPIClientMockRecorder) ArchiveConversationContext(ctx, channelID any) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "ArchiveConversationContext", reflect.TypeOf((*MockAPIClient)(nil).ArchiveConversationContext), ctx, channelID)
}

// CloseConversationContext mocks base method.
func (m *MockAPIClient) CloseConversationContext(ctx context.Context, channelID string) (bool, bool, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "CloseConversationContext", ctx, channelID)
	ret0, _ := ret[0].(bool)
	ret1, _ := ret[1].(bool)
	ret2, _ := ret[2].(error)
	return ret0, ret1, ret2
}

// CloseConversationContext indicates an expected call of CloseConversationContext.
func (mr *MockAPIClientMockRecorder) CloseConversationContext(ctx, channelID any) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "CloseConversationContext", reflect.TypeOf((*MockAPIClient)(nil).CloseConversationContext), ctx, channelID)
}

// CreateConversationContext mocks base method.
func (m *MockAPIClient) CreateConversationContext(ctx context.Context, params slack.CreateConversationParams) (*slack.Channel, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "CreateConversationContext", ctx, params)
	ret0, _ := ret[0].(*slack.Channel)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// CreateConversationContext indicates an expected call of CreateConversationContext.
func (mr *MockAPIClientMockRecorder) CreateConversationContext(ctx, params any) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "CreateConversationContext", reflect.TypeOf((*MockAPIClient)(nil).CreateConversationContext), ctx, params)
}

// CreateUserGroupContext mocks base method.
func (m *MockAPIClient) CreateUserGroupContext(ctx context.Context, userGroup slack.UserGroup) (slack.UserGroup, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "CreateUserGroupContext", ctx, userGroup)
	ret0, _ := ret[0].(slack.UserGroup)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// CreateUserGroupContext indicates an expected call of CreateUserGroupContext.
func (mr *MockAPIClientMockRecorder) CreateUserGroupContext(ctx, userGroup any) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "CreateUserGroupContext", reflect.TypeOf((*MockAPIClient)(nil).CreateUserGroupContext), ctx, userGroup)
}

// DisableUserGroupContext mocks base method.
func (m *MockAPIClient) DisableUserGroupContext(ctx context.Context, userGroup string) (slack.UserGroup, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "DisableUserGroupContext", ctx, userGroup)
	ret0, _ := ret[0].(slack.UserGroup)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// DisableUserGroupContext indicates an expected call of DisableUserGroupContext.
func (mr *MockAPIClientMockRecorder) DisableUserGroupContext(ctx, userGroup any) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "DisableUserGroupContext", reflect.TypeOf((*MockAPIClient)(nil).DisableUserGroupContext), ctx, userGroup)
}

// EnableUserGroupContext mocks base method.
func (m *MockAPIClient) EnableUserGroupContext(ctx context.Context, userGroup string) (slack.UserGroup, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "EnableUserGroupContext", ctx, userGroup)
	ret0, _ := ret[0].(slack.UserGroup)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// EnableUserGroupContext indicates an expected call of EnableUserGroupContext.
func (mr *MockAPIClientMockRecorder) EnableUserGroupContext(ctx, userGroup any) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "EnableUserGroupContext", reflect.TypeOf((*MockAPIClient)(nil).EnableUserGroupContext), ctx, userGroup)
}

// GetConversationInfoContext mocks base method.
func (m *MockAPIClient) GetConversationInfoContext(ctx context.Context, input *slack.GetConversationInfoInput) (*slack.Channel, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "GetConversationInfoContext", ctx, input)
	ret0, _ := ret[0].(*slack.Channel)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// GetConversationInfoContext indicates an expected call of GetConversationInfoContext.
func (mr *MockAPIClientMockRecorder) GetConversationInfoContext(ctx, input any) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "GetConversationInfoContext", reflect.TypeOf((*MockAPIClient)(nil).GetConversationInfoContext), ctx, input)
}

// GetUserByEmailContext mocks base method.
func (m *MockAPIClient) GetUserByEmailContext(ctx context.Context, email string) (*slack.User, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "GetUserByEmailContext", ctx, email)
	ret0, _ := ret[0].(*slack.User)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// GetUserByEmailContext indicates an expected call of GetUserByEmailContext.
func (mr *MockAPIClientMockRecorder) GetUserByEmailContext(ctx, email any) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "GetUserByEmailContext", reflect.TypeOf((*MockAPIClient)(nil).GetUserByEmailContext), ctx, email)
}

// GetUserGroupsContext mocks base method.
func (m *MockAPIClient) GetUserGroupsContext(ctx context.Context, opts ...slack.GetUserGroupsOption) ([]slack.UserGroup, error) {
	m.ctrl.T.Helper()
	varargs := []any{ctx}
	for _, a := range opts {
		varargs = append(varargs, a)
	}
	ret := m.ctrl.Call(m, "GetUserGroupsContext", varargs...)
	ret0, _ := ret[0].([]slack.UserGroup)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// GetUserGroupsContext indicates an expected call of GetUserGroupsContext.
func (mr *MockAPIClientMockRecorder) GetUserGroupsContext(ctx any, opts ...any) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	varargs := append([]any{ctx}, opts...)
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "GetUserGroupsContext", reflect.TypeOf((*MockAPIClient)(nil).GetUserGroupsContext), varargs...)
}

// GetUsersInConversationContext mocks base method.
func (m *MockAPIClient) GetUsersInConversationContext(ctx context.Context, params *slack.GetUsersInConversationParameters) ([]string, string, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "GetUsersInConversationContext", ctx, params)
	ret0, _ := ret[0].([]string)
	ret1, _ := ret[1].(string)
	ret2, _ := ret[2].(error)
	return ret0, ret1, ret2
}

// GetUsersInConversationContext indicates an expected call of GetUsersInConversationContext.
func (mr *MockAPIClientMockRecorder) GetUsersInConversationContext(ctx, params any) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "GetUsersInConversationContext", reflect.TypeOf((*MockAPIClient)(nil).GetUsersInConversationContext), ctx, params)
}

// InviteUsersToConversationContext mocks base method.
func (m *MockAPIClient) InviteUsersToConversationContext(ctx context.Context, channelID string, users ...string) (*slack.Channel, error) {
	m.ctrl.T.Helper()
	varargs := []any{ctx, channelID}
	for _, a := range users {
		varargs = append(varargs, a)
	}
	ret := m.ctrl.Call(m, "InviteUsersToConversationContext", varargs...)
	ret0, _ := ret[0].(*slack.Channel)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// InviteUsersToConversationContext indicates an expected call of InviteUsersToConversationContext.
func (mr *MockAPIClientMockRecorder) InviteUsersToConversationContext(ctx, channelID any, users ...any) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	varargs := append([]any{ctx, channelID}, users...)
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "InviteUsersToConversationContext", reflect.TypeOf((*MockAPIClient)(nil).InviteUsersToConversationContext), varargs...)
}

// KickUserFromConversationContext mocks base method.
func (m *MockAPIClient) KickUserFromConversationContext(ctx context.Context, channelID, user string) error {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "KickUserFromConversationContext", ctx, channelID, user)
	ret0, _ := ret[0].(error)
	return ret0
}

// KickUserFromConversationContext indicates an expected call of KickUserFromConversationContext.
func (mr *MockAPIClientMockRecorder) KickUserFromConversationContext(ctx, channelID, user any) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "KickUserFromConversationContext", reflect.TypeOf((*MockAPIClient)(nil).KickUserFromConversationContext), ctx, channelID, user)
}

// SetPurposeOfConversationContext mocks base method.
func (m *MockAPIClient) SetPurposeOfConversationContext(ctx context.Context, channelID, purpose string) (*slack.Channel, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "SetPurposeOfConversationContext", ctx, channelID, purpose)
	ret0, _ := ret[0].(*slack.Channel)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// SetPurposeOfConversationContext indicates an expected call of SetPurposeOfConversationContext.
func (mr *MockAPIClientMockRecorder) SetPurposeOfConversationContext(ctx, channelID, purpose any) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "SetPurposeOfConversationContext", reflect.TypeOf((*MockAPIClient)(nil).SetPurposeOfConversationContext), ctx, channelID, purpose)
}

// SetTopicOfConversationContext mocks base method.
func (m *MockAPIClient) SetTopicOfConversationContext(ctx context.Context, channelID, topic string) (*slack.Channel, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "SetTopicOfConversationContext", ctx, channelID, topic)
	ret0, _ := ret[0].(*slack.Channel)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// SetTopicOfConversationContext indicates an expected call of SetTopicOfConversationContext.
func (mr *MockAPIClientMockRecorder) SetTopicOfConversationContext(ctx, channelID, topic any) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "SetTopicOfConversationContext", reflect.TypeOf((*MockAPIClient)(nil).SetTopicOfConversationContext), ctx, channelID, topic)
}

// UpdateUserGroupContext mocks base method.
func (m *MockAPIClient) UpdateUserGroupContext(ctx context.Context, userGroupID string, opts ...slack.UpdateUserGroupsOption) (slack.UserGroup, error) {
	m.ctrl.T.Helper()
	varargs := []any{ctx, userGroupID}
	for _, a := range opts {
		varargs = append(varargs, a)
	}
	ret := m.ctrl.Call(m, "UpdateUserGroupContext", varargs...)
	ret0, _ := ret[0].(slack.UserGroup)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// UpdateUserGroupContext indicates an expected call of UpdateUserGroupContext.
func (mr *MockAPIClientMockRecorder) UpdateUserGroupContext(ctx, userGroupID any, opts ...any) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	varargs := append([]any{ctx, userGroupID}, opts...)
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "UpdateUserGroupContext", reflect.TypeOf((*MockAPIClient)(nil).UpdateUserGroupContext), varargs...)
}

// UpdateUserGroupMembersContext mocks base method.
func (m *MockAPIClient) UpdateUserGroupMembersContext(ctx context.Context, userGroup, members string) (slack.UserGroup, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "UpdateUserGroupMembersContext", ctx, userGroup, members)
	ret0, _ := ret[0].(slack.UserGroup)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// UpdateUserGroupMembersContext indicates an expected call of UpdateUserGroupMembersContext.
func (mr *MockAPIClientMockRecorder) UpdateUserGroupMembersContext(ctx, userGroup, members any) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "UpdateUserGroupMembersContext", reflect.TypeOf((*MockAPIClient)(nil).UpdateUserGroupMembersContext), ctx, userGroup, members)
}
