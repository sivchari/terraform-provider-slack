package internal

import (
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/sivchari/terraform-provider-slack/internal/mock"
	"github.com/slack-go/slack"
	"go.uber.org/mock/gomock"
)

func TestAccDataSourceConversation(t *testing.T) {
	t.Parallel()

	conversationInfoResp := &slack.Channel{
		GroupConversation: slack.GroupConversation{
			Conversation: slack.Conversation{
				ID:               "test",
				IsPrivate:        false,
				User:             "test",
				ConnectedTeamIDs: []string{"test"},
				SharedTeamIDs:    []string{"test"},
				InternalTeamIDs:  []string{"test"},
			},
			Name:       "test",
			Creator:    "test",
			IsArchived: false,
			Topic: slack.Topic{
				Value:   "test",
				Creator: "test",
			},
			Purpose: slack.Purpose{
				Value:   "test",
				Creator: "test",
			},
		},
	}

	usersResp := []string{"test"}

	ctrl := gomock.NewController(t)
	client := mock.NewMockAPIClient(ctrl)
	client.EXPECT().GetConversationInfoContext(gomock.Any(), &slack.GetConversationInfoInput{
		ChannelID: "test",
	}).Return(conversationInfoResp, nil).AnyTimes()
	client.EXPECT().GetUsersInConversationContext(gomock.Any(), &slack.GetUsersInConversationParameters{
		ChannelID: "test",
	}).Return(usersResp, "", nil).AnyTimes()

	resource.UnitTest(t, resource.TestCase{
		ProtoV6ProviderFactories: protoV6ProviderFactories(client),
		Steps: []resource.TestStep{
			{
				Config: testAccDataSourceConversation(),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("data.slack_conversation.test", "id", "test"),
					resource.TestCheckResourceAttr("data.slack_conversation.test", "name", "test"),
					resource.TestCheckResourceAttr("data.slack_conversation.test", "creator", "test"),
					resource.TestCheckResourceAttr("data.slack_conversation.test", "is_archived", "false"),
					resource.TestCheckResourceAttr("data.slack_conversation.test", "members.0", "test"),
					resource.TestCheckResourceAttr("data.slack_conversation.test", "topic.value", "test"),
					resource.TestCheckResourceAttr("data.slack_conversation.test", "topic.creator", "test"),
					resource.TestCheckResourceAttr("data.slack_conversation.test", "purpose.value", "test"),
					resource.TestCheckResourceAttr("data.slack_conversation.test", "purpose.creator", "test"),
					resource.TestCheckResourceAttr("data.slack_conversation.test", "is_private", "false"),
					resource.TestCheckResourceAttr("data.slack_conversation.test", "user", "test"),
					resource.TestCheckResourceAttr("data.slack_conversation.test", "connected_team_ids.0", "test"),
					resource.TestCheckResourceAttr("data.slack_conversation.test", "shared_team_ids.0", "test"),
					resource.TestCheckResourceAttr("data.slack_conversation.test", "internal_team_ids.0", "test"),
				),
			},
		},
	})
}

func testAccDataSourceConversation() string {
	return providerConfig + `
data "slack_conversation" "test" {
	id = "test"
}`
}
