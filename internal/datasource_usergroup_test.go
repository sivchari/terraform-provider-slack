package internal

import (
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/sivchari/terraform-provider-slack/internal/mock"
	"github.com/slack-go/slack"
	"go.uber.org/mock/gomock"
)

func TestAccUserGroup(t *testing.T) {
	t.Parallel()

	resp := []slack.UserGroup{
		{
			ID:          "test",
			TeamID:      "test",
			IsUserGroup: true,
			Name:        "test",
			Description: "test",
			Handle:      "admins",
			IsExternal:  true,
			AutoType:    "test",
			CreatedBy:   "test",
			UpdatedBy:   "test",
			DeletedBy:   "test",
			Prefs: slack.UserGroupPrefs{
				Channels: []string{"test"},
				Groups:   []string{"test"},
			},
			UserCount: 1,
			Users:     []string{"test"},
		},
	}

	ctrl := gomock.NewController(t)
	client := mock.NewMockAPIClient(ctrl)
	client.EXPECT().GetUserGroupsContext(gomock.Any(), gomock.Any()).Return(resp, nil).AnyTimes()

	resource.UnitTest(t, resource.TestCase{
		ProtoV6ProviderFactories: protoV6ProviderFactories(client),
		Steps: []resource.TestStep{
			{
				Config: testAccUserGroup(),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("data.slack_usergroup.test", "id", "test"),
					resource.TestCheckResourceAttr("data.slack_usergroup.test", "team_id", "test"),
					resource.TestCheckResourceAttr("data.slack_usergroup.test", "is_user_group", "true"),
					resource.TestCheckResourceAttr("data.slack_usergroup.test", "name", "test"),
					resource.TestCheckResourceAttr("data.slack_usergroup.test", "description", "test"),
					resource.TestCheckResourceAttr("data.slack_usergroup.test", "handle", "admins"),
					resource.TestCheckResourceAttr("data.slack_usergroup.test", "is_external", "true"),
					resource.TestCheckResourceAttr("data.slack_usergroup.test", "auto_type", "test"),
					resource.TestCheckResourceAttr("data.slack_usergroup.test", "created_by", "test"),
					resource.TestCheckResourceAttr("data.slack_usergroup.test", "updated_by", "test"),
					resource.TestCheckResourceAttr("data.slack_usergroup.test", "deleted_by", "test"),
					resource.TestCheckResourceAttr("data.slack_usergroup.test", "prefs.channels.0", "test"),
					resource.TestCheckResourceAttr("data.slack_usergroup.test", "prefs.groups.0", "test"),
					resource.TestCheckResourceAttr("data.slack_usergroup.test", "user_count", "1"),
					resource.TestCheckResourceAttr("data.slack_usergroup.test", "users.0", "test"),
				),
			},
		},
	})
}

func testAccUserGroup() string {
	return providerConfig + `
data "slack_usergroup" "test" {
    id = "test"
}`
}
