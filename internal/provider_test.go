package internal

import (
	"context"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/provider"
	"github.com/hashicorp/terraform-plugin-framework/providerserver"
	"github.com/hashicorp/terraform-plugin-framework/tfsdk"
	"github.com/hashicorp/terraform-plugin-go/tfprotov6"
	"github.com/hashicorp/terraform-plugin-go/tftypes"
)

const (
	providerConfig = `
provider "slack" {
	token = "test"
}`

	providerConfigNoToken = `
provider "slack" {
	app_configuration_token = "test"
}`
)

func TestConfigure_TokenEnvFallback(t *testing.T) {
	tests := []struct {
		name            string
		token           *string
		appConfigToken  *string
		envToken        string
		envAppToken     string
		wantHasToken    bool
		wantConfigToken string
	}{
		{
			name:            "explicit config wins over env",
			token:           ptr("cfg-bot"),
			appConfigToken:  ptr("cfg-app"),
			envToken:        "env-bot",
			envAppToken:     "env-app",
			wantHasToken:    true,
			wantConfigToken: "cfg-app",
		},
		{
			name:            "unset attributes fall back to env",
			envToken:        "env-bot",
			envAppToken:     "env-app",
			wantHasToken:    true,
			wantConfigToken: "env-app",
		},
		{
			name:            "unset attributes without env stay empty",
			wantHasToken:    false,
			wantConfigToken: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Setenv("SLACK_TOKEN", tt.envToken)
			t.Setenv("SLACK_APP_CONFIGURATION_TOKEN", tt.envAppToken)

			p := &SlackProvider{}
			var schemaResp provider.SchemaResponse
			p.Schema(context.Background(), provider.SchemaRequest{}, &schemaResp)

			objType := tftypes.Object{AttributeTypes: map[string]tftypes.Type{
				"token":                   tftypes.String,
				"app_configuration_token": tftypes.String,
			}}
			raw := tftypes.NewValue(objType, map[string]tftypes.Value{
				"token":                   tftypes.NewValue(tftypes.String, stringOrNil(tt.token)),
				"app_configuration_token": tftypes.NewValue(tftypes.String, stringOrNil(tt.appConfigToken)),
			})

			var resp provider.ConfigureResponse
			p.Configure(context.Background(), provider.ConfigureRequest{
				Config: tfsdk.Config{Raw: raw, Schema: schemaResp.Schema},
			}, &resp)

			if resp.Diagnostics.HasError() {
				t.Errorf("Configure() diagnostics: %v", resp.Diagnostics)
				return
			}
			c, ok := p.client.(*Client)
			if !ok {
				t.Errorf("client = %T, want *Client", p.client)
				return
			}
			if c.hasToken != tt.wantHasToken {
				t.Errorf("hasToken = %v, want %v", c.hasToken, tt.wantHasToken)
			}
			if c.configToken != tt.wantConfigToken {
				t.Errorf("configToken = %q, want %q", c.configToken, tt.wantConfigToken)
			}
		})
	}
}

func ptr(s string) *string { return &s }

func stringOrNil(s *string) any {
	if s == nil {
		return nil
	}
	return *s
}

func protoV6ProviderFactories(client APIClient) map[string]func() (tfprotov6.ProviderServer, error) {
	return map[string]func() (tfprotov6.ProviderServer, error){
		"slack": providerserver.NewProtocol6WithError(
			&SlackProvider{
				client: client,
			},
		),
	}
}
