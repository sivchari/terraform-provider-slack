package internal

import (
	"context"
	"errors"
	"fmt"
	"reflect"
	"regexp"
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
			{
				ResourceName:      "slack_app.test",
				ImportState:       true,
				ImportStateVerify: true,
				// credentials are only returned by apps.manifest.create and
				// stay null after an import
				ImportStateVerifyIgnore: []string{
					"client_id",
					"client_secret",
					"verification_token",
					"signing_secret",
					"oauth_authorize_url",
				},
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

// TestAccAppResourceDeleteAppDeletedRemotely covers destroy of an app that is
// already gone: apps.manifest.delete answers app_not_found, which must count
// as success or the resource could never leave state.
func TestAccAppResourceDeleteAppDeletedRemotely(t *testing.T) {
	t.Parallel()

	ctrl := gomock.NewController(t)
	client := mock.NewMockAPIClient(ctrl)

	remote := testAccAppManifest("test")
	client.EXPECT().CreateAppManifest(gomock.Any(), gomock.Any(), "").Return(&appmanifest.CreateResponse{AppID: "A012345678"}, nil).AnyTimes()
	client.EXPECT().ExportAppManifest(gomock.Any(), "", "A012345678").Return(remote, nil).AnyTimes()
	client.EXPECT().DeleteManifestContext(gomock.Any(), "", "A012345678").Return(
		&slack.SlackResponse{Ok: false, Error: "app_not_found"},
		slack.SlackErrorResponse{Err: "app_not_found"},
	).MinTimes(1)

	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: protoV6ProviderFactories(client),
		Steps: []resource.TestStep{
			{
				Config: testAccAppResource(),
				Check:  resource.TestCheckResourceAttr("slack_app.test", "id", "A012345678"),
			},
		},
	})
}

// TestAccAppResourceUpdateCallAppNotFound covers an update where the export
// succeeds but apps.manifest.update itself answers app_not_found.
func TestAccAppResourceUpdateCallAppNotFound(t *testing.T) {
	t.Parallel()

	ctrl := gomock.NewController(t)
	client := mock.NewMockAPIClient(ctrl)

	client.EXPECT().CreateAppManifest(gomock.Any(), gomock.Any(), "").Return(&appmanifest.CreateResponse{AppID: "A012345678"}, nil).AnyTimes()
	client.EXPECT().ExportAppManifest(gomock.Any(), "", "A012345678").Return(testAccAppManifest("test"), nil).AnyTimes()
	client.EXPECT().UpdateAppManifest(gomock.Any(), gomock.Any(), "", "A012345678").Return(
		nil,
		&appmanifest.Error{Method: "apps.manifest.update", Code: "app_not_found"},
	).MinTimes(1)
	client.EXPECT().DeleteManifestContext(gomock.Any(), "", "A012345678").Return(&slack.SlackResponse{Ok: true}, nil).AnyTimes()

	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: protoV6ProviderFactories(client),
		Steps: []resource.TestStep{
			{
				Config: testAccAppResource(),
				Check:  resource.TestCheckResourceAttr("slack_app.test", "id", "A012345678"),
			},
			{
				Config:      testAccAppResourceRenamed(),
				ExpectError: regexp.MustCompile(`(?is)app\s+not\s+found.*run\s+terraform\s+plan\s+again\s+to\s+recreate`),
			},
		},
	})
}

