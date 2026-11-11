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

func testSlackUser() *slack.User {
	twoFactorType := "app"
	return &slack.User{
		ID: "U1",
		Profile: slack.UserProfile{
			Email: "test@example.com",
		},
		TeamID:            "T1",
		Name:              "test",
		Deleted:           false,
		RealName:          "Test User",
		IsBot:             true,
		IsAdmin:           true,
		IsOwner:           false,
		IsPrimaryOwner:    false,
		IsRestricted:      false,
		IsUltraRestricted: false,
		IsStranger:        false,
		IsAppUser:         false,
		IsInvitedUser:     false,
		Has2FA:            true,
		TwoFactorType:     &twoFactorType,
		HasFiles:          false,
		Presence:          "away",
		Locale:            "en-US",
	}
}

func testDataSourceUserChecks() resource.TestCheckFunc {
	return resource.ComposeAggregateTestCheckFunc(
		resource.TestCheckResourceAttr("data.slack_user.test", "id", "U1"),
		resource.TestCheckResourceAttr("data.slack_user.test", "email", "test@example.com"),
		resource.TestCheckResourceAttr("data.slack_user.test", "team_id", "T1"),
		resource.TestCheckResourceAttr("data.slack_user.test", "name", "test"),
		resource.TestCheckResourceAttr("data.slack_user.test", "deleted", "false"),
		resource.TestCheckResourceAttr("data.slack_user.test", "real_name", "Test User"),
		resource.TestCheckResourceAttr("data.slack_user.test", "is_bot", "true"),
		resource.TestCheckResourceAttr("data.slack_user.test", "is_admin", "true"),
		resource.TestCheckResourceAttr("data.slack_user.test", "is_owner", "false"),
		resource.TestCheckResourceAttr("data.slack_user.test", "is_primary_owner", "false"),
		resource.TestCheckResourceAttr("data.slack_user.test", "is_restricted", "false"),
		resource.TestCheckResourceAttr("data.slack_user.test", "is_ultra_restricted", "false"),
		resource.TestCheckResourceAttr("data.slack_user.test", "is_stranger", "false"),
		resource.TestCheckResourceAttr("data.slack_user.test", "is_app_user", "false"),
		resource.TestCheckResourceAttr("data.slack_user.test", "is_invited_user", "false"),
		resource.TestCheckResourceAttr("data.slack_user.test", "has_2fa", "true"),
		resource.TestCheckResourceAttr("data.slack_user.test", "two_factor_type", "app"),
		resource.TestCheckResourceAttr("data.slack_user.test", "has_files", "false"),
		resource.TestCheckResourceAttr("data.slack_user.test", "presence", "away"),
		resource.TestCheckResourceAttr("data.slack_user.test", "locale", "en-US"),
	)
}

func TestAccDataSourceUser_Email(t *testing.T) {
	t.Parallel()

	ctrl := gomock.NewController(t)
	client := mock.NewMockAPIClient(ctrl)
	client.EXPECT().HasBotToken().Return(true).AnyTimes()
	client.EXPECT().GetUserByEmailContext(gomock.Any(), "test@example.com").Return(testSlackUser(), nil).AnyTimes()

	resource.UnitTest(t, resource.TestCase{
		ProtoV6ProviderFactories: protoV6ProviderFactories(client),
		Steps: []resource.TestStep{
			{
				Config: testAccDataSourceUser(`email = "test@example.com"`),
				Check:  testDataSourceUserChecks(),
			},
		},
	})
}

func TestAccDataSourceUser_ID(t *testing.T) {
	t.Parallel()

	ctrl := gomock.NewController(t)
	client := mock.NewMockAPIClient(ctrl)
	client.EXPECT().HasBotToken().Return(true).AnyTimes()
	client.EXPECT().GetUserInfoContext(gomock.Any(), "U1").Return(testSlackUser(), nil).AnyTimes()

	resource.UnitTest(t, resource.TestCase{
		ProtoV6ProviderFactories: protoV6ProviderFactories(client),
		Steps: []resource.TestStep{
			{
				Config: testAccDataSourceUser(`id = "U1"`),
				Check:  testDataSourceUserChecks(),
			},
		},
	})
}

func TestAccDataSourceUser_NoTwoFactor(t *testing.T) {
	t.Parallel()

	user := testSlackUser()
	user.TwoFactorType = nil

	ctrl := gomock.NewController(t)
	client := mock.NewMockAPIClient(ctrl)
	client.EXPECT().HasBotToken().Return(true).AnyTimes()
	client.EXPECT().GetUserInfoContext(gomock.Any(), "U1").Return(user, nil).AnyTimes()

	resource.UnitTest(t, resource.TestCase{
		ProtoV6ProviderFactories: protoV6ProviderFactories(client),
		Steps: []resource.TestStep{
			{
				Config: testAccDataSourceUser(`id = "U1"`),
				Check:  resource.TestCheckNoResourceAttr("data.slack_user.test", "two_factor_type"),
			},
		},
	})
}

func TestAccDataSourceUser_EmailLookupFailure(t *testing.T) {
	t.Parallel()

	ctrl := gomock.NewController(t)
	client := mock.NewMockAPIClient(ctrl)
	client.EXPECT().HasBotToken().Return(true).AnyTimes()
	client.EXPECT().GetUserByEmailContext(gomock.Any(), "gone@example.com").Return(nil, errors.New("users_not_found")).AnyTimes()

	resource.UnitTest(t, resource.TestCase{
		ProtoV6ProviderFactories: protoV6ProviderFactories(client),
		Steps: []resource.TestStep{
			{
				Config:      testAccDataSourceUser(`email = "gone@example.com"`),
				ExpectError: regexp.MustCompile(`(?s)email\s+"gone@example\.com".*users_not_found`),
			},
		},
	})
}

func TestAccDataSourceUser_IDLookupFailure(t *testing.T) {
	t.Parallel()

	ctrl := gomock.NewController(t)
	client := mock.NewMockAPIClient(ctrl)
	client.EXPECT().HasBotToken().Return(true).AnyTimes()
	client.EXPECT().GetUserInfoContext(gomock.Any(), "U404").Return(nil, errors.New("user_not_found")).AnyTimes()

	resource.UnitTest(t, resource.TestCase{
		ProtoV6ProviderFactories: protoV6ProviderFactories(client),
		Steps: []resource.TestStep{
			{
				Config:      testAccDataSourceUser(`id = "U404"`),
				ExpectError: regexp.MustCompile(`(?s)id\s+"U404".*user_not_found`),
			},
		},
	})
}

func TestAccDataSourceUser_LookupKeys(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		config string
	}{
		{name: "both set", config: `
	id    = "U1"
	email = "test@example.com"`},
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
						Config:      testAccDataSourceUser(tt.config),
						ExpectError: regexp.MustCompile(`(?s)exactly\s+one\s+of\s+id,\s+email\s+must\s+be\s+set`),
					},
				},
			})
		})
	}
}

func testAccDataSourceUser(attrs string) string {
	return providerConfig + `
data "slack_user" "test" {
	` + attrs + `
}`
}
