package internal

import (
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/slack-go/slack"
	"go.uber.org/mock/gomock"

	"github.com/sivchari/terraform-provider-slack/internal/mock"
)

func TestAccConversationResource(t *testing.T) {
	t.Parallel()

	resp := slack.Channel{
		GroupConversation: slack.GroupConversation{
			Conversation: slack.Conversation{
				ID:        "test",
				IsPrivate: true,
			},
			Name: "test",
			Topic: slack.Topic{
				Value: "test",
			},
			Purpose: slack.Purpose{
				Value: "test",
			},
			Members: []string{"test"},
		},
	}

	ctrl := gomock.NewController(t)
	client := mock.NewMockAPIClient(ctrl)

	client.EXPECT().CreateConversationContext(gomock.Any(), gomock.Any()).Return(&resp, nil).AnyTimes()
	client.EXPECT().SetTopicOfConversationContext(gomock.Any(), "test", "test").Return(&resp, nil).AnyTimes()
	client.EXPECT().SetPurposeOfConversationContext(gomock.Any(), "test", "test").Return(&resp, nil).AnyTimes()
	client.EXPECT().InviteUsersToConversationContext(gomock.Any(), "test", "test,test2").Return(&resp, nil).AnyTimes()
	client.EXPECT().GetUsersInConversationContext(gomock.Any(), gomock.Any()).Return([]string{"test", "test2", "test3"}, "", nil).AnyTimes()
	client.EXPECT().KickUserFromConversationContext(gomock.Any(), "test", "test3").Return(nil).AnyTimes()
	client.EXPECT().GetConversationInfoContext(gomock.Any(), gomock.Any()).Return(&resp, nil).AnyTimes()
	client.EXPECT().ArchiveConversationContext(gomock.Any(), "test").Return(nil).AnyTimes()

	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: protoV6ProviderFactories(client),
		Steps: []resource.TestStep{
			{
				Config: testAccConversationResource(),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("slack_conversation.test", "id", "test"),
					resource.TestCheckResourceAttr("slack_conversation.test", "name", "test"),
					resource.TestCheckResourceAttr("slack_conversation.test", "topic", "test"),
					resource.TestCheckResourceAttr("slack_conversation.test", "purpose", "test"),
					resource.TestCheckResourceAttr("slack_conversation.test", "is_private", "true"),
					resource.TestCheckResourceAttr("slack_conversation.test", "members.0", "test"),
					resource.TestCheckResourceAttr("slack_conversation.test", "members.1", "test2"),
				),
			},
		},
	})
}

func testAccConversationResource() string {
	return providerConfig + `
resource "slack_conversation" "test" {
	name = "test"
	topic = "test"
	purpose = "test"
	is_private = true
	members = ["test", "test2"]
}`
}
