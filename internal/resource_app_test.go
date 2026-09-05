package internal

import (
	"context"
	"fmt"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/terraform"
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
	client.EXPECT().ExportAppManifest(gomock.Any(), "", "A012345678").Return(testAccAppManifest("test"), nil).AnyTimes()
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
	client.EXPECT().ExportAppManifest(gomock.Any(), "", "A012345678").Return(testAccAppManifest("test"), nil).AnyTimes()
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

	remote := testAccAppManifest("test")
	client.EXPECT().CreateAppManifest(gomock.Any(), gomock.Any(), "").Return(createResp, nil).AnyTimes()
	client.EXPECT().ExportAppManifest(gomock.Any(), "", "A012345678").DoAndReturn(
		func(context.Context, string, string) (appmanifest.Document, error) {
			return remote, nil
		},
	).AnyTimes()
	// The expectation pins the app_id argument: an unknown plan ID would reach
	// Slack as "" and fail the test here.
	client.EXPECT().UpdateAppManifest(gomock.Any(), gomock.Any(), "", "A012345678").DoAndReturn(
		func(_ context.Context, manifest appmanifest.Document, _, _ string) (*slack.UpdateManifestResponse, error) {
			remote = manifest
			return &slack.UpdateManifestResponse{}, nil
		},
	).AnyTimes()
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

func TestAccAppResourceRemoteDrift(t *testing.T) {
	t.Parallel()

	ctrl := gomock.NewController(t)
	client := mock.NewMockAPIClient(ctrl)

	client.EXPECT().CreateAppManifest(gomock.Any(), gomock.Any(), "").Return(&appmanifest.CreateResponse{AppID: "A012345678"}, nil).AnyTimes()
	client.EXPECT().ExportAppManifest(gomock.Any(), "", "A012345678").Return(testAccAppManifest("drifted"), nil).AnyTimes()
	client.EXPECT().DeleteManifestContext(gomock.Any(), "", "A012345678").Return(&slack.SlackResponse{Ok: true}, nil).AnyTimes()

	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: protoV6ProviderFactories(client),
		Steps: []resource.TestStep{
			{
				Config:             testAccAppResource(),
				ExpectNonEmptyPlan: true,
			},
			{
				RefreshState:       true,
				ExpectNonEmptyPlan: true,
				Check:              resource.TestCheckResourceAttr("slack_app.test", "display_information.name", "drifted"),
			},
		},
	})
}

func TestAccAppResourceRemovedRemotely(t *testing.T) {
	t.Parallel()

	ctrl := gomock.NewController(t)
	client := mock.NewMockAPIClient(ctrl)

	client.EXPECT().CreateAppManifest(gomock.Any(), gomock.Any(), "").Return(&appmanifest.CreateResponse{AppID: "A012345678"}, nil).AnyTimes()
	notFound := &appmanifest.Error{Method: "apps.manifest.export", Code: "app_not_found"}
	client.EXPECT().ExportAppManifest(gomock.Any(), "", "A012345678").Return(nil, notFound).AnyTimes()
	client.EXPECT().DeleteManifestContext(gomock.Any(), "", "A012345678").Return(&slack.SlackResponse{Ok: true}, nil).AnyTimes()

	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: protoV6ProviderFactories(client),
		Steps: []resource.TestStep{
			{
				Config: testAccAppResource(),
				// The post-apply refresh drops the remotely deleted app from
				// state, so the follow-up plan proposes recreating it.
				ExpectNonEmptyPlan: true,
			},
			{
				RefreshState:       true,
				ExpectNonEmptyPlan: true,
				Check: func(s *terraform.State) error {
					if _, ok := s.RootModule().Resources["slack_app.test"]; ok {
						return fmt.Errorf("slack_app.test still in state, want it removed")
					}
					return nil
				},
			},
		},
	})
}

// TestAccAppResourceUpdatePreservesUnmanagedFields covers the PUT semantics
// of apps.manifest.update: fields outside the schema (Agents & AI Apps,
// unfurl domains, token rotation, _metadata, ...) must be sent back as-is
// or Slack removes them from the app.
func TestAccAppResourceUpdatePreservesUnmanagedFields(t *testing.T) {
	t.Parallel()

	ctrl := gomock.NewController(t)
	client := mock.NewMockAPIClient(ctrl)

	remote := testAccAppManifest("test")
	remote["_metadata"] = map[string]any{"major_version": float64(1), "minor_version": float64(1)}
	remote["features"].(map[string]any)["unfurl_domains"] = []any{"example.com"}
	remote["settings"] = map[string]any{"token_rotation_enabled": false}

	var updated appmanifest.Document
	client.EXPECT().CreateAppManifest(gomock.Any(), gomock.Any(), "").Return(&appmanifest.CreateResponse{AppID: "A012345678"}, nil).AnyTimes()
	client.EXPECT().ExportAppManifest(gomock.Any(), "", "A012345678").DoAndReturn(
		func(context.Context, string, string) (appmanifest.Document, error) {
			return remote, nil
		},
	).AnyTimes()
	client.EXPECT().UpdateAppManifest(gomock.Any(), gomock.Any(), "", "A012345678").DoAndReturn(
		func(_ context.Context, manifest appmanifest.Document, _, _ string) (*slack.UpdateManifestResponse, error) {
			updated = manifest
			remote = manifest
			return &slack.UpdateManifestResponse{}, nil
		},
	).AnyTimes()
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
					resource.TestCheckResourceAttr("slack_app.test", "display_information.name", "test-renamed"),
					func(*terraform.State) error {
						if updated == nil {
							return fmt.Errorf("apps.manifest.update was not called")
						}
						if got := updated["display_information"].(map[string]any)["name"]; got != "test-renamed" {
							return fmt.Errorf("updated display_information.name = %v, want %q", got, "test-renamed")
						}
						if _, ok := updated["_metadata"]; !ok {
							return fmt.Errorf("updated manifest %v lost _metadata", updated)
						}
						features, _ := updated["features"].(map[string]any)
						if _, ok := features["unfurl_domains"]; !ok {
							return fmt.Errorf("updated manifest %v lost features.unfurl_domains", updated)
						}
						settings, _ := updated["settings"].(map[string]any)
						if _, ok := settings["token_rotation_enabled"]; !ok {
							return fmt.Errorf("updated manifest %v lost settings.token_rotation_enabled", updated)
						}
						return nil
					},
				),
			},
		},
	})
}

func testAccAppManifest(name string) appmanifest.Document {
	return appmanifest.Document{
		"display_information": map[string]any{"name": name},
		"features": map[string]any{
			"bot_user": map[string]any{"display_name": "test-bot"},
		},
		"oauth_config": map[string]any{
			"scopes": map[string]any{"bot": []any{"chat:write"}},
		},
	}
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
