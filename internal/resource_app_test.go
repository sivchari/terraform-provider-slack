package internal

import (
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/slack-go/slack"
	"go.uber.org/mock/gomock"

	"github.com/sivchari/terraform-provider-slack/internal/appmanifest"
	"github.com/sivchari/terraform-provider-slack/internal/mock"
)

func TestAccAppResource(t *testing.T) {
	t.Parallel()

	createResp := &appmanifest.CreateResponse{
		AppID: "A012345678",
		Credentials: appmanifest.Credentials{
			ClientID:          "1234567890.1234567890123",
			ClientSecret:      "abcdefghijklmnopqrstuvwxyz012345",
			VerificationToken: "abcdefghijklmnopqrstuvwx",
			SigningSecret:     "0123456789abcdef0123456789abcdef",
		},
		OAuthAuthorizeURL: "https://slack.com/oauth/v2/authorize?client_id=1234567890.1234567890123",
	}

	ctrl := gomock.NewController(t)
	client := mock.NewMockAPIClient(ctrl)

	client.EXPECT().CreateAppManifest(gomock.Any(), gomock.Any(), "").Return(createResp, nil).AnyTimes()
	client.EXPECT().DeleteManifestContext(gomock.Any(), "", "A012345678").Return(&slack.SlackResponse{Ok: true}, nil).AnyTimes()

	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: protoV6ProviderFactories(client),
		Steps: []resource.TestStep{
			{
				Config: testAccAppResource(),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("slack_app.test", "id", "A012345678"),
					resource.TestCheckResourceAttr("slack_app.test", "client_id", "1234567890.1234567890123"),
					resource.TestCheckResourceAttr("slack_app.test", "client_secret", "abcdefghijklmnopqrstuvwxyz012345"),
					resource.TestCheckResourceAttr("slack_app.test", "verification_token", "abcdefghijklmnopqrstuvwx"),
					resource.TestCheckResourceAttr("slack_app.test", "signing_secret", "0123456789abcdef0123456789abcdef"),
					resource.TestCheckResourceAttr("slack_app.test", "oauth_authorize_url", "https://slack.com/oauth/v2/authorize?client_id=1234567890.1234567890123"),
					resource.TestCheckResourceAttr("slack_app.test", "display_information.name", "test"),
					resource.TestCheckResourceAttr("slack_app.test", "features.bot_user.display_name", "test-bot"),
					resource.TestCheckResourceAttr("slack_app.test", "oauth_config.scopes.bot.0", "chat:write"),
				),
			},
		},
	})
}

func TestAccAppResourceWithoutBotToken(t *testing.T) {
	t.Parallel()

	createResp := &appmanifest.CreateResponse{
		AppID: "A012345678",
	}

	ctrl := gomock.NewController(t)
	client := mock.NewMockAPIClient(ctrl)

	client.EXPECT().CreateAppManifest(gomock.Any(), gomock.Any(), "").Return(createResp, nil).AnyTimes()
	client.EXPECT().DeleteManifestContext(gomock.Any(), "", "A012345678").Return(&slack.SlackResponse{Ok: true}, nil).AnyTimes()

	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: protoV6ProviderFactories(client),
		Steps: []resource.TestStep{
			{
				Config: providerConfigNoToken + testAccAppResourceConfig(),
				Check:  resource.TestCheckResourceAttr("slack_app.test", "id", "A012345678"),
			},
		},
	})
}

func TestAccAppResourceUpdate(t *testing.T) {
	t.Parallel()

	createResp := &appmanifest.CreateResponse{
		AppID: "A012345678",
		Credentials: appmanifest.Credentials{
			ClientID:          "1234567890.1234567890123",
			ClientSecret:      "abcdefghijklmnopqrstuvwxyz012345",
			VerificationToken: "abcdefghijklmnopqrstuvwx",
			SigningSecret:     "0123456789abcdef0123456789abcdef",
		},
		OAuthAuthorizeURL: "https://slack.com/oauth/v2/authorize?client_id=1234567890.1234567890123",
	}

	ctrl := gomock.NewController(t)
	client := mock.NewMockAPIClient(ctrl)

	client.EXPECT().CreateAppManifest(gomock.Any(), gomock.Any(), "").Return(createResp, nil).AnyTimes()
	// The expectation pins the app_id argument: an unknown plan ID would reach
	// Slack as "" and fail the test here.
	client.EXPECT().UpdateManifestContext(gomock.Any(), gomock.Any(), "", "A012345678").Return(&slack.UpdateManifestResponse{}, nil).AnyTimes()
	client.EXPECT().DeleteManifestContext(gomock.Any(), "", "A012345678").Return(&slack.SlackResponse{Ok: true}, nil).AnyTimes()

	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: protoV6ProviderFactories(client),
		Steps: []resource.TestStep{
			{
				Config: testAccAppResource(),
				Check:  resource.TestCheckResourceAttr("slack_app.test", "id", "A012345678"),
			},
			{
				Config: testAccAppResourceRenamed(),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("slack_app.test", "id", "A012345678"),
					resource.TestCheckResourceAttr("slack_app.test", "display_information.name", "test-renamed"),
				),
			},
		},
	})
}

func testAccAppResourceRenamed() string {
	return providerConfig + `
resource "slack_app" "test" {
	display_information = {
		name = "test-renamed"
	}
	features = {
		bot_user = {
			display_name = "test-bot"
		}
	}
	oauth_config = {
		scopes = {
			bot = ["chat:write"]
		}
	}
}`
}

func testAccAppResource() string {
	return providerConfig + testAccAppResourceConfig()
}

func testAccAppResourceConfig() string {
	return `
resource "slack_app" "test" {
	display_information = {
		name = "test"
	}
	features = {
		bot_user = {
			display_name = "test-bot"
		}
	}
	oauth_config = {
		scopes = {
			bot = ["chat:write"]
		}
	}
}`
}
