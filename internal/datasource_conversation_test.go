package internal

import (
	"errors"
	"regexp"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/sivchari/terraform-provider-slack/internal/mock"
	"github.com/slack-go/slack"
	"go.uber.org/mock/gomock"
)

func testSlackChannel(id, name string) *slack.Channel {
	return &slack.Channel{
		GroupConversation: slack.GroupConversation{
			Conversation: slack.Conversation{
				ID:               id,
				IsPrivate:        true,
				User:             "U9",
				ConnectedTeamIDs: []string{"T1"},
				SharedTeamIDs:    []string{"T2"},
				InternalTeamIDs:  []string{"T3"},
			},
			Name:       name,
			Creator:    "U1",
			IsArchived: true,
			Topic: slack.Topic{
				Value:   "topic",
				Creator: "U2",
			},
			Purpose: slack.Purpose{
				Value:   "purpose",
				Creator: "U3",
			},
		},
	}
}

func testDataSourceConversationChecks() resource.TestCheckFunc {
	return resource.ComposeAggregateTestCheckFunc(
		resource.TestCheckResourceAttr("data.slack_conversation.test", "id", "C2"),
		resource.TestCheckResourceAttr("data.slack_conversation.test", "name", "general"),
		resource.TestCheckResourceAttr("data.slack_conversation.test", "creator", "U1"),
		resource.TestCheckResourceAttr("data.slack_conversation.test", "is_archived", "true"),
		resource.TestCheckResourceAttr("data.slack_conversation.test", "members.#", "3"),
		resource.TestCheckResourceAttr("data.slack_conversation.test", "members.0", "U1"),
		resource.TestCheckResourceAttr("data.slack_conversation.test", "members.1", "U2"),
		resource.TestCheckResourceAttr("data.slack_conversation.test", "members.2", "U3"),
		resource.TestCheckResourceAttr("data.slack_conversation.test", "topic.value", "topic"),
		resource.TestCheckResourceAttr("data.slack_conversation.test", "topic.creator", "U2"),
		resource.TestCheckResourceAttr("data.slack_conversation.test", "purpose.value", "purpose"),
		resource.TestCheckResourceAttr("data.slack_conversation.test", "purpose.creator", "U3"),
		resource.TestCheckResourceAttr("data.slack_conversation.test", "is_private", "true"),
		resource.TestCheckResourceAttr("data.slack_conversation.test", "user", "U9"),
		resource.TestCheckResourceAttr("data.slack_conversation.test", "connected_team_ids.0", "T1"),
		resource.TestCheckResourceAttr("data.slack_conversation.test", "shared_team_ids.0", "T2"),
		resource.TestCheckResourceAttr("data.slack_conversation.test", "internal_team_ids.0", "T3"),
	)
}

// expectConversationInfo expects conversations.info and the two pages of
// conversations.members for channel C2.
func expectConversationInfo(client *mock.MockAPIClient) {
	client.EXPECT().GetConversationInfoContext(gomock.Any(), &slack.GetConversationInfoInput{
		ChannelID: "C2",
	}).Return(testSlackChannel("C2", "general"), nil).AnyTimes()
	client.EXPECT().GetUsersInConversationContext(gomock.Any(), &slack.GetUsersInConversationParameters{
		ChannelID: "C2",
		Limit:     conversationPageSize,
	}).Return([]string{"U1", "U2"}, "members-page-2", nil).AnyTimes()
	client.EXPECT().GetUsersInConversationContext(gomock.Any(), &slack.GetUsersInConversationParameters{
		ChannelID: "C2",
		Cursor:    "members-page-2",
		Limit:     conversationPageSize,
	}).Return([]string{"U3"}, "", nil).AnyTimes()
}

func conversationListParams(cursor string) *slack.GetConversationsParameters {
	return &slack.GetConversationsParameters{
		Cursor: cursor,
		Types:  []string{"public_channel", "private_channel"},
		Limit:  conversationPageSize,
	}
}

func TestAccDataSourceConversation_ID(t *testing.T) {
	t.Parallel()

	ctrl := gomock.NewController(t)
	client := mock.NewMockAPIClient(ctrl)
	client.EXPECT().HasBotToken().Return(true).AnyTimes()
	expectConversationInfo(client)

	resource.UnitTest(t, resource.TestCase{
		ProtoV6ProviderFactories: protoV6ProviderFactories(client),
		Steps: []resource.TestStep{
			{
				Config: testAccDataSourceConversation(`id = "C2"`),
				Check:  testDataSourceConversationChecks(),
			},
		},
	})
}

