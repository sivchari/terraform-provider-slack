//go:generate mockgen -source=$GOFILE -package=mock -destination=./mock/mock.go

package internal

import (
	"context"
	"os"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/provider"
	"github.com/hashicorp/terraform-plugin-framework/provider/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-log/tflog"
	"github.com/sivchari/terraform-provider-slack/internal/appmanifest"
	"github.com/slack-go/slack"
)

var _ provider.Provider = &SlackProvider{}

type APIClient interface {
	GetUserByEmailContext(ctx context.Context, email string) (*slack.User, error)
	// User Groups
	CreateUserGroupContext(ctx context.Context, userGroup slack.UserGroup) (slack.UserGroup, error)
	GetUserGroupsContext(ctx context.Context, opts ...slack.GetUserGroupsOption) ([]slack.UserGroup, error)
	UpdateUserGroupContext(ctx context.Context, userGroupID string, opts ...slack.UpdateUserGroupsOption) (slack.UserGroup, error)
	UpdateUserGroupMembersContext(ctx context.Context, userGroup string, members string) (slack.UserGroup, error)
	EnableUserGroupContext(ctx context.Context, userGroup string) (slack.UserGroup, error)
	DisableUserGroupContext(ctx context.Context, userGroup string) (slack.UserGroup, error)
	// Conversations
	GetConversationInfoContext(ctx context.Context, input *slack.GetConversationInfoInput) (*slack.Channel, error)
	GetUsersInConversationContext(ctx context.Context, params *slack.GetUsersInConversationParameters) ([]string, string, error)
	CreateConversationContext(ctx context.Context, params slack.CreateConversationParams) (*slack.Channel, error)
	SetTopicOfConversationContext(ctx context.Context, channelID, topic string) (*slack.Channel, error)
	SetPurposeOfConversationContext(ctx context.Context, channelID, purpose string) (*slack.Channel, error)
	InviteUsersToConversationContext(ctx context.Context, channelID string, users ...string) (*slack.Channel, error)
	KickUserFromConversationContext(ctx context.Context, channelID string, user string) error
	ArchiveConversationContext(ctx context.Context, channelID string) error
	CloseConversationContext(ctx context.Context, channelID string) (noOp bool, alreadyClosed bool, err error)
	// App Manifests
	CreateAppManifest(ctx context.Context, manifest *appmanifest.Manifest, token string) (*appmanifest.CreateResponse, error)
	UpdateAppManifest(ctx context.Context, manifest appmanifest.Document, token, appID string) (*slack.UpdateManifestResponse, error)
	ExportAppManifest(ctx context.Context, token, appID string) (appmanifest.Document, error)
	DeleteManifestContext(ctx context.Context, token, appID string) (*slack.SlackResponse, error)
	HasBotToken() bool
}

type SlackProvider struct {
	client APIClient
}

type SlackProviderConfig struct {
	Token                 types.String `tfsdk:"token"`
	AppConfigurationToken types.String `tfsdk:"app_configuration_token"`
}

func New() func() provider.Provider {
	return func() provider.Provider {
		return &SlackProvider{}
	}
}

func (m *SlackProvider) Schema(_ context.Context, _ provider.SchemaRequest, resp *provider.SchemaResponse) {
	resp.Schema = schema.Schema{
		Attributes: map[string]schema.Attribute{
			"token": schema.StringAttribute{
				Optional:  true,
				Sensitive: true,
				Description: "Bot token required by the slack_usergroup and slack_conversation " +
					"resources and the slack_user, slack_usergroup and slack_conversation data " +
					"sources. Not needed when only managing slack_app manifests with " +
					"app_configuration_token. Falls back to the SLACK_TOKEN environment " +
					"variable when unset.",
			},
			"app_configuration_token": schema.StringAttribute{
				Optional:  true,
				Sensitive: true,
				Description: "App configuration token used for slack_app manifest calls. This token " +
					"expires after 12 hours and must be rotated outside Terraform (for example, a " +
					"scheduled job that calls tooling.tokens.rotate and writes the result to a " +
					"secret store). Falls back to the SLACK_APP_CONFIGURATION_TOKEN environment " +
					"variable when unset; prefer the environment variable over a TF_VAR in saved-plan " +
					"workflows, since values set here are captured in the plan file at plan time and " +
					"may expire before apply.",
			},
		},
	}
}

func (m *SlackProvider) Metadata(_ context.Context, _ provider.MetadataRequest, resp *provider.MetadataResponse) {
	resp.TypeName = "slack"
}

func (m *SlackProvider) Configure(ctx context.Context, req provider.ConfigureRequest, resp *provider.ConfigureResponse) {
	var cfg SlackProviderConfig
	diags := req.Config.Get(ctx, &cfg)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}
	token := cfg.Token.ValueString()
	if token == "" {
		token = os.Getenv("SLACK_TOKEN")
	}
	appConfigToken := cfg.AppConfigurationToken.ValueString()
	if appConfigToken == "" {
		appConfigToken = os.Getenv("SLACK_APP_CONFIGURATION_TOKEN")
	}
	if m.client == nil {
		m.client = NewClient(token, appConfigToken)
	}
	resp.DataSourceData = m.client
	resp.ResourceData = m.client
	tflog.Info(ctx, "configured slack-provider")
}

func (m *SlackProvider) Resources(_ context.Context) []func() resource.Resource {
	return []func() resource.Resource{
		NewResourceUserGroup,
		NewResourceConversation,
		NewResourceApp,
	}
}

func (m *SlackProvider) DataSources(_ context.Context) []func() datasource.DataSource {
	return []func() datasource.DataSource{
		NewDataSourceUser,
		NewDataSourceUserGroup,
		NewDataSourceConversation,
	}
}
