package internal

import (
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/sivchari/terraform-provider-slack/internal/mock"
	"github.com/slack-go/slack"
	"go.uber.org/mock/gomock"
)

func TestAccUser(t *testing.T) {
	t.Parallel()

	resp := &slack.User{
		ID: "test",
		Profile: slack.UserProfile{
			Email: "test@example.com",
		},
		TeamID:            "test",
		Name:              "test",
		Deleted:           false,
		RealName:          "test",
		IsBot:             true,
		IsAdmin:           false,
		IsOwner:           false,
		IsPrimaryOwner:    false,
		IsRestricted:      false,
		IsUltraRestricted: false,
		IsStranger:        false,
		IsAppUser:         false,
		IsInvitedUser:     false,
		Has2FA:            false,
		TwoFactorType:     nil,
		HasFiles:          false,
		Presence:          "test",
		Locale:            "test",
	}

	ctrl := gomock.NewController(t)
	client := mock.NewMockAPIClient(ctrl)
	client.EXPECT().GetUserByEmailContext(gomock.Any(), "test@example.com").Return(resp, nil).AnyTimes()

	resource.UnitTest(t, resource.TestCase{
		ProtoV6ProviderFactories: protoV6ProviderFactories(client),
		Steps: []resource.TestStep{
			{
				Config: testAccUser(),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("data.slack_user.test", "id", "test"),
					resource.TestCheckResourceAttr("data.slack_user.test", "email", "test@example.com"),
					resource.TestCheckResourceAttr("data.slack_user.test", "team_id", "test"),
					resource.TestCheckResourceAttr("data.slack_user.test", "name", "test"),
					resource.TestCheckResourceAttr("data.slack_user.test", "deleted", "false"),
					resource.TestCheckResourceAttr("data.slack_user.test", "real_name", "test"),
					resource.TestCheckResourceAttr("data.slack_user.test", "is_bot", "true"),
					resource.TestCheckResourceAttr("data.slack_user.test", "is_admin", "false"),
					resource.TestCheckResourceAttr("data.slack_user.test", "is_owner", "false"),
					resource.TestCheckResourceAttr("data.slack_user.test", "is_primary_owner", "false"),
					resource.TestCheckResourceAttr("data.slack_user.test", "is_restricted", "false"),
					resource.TestCheckResourceAttr("data.slack_user.test", "is_ultra_restricted", "false"),
					resource.TestCheckResourceAttr("data.slack_user.test", "is_stranger", "false"),
					resource.TestCheckResourceAttr("data.slack_user.test", "is_app_user", "false"),
					resource.TestCheckResourceAttr("data.slack_user.test", "is_invited_user", "false"),
					resource.TestCheckResourceAttr("data.slack_user.test", "has_2fa", "false"),
					resource.TestCheckResourceAttr("data.slack_user.test", "has_files", "false"),
					resource.TestCheckResourceAttr("data.slack_user.test", "presence", "test"),
					resource.TestCheckResourceAttr("data.slack_user.test", "locale", "test"),
				),
			},
		},
	})
}

func testAccUser() string {
	return providerConfig + `
data "slack_user" "test" {
    email = "test@example.com"
}`
}