func TestAccDataSourceConversation_Name(t *testing.T) {
	t.Parallel()

	ctrl := gomock.NewController(t)
	client := mock.NewMockAPIClient(ctrl)
	client.EXPECT().HasBotToken().Return(true).AnyTimes()
	client.EXPECT().GetConversationsContext(gomock.Any(), conversationListParams("")).
		Return([]slack.Channel{*testSlackChannel("C1", "random")}, "list-page-2", nil).AnyTimes()
	client.EXPECT().GetConversationsContext(gomock.Any(), conversationListParams("list-page-2")).
		Return([]slack.Channel{*testSlackChannel("C2", "general")}, "", nil).AnyTimes()
	expectConversationInfo(client)

	resource.UnitTest(t, resource.TestCase{
		ProtoV6ProviderFactories: protoV6ProviderFactories(client),
		Steps: []resource.TestStep{
			{
				Config: testAccDataSourceConversation(`name = "general"`),
				Check:  testDataSourceConversationChecks(),
			},
		},
	})
}

func TestAccDataSourceConversation_NameNotFound(t *testing.T) {
	t.Parallel()

	ctrl := gomock.NewController(t)
	client := mock.NewMockAPIClient(ctrl)
	client.EXPECT().HasBotToken().Return(true).AnyTimes()
	client.EXPECT().GetConversationsContext(gomock.Any(), conversationListParams("")).
		Return([]slack.Channel{*testSlackChannel("C1", "random")}, "list-page-2", nil).AnyTimes()
	client.EXPECT().GetConversationsContext(gomock.Any(), conversationListParams("list-page-2")).
		Return([]slack.Channel{*testSlackChannel("C3", "dev")}, "", nil).AnyTimes()

	resource.UnitTest(t, resource.TestCase{
		ProtoV6ProviderFactories: protoV6ProviderFactories(client),
		Steps: []resource.TestStep{
			{
				Config:      testAccDataSourceConversation(`name = "general"`),
				ExpectError: regexp.MustCompile(`(?s)No\s+public\s+or\s+private\s+channel\s+is\s+named\s+"general"`),
			},
		},
	})
}

func TestAccDataSourceConversation_NameListError(t *testing.T) {
	t.Parallel()

	ctrl := gomock.NewController(t)
	client := mock.NewMockAPIClient(ctrl)
	client.EXPECT().HasBotToken().Return(true).AnyTimes()
	client.EXPECT().GetConversationsContext(gomock.Any(), conversationListParams("")).
		Return(nil, "", errors.New("missing_scope")).AnyTimes()

	resource.UnitTest(t, resource.TestCase{
		ProtoV6ProviderFactories: protoV6ProviderFactories(client),
		Steps: []resource.TestStep{
			{
				Config:      testAccDataSourceConversation(`name = "general"`),
				ExpectError: regexp.MustCompile(`(?s)Failed\s+to\s+list.*"general".*missing_scope`),
			},
		},
	})
}

func TestAccDataSourceConversation_IDNotFound(t *testing.T) {
	t.Parallel()

	ctrl := gomock.NewController(t)
	client := mock.NewMockAPIClient(ctrl)
	client.EXPECT().HasBotToken().Return(true).AnyTimes()
	client.EXPECT().GetConversationInfoContext(gomock.Any(), &slack.GetConversationInfoInput{
		ChannelID: "C404",
	}).Return(nil, errors.New("channel_not_found")).AnyTimes()

	resource.UnitTest(t, resource.TestCase{
		ProtoV6ProviderFactories: protoV6ProviderFactories(client),
		Steps: []resource.TestStep{
			{
				Config:      testAccDataSourceConversation(`id = "C404"`),
				ExpectError: regexp.MustCompile(`(?s)id\s+"C404".*channel_not_found`),
			},
		},
	})
}

func TestAccDataSourceConversation_LookupKeys(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		config string
	}{
		{name: "both set", config: `
	id   = "C2"
	name = "general"`},
		{name: "neither set", config: ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			ctrl := gomock.NewController(t)
			client := mock.NewMockAPIClient(ctrl)
			client.EXPECT().HasBotToken().Return(true).AnyTimes()

			resource.UnitTest(t, resource.TestCase{
				ProtoV6ProviderFactories: protoV6ProviderFactories(client),
				Steps: []resource.TestStep{
					{
						Config:      testAccDataSourceConversation(tt.config),
						ExpectError: regexp.MustCompile(`(?s)exactly\s+one\s+of\s+id,\s+name\s+must\s+be\s+set`),
					},
				},
			})
		})
	}
}

func testAccDataSourceConversation(attrs string) string {
	return providerConfig + `
data "slack_conversation" "test" {
	` + attrs + `
}`
}
