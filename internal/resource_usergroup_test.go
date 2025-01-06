package internal

import (
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/slack-go/slack"
	"go.uber.org/mock/gomock"

	"github.com/sivchari/terraform-provider-slack/internal/mock"
)

func TestAccUserGroupResource(t *testing.T) {
	t.Parallel()

	resp := slack.UserGroup{
		ID:   "test",
		Name: "test",
		Prefs: slack.UserGroupPrefs{
			Channels: []string{"test"},
		},
		Users:       []string{"test"},
		Description: "test",
		Handle:      "test",
		TeamID:      "test",
	}

	ctrl := gomock.NewController(t)
	client := mock.NewMockAPIClient(ctrl)

	// TODO: make req's type more strict
	client.EXPECT().CreateUserGroupContext(gomock.Any(), gomock.Any()).Return(resp, nil).AnyTimes()
	client.EXPECT().DisableUserGroupContext(gomock.Any(), "test").Return(resp, nil).AnyTimes()
	client.EXPECT().UpdateUserGroupContext(gomock.Any(), gomock.Any()).Return(resp, nil).AnyTimes()
	client.EXPECT().UpdateUserGroupMembersContext(gomock.Any(), "test", "test").Return(resp, nil).AnyTimes()

	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: protoV6ProviderFactories(client),
		Steps: []resource.TestStep{
			{
				Config: testAccUserGroupResource(),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("slack_usergroup.test", "id", "test"),
					resource.TestCheckResourceAttr("slack_usergroup.test", "name", "test"),
					resource.TestCheckResourceAttr("slack_usergroup.test", "channels.0", "test"),
					resource.TestCheckResourceAttr("slack_usergroup.test", "users.0", "test"),
					resource.TestCheckResourceAttr("slack_usergroup.test", "description", "test"),
					resource.TestCheckResourceAttr("slack_usergroup.test", "handle", "test"),
					resource.TestCheckResourceAttr("slack_usergroup.test", "team_id", "test"),
					resource.TestCheckResourceAttr("slack_usergroup.test", "enabled", "true"),
				),
			},
		},
	})
}

func testAccUserGroupResource() string {
	return providerConfig + `
resource "slack_usergroup" "test" {
	name = "test"
	channels = ["test"]
	users = ["test"]
	description = "test"
	handle = "test"
	team_id = "test"
}`
}
