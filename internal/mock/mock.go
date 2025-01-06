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
