package internal

import (
	"context"
	"errors"
	"fmt"
	"reflect"
	"strings"
	"testing"

	"github.com/slack-go/slack"
	"go.uber.org/mock/gomock"

	"github.com/sivchari/terraform-provider-slack/internal/mock"
)

func TestAllConversationMembers_Pages(t *testing.T) {
	t.Parallel()

	ctrl := gomock.NewController(t)
	client := mock.NewMockAPIClient(ctrl)
	gomock.InOrder(
		client.EXPECT().GetUsersInConversationContext(gomock.Any(), &slack.GetUsersInConversationParameters{
			ChannelID: "C1",
			Limit:     conversationPageSize,
		}).Return([]string{"U1", "U2"}, "page2", nil),
		client.EXPECT().GetUsersInConversationContext(gomock.Any(), &slack.GetUsersInConversationParameters{
			ChannelID: "C1",
			Cursor:    "page2",
			Limit:     conversationPageSize,
		}).Return([]string{"U3"}, "", nil),
	)

	got, err := allConversationMembers(context.Background(), client, "C1")
	if err != nil {
		t.Errorf("allConversationMembers() error = %v", err)
		return
	}
	if want := []string{"U1", "U2", "U3"}; !reflect.DeepEqual(got, want) {
		t.Errorf("allConversationMembers() = %v, want %v", got, want)
	}
}

func TestAllConversationMembers_Error(t *testing.T) {
	t.Parallel()

	ctrl := gomock.NewController(t)
	client := mock.NewMockAPIClient(ctrl)
	client.EXPECT().GetUsersInConversationContext(gomock.Any(), gomock.Any()).Return(nil, "", errors.New("channel_not_found"))

	got, err := allConversationMembers(context.Background(), client, "C1")
	if err == nil {
		t.Errorf("allConversationMembers() = %v, want an error", got)
		return
	}
	if !isSlackError(err, "channel_not_found") {
		t.Errorf("allConversationMembers() error = %v, want it to keep the Slack error code", err)
	}
}

func TestFindConversationByName_Pages(t *testing.T) {
	t.Parallel()

	ctrl := gomock.NewController(t)
	client := mock.NewMockAPIClient(ctrl)
	wantTypes := []string{"public_channel", "private_channel"}
	gomock.InOrder(
		client.EXPECT().GetConversationsContext(gomock.Any(), &slack.GetConversationsParameters{
			Types: wantTypes,
			Limit: conversationPageSize,
		}).Return([]slack.Channel{testChannel("C1", "general")}, "page2", nil),
		client.EXPECT().GetConversationsContext(gomock.Any(), &slack.GetConversationsParameters{
			Cursor: "page2",
			Types:  wantTypes,
			Limit:  conversationPageSize,
		}).Return([]slack.Channel{testChannel("C2", "random"), testChannel("C3", "target")}, "", nil),
	)

	got, err := findConversationByName(context.Background(), client, "target")
	if err != nil {
		t.Errorf("findConversationByName() error = %v", err)
		return
	}
	if got == nil || got.ID != "C3" {
		t.Errorf("findConversationByName() = %v, want channel C3", got)
	}
}

func TestFindConversationByName_NotFound(t *testing.T) {
	t.Parallel()

	ctrl := gomock.NewController(t)
	client := mock.NewMockAPIClient(ctrl)
	client.EXPECT().GetConversationsContext(gomock.Any(), gomock.Any()).
		Return([]slack.Channel{testChannel("C1", "general-2")}, "", nil)

	got, err := findConversationByName(context.Background(), client, "general")
	if err != nil {
		t.Errorf("findConversationByName() error = %v", err)
		return
	}
	if got != nil {
		t.Errorf("findConversationByName() = %v, want nil for a name that is only a prefix match", got)
	}
}

func TestFindConversationByName_Error(t *testing.T) {
	t.Parallel()

	ctrl := gomock.NewController(t)
	client := mock.NewMockAPIClient(ctrl)
	client.EXPECT().GetConversationsContext(gomock.Any(), gomock.Any()).Return(nil, "", errors.New("missing_scope"))

	if got, err := findConversationByName(context.Background(), client, "general"); err == nil {
		t.Errorf("findConversationByName() = %v, want an error", got)
	}
}