// TestAccAppResourceUpdatePermissionsUpdated covers an update whose scope
// change Slack reports through permissions_updated. The provider only warns,
// so the apply must still succeed and store the planned values.
func TestAccAppResourceUpdatePermissionsUpdated(t *testing.T) {
	t.Parallel()

	ctrl := gomock.NewController(t)
	client := mock.NewMockAPIClient(ctrl)

	remote := testAccAppManifest("test")
	client.EXPECT().CreateAppManifest(gomock.Any(), gomock.Any(), "").Return(&appmanifest.CreateResponse{AppID: "A012345678"}, nil).AnyTimes()
	client.EXPECT().ExportAppManifest(gomock.Any(), "", "A012345678").DoAndReturn(
		func(context.Context, string, string) (appmanifest.Document, error) {
			return remote, nil
		},
	).AnyTimes()
	client.EXPECT().UpdateAppManifest(gomock.Any(), gomock.Any(), "", "A012345678").DoAndReturn(
		func(_ context.Context, manifest appmanifest.Document, _, _ string) (*slack.UpdateManifestResponse, error) {
			remote = manifest
			return &slack.UpdateManifestResponse{AppId: "A012345678", PermissionsUpdated: true}, nil
		},
	).MinTimes(1)
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

func TestIsAppNotFound_ErrorForms(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		err  error
		want bool
	}{
		{
			name: "appmanifest error",
			err:  &appmanifest.Error{Method: "apps.manifest.export", Code: "app_not_found"},
			want: true,
		},
		{
			name: "slack error response by value",
			err:  slack.SlackErrorResponse{Err: "app_not_found"},
			want: true,
		},
		{
			name: "wrapped slack error response",
			err:  fmt.Errorf("delete: %w", slack.SlackErrorResponse{Err: "app_not_found"}),
			want: true,
		},
		{
			name: "appmanifest error with another code",
			err:  &appmanifest.Error{Method: "apps.manifest.export", Code: "invalid_auth"},
			want: false,
		},
		{
			name: "slack error response with another code",
			err:  slack.SlackErrorResponse{Err: "invalid_auth"},
			want: false,
		},
		{
			name: "slack error response by pointer",
			err:  &slack.SlackErrorResponse{Err: "app_not_found"},
			want: false,
		},
		{
			name: "plain error with the code",
			err:  errors.New("app_not_found"),
			want: false,
		},
		{
			name: "unrelated error",
			err:  errors.New("connection refused"),
			want: false,
		},
		{
			name: "nil",
			err:  nil,
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			if got := isAppNotFound(tt.err); got != tt.want {
				t.Errorf("isAppNotFound(%v) = %v, want %v", tt.err, got, tt.want)
			}
		})
	}
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

// TestAccAppResourceUpdateAppDeletedRemotely covers Update's own export call
// hitting app_not_found: the framework cannot remove a resource from state
// mid-apply, so this must surface as a clear error telling the user to plan
// again, instead of the generic export-failure diagnostic.
func TestAccAppResourceUpdateAppDeletedRemotely(t *testing.T) {
	t.Parallel()

	ctrl := gomock.NewController(t)
	client := mock.NewMockAPIClient(ctrl)

	remote := testAccAppManifest("test")
	notFound := &appmanifest.Error{Method: "apps.manifest.export", Code: "app_not_found"}
	var exportCalls int
	client.EXPECT().CreateAppManifest(gomock.Any(), gomock.Any(), "").Return(&appmanifest.CreateResponse{AppID: "A012345678"}, nil).AnyTimes()
	client.EXPECT().ExportAppManifest(gomock.Any(), "", "A012345678").DoAndReturn(
		func(context.Context, string, string) (appmanifest.Document, error) {
			exportCalls++
			// Calls 1 and 2 are the refreshes after step 1 and before step
			// 2's plan; call 3 is Update's own export and must fail.
			if exportCalls <= 2 {
				return remote, nil
			}
			return nil, notFound
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
				Config:      testAccAppResourceRenamed(),
				ExpectError: regexp.MustCompile("app not found"),
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
	// an unmanaged nested empty object, meaningful to Slack, that no
	// pruning may reach
	originalFunctions := map[string]any{
		"f": map[string]any{"output_parameters": map[string]any{"properties": map[string]any{}}},
	}
	remote["functions"] = originalFunctions

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
						if !reflect.DeepEqual(updated["functions"], originalFunctions) {
							return fmt.Errorf("updated[\"functions\"] = %v, want %v", updated["functions"], originalFunctions)
						}
						return nil
					},
				),
			},
		},
	})
}
