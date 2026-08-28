package internal

import (
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/slack-go/slack"
	"go.uber.org/mock/gomock"

	"github.com/sivchari/terraform-provider-slack/internal/mock"
)

func TestAccAppConfigTokenResource(t *testing.T) {
	t.Parallel()

	resp := &slack.TokenResponse{
		Token:        "xoxe.xoxp-test-token",
		RefreshToken: "xoxe-1-test-refresh-token",
		TeamId:       "T012345",
		ExpiresAt:    9999999999,
	}

	ctrl := gomock.NewController(t)
	client := mock.NewMockAPIClient(ctrl)

	client.EXPECT().RotateTokensContext(gomock.Any(), "", "xoxe-1-seed-refresh-token").Return(resp, nil).AnyTimes()

	original := appConfigTokenGetenv
	appConfigTokenGetenv = func(string) string { return "xoxe-1-seed-refresh-token" }
	t.Cleanup(func() { appConfigTokenGetenv = original })

	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: protoV6ProviderFactories(client),
		Steps: []resource.TestStep{
			{
				Config: testAccAppConfigTokenResource(),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("slack_app_config_token.test", "id", "T012345"),
					resource.TestCheckResourceAttr("slack_app_config_token.test", "token", "xoxe.xoxp-test-token"),
					resource.TestCheckResourceAttr("slack_app_config_token.test", "refresh_token", "xoxe-1-test-refresh-token"),
					resource.TestCheckResourceAttr("slack_app_config_token.test", "expires_at", "9999996399"),
				),
			},
		},
	})
}

func testAccAppConfigTokenResource() string {
	return providerConfig + `
resource "slack_app_config_token" "test" {}`
}

func TestResolveSeedRefreshToken(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		env     map[string]string
		want    string
		wantErr bool
	}{
		{
			name: "reads from env",
			env:  map[string]string{appConfigTokenRefreshEnvVar: "xoxe-1-env"},
			want: "xoxe-1-env",
		},
		{
			name:    "errors when unset",
			env:     map[string]string{},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got, err := resolveSeedRefreshToken(func(key string) string { return tt.env[key] })
			if (err != nil) != tt.wantErr {
				t.Errorf("resolveSeedRefreshToken() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if got != tt.want {
				t.Errorf("resolveSeedRefreshToken() = %q, want %q", got, tt.want)
			}
		})
	}
}