func TestIsSlackError(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		err  error
		code string
		want bool
	}{
		{name: "nil", err: nil, code: "name_taken", want: false},
		{name: "SlackErrorResponse value", err: slack.SlackErrorResponse{Err: "name_taken"}, code: "name_taken", want: true},
		{name: "SlackErrorResponse pointer", err: &slack.SlackErrorResponse{Err: "name_taken"}, code: "name_taken", want: true},
		{name: "wrapped SlackErrorResponse", err: fmt.Errorf("wrapped: %w", slack.SlackErrorResponse{Err: "name_taken"}), code: "name_taken", want: true},
		{name: "plain error with the code", err: errors.New("name_taken"), code: "name_taken", want: true},
		{name: "different code", err: slack.SlackErrorResponse{Err: "already_archived"}, code: "name_taken", want: false},
		{name: "plain error with a different message", err: errors.New("boom"), code: "name_taken", want: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			if got := isSlackError(tt.err, tt.code); got != tt.want {
				t.Errorf("isSlackError(%v, %q) = %v, want %v", tt.err, tt.code, got, tt.want)
			}
		})
	}
}

func TestMergeMemberOrder(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		prior  []string
		remote []string
		want   []string
	}{
		{name: "same order", prior: []string{"U1", "U2"}, remote: []string{"U1", "U2"}, want: []string{"U1", "U2"}},
		{name: "reordered remote keeps prior order", prior: []string{"U1", "U2", "U3"}, remote: []string{"U3", "U1", "U2"}, want: []string{"U1", "U2", "U3"}},
		{name: "removed member is dropped", prior: []string{"U1", "U2"}, remote: []string{"U2"}, want: []string{"U2"}},
		{name: "new member is appended in remote order", prior: []string{"U1"}, remote: []string{"U9", "U1", "U8"}, want: []string{"U1", "U9", "U8"}},
		{name: "bot is not appended", prior: []string{"U1"}, remote: []string{"UBOT", "U1"}, want: []string{"U1"}},
		{name: "bot already declared stays", prior: []string{"U1", "UBOT"}, remote: []string{"UBOT", "U1"}, want: []string{"U1", "UBOT"}},
		{name: "empty prior", prior: nil, remote: []string{"UBOT", "U1"}, want: []string{"U1"}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got := mergeMemberOrder(tt.prior, tt.remote, "UBOT")
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("mergeMemberOrder(%v, %v) = %v, want %v", tt.prior, tt.remote, got, tt.want)
			}
		})
	}
}

func TestDiffMembers(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		desired    []string
		current    []string
		wantInvite []string
		wantKick   []string
	}{
		{name: "no change", desired: []string{"U1"}, current: []string{"U1"}},
		{name: "invite and kick", desired: []string{"U1", "U2"}, current: []string{"U1", "U3"}, wantInvite: []string{"U2"}, wantKick: []string{"U3"}},
		{name: "bot is never invited", desired: []string{"UBOT", "U1"}, current: []string{"U1"}},
		{name: "bot is never kicked", desired: []string{"U1"}, current: []string{"UBOT", "U1"}},
		{name: "duplicates in desired are invited once", desired: []string{"U1", "U1"}, current: nil, wantInvite: []string{"U1"}},
		{name: "invite keeps desired order", desired: []string{"U3", "U1", "U2"}, current: nil, wantInvite: []string{"U3", "U1", "U2"}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			invite, kick := diffMembers(tt.desired, tt.current, "UBOT")
			if !reflect.DeepEqual(invite, tt.wantInvite) {
				t.Errorf("invite = %v, want %v", invite, tt.wantInvite)
			}
			if !reflect.DeepEqual(kick, tt.wantKick) {
				t.Errorf("kick = %v, want %v", kick, tt.wantKick)
			}
		})
	}
}

func TestInviteMembers_Batches(t *testing.T) {
	t.Parallel()

	users := make([]string, inviteBatchSize+1)
	for i := range users {
		users[i] = fmt.Sprintf("U%d", i)
	}

	ctrl := gomock.NewController(t)
	client := mock.NewMockAPIClient(ctrl)
	gomock.InOrder(
		client.EXPECT().InviteUsersToConversationContext(gomock.Any(), "C1", strings.Join(users[:inviteBatchSize], ",")).Return(nil, nil),
		client.EXPECT().InviteUsersToConversationContext(gomock.Any(), "C1", users[inviteBatchSize]).Return(nil, nil),
	)

	if err := inviteMembers(context.Background(), client, "C1", users); err != nil {
		t.Errorf("inviteMembers() error = %v", err)
	}
}

func testChannel(id, name string) slack.Channel {
	return slack.Channel{
		GroupConversation: slack.GroupConversation{
			Conversation: slack.Conversation{ID: id},
			Name:         name,
		},
	}
}
