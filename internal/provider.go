//go:generate mockgen -source=$GOFILE -package=mock -destination=./mock/mock.go

package internal

import (
	"context"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/provider"
	"github.com/hashicorp/terraform-plugin-framework/provider/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-log/tflog"
	"github.com/slack-go/slack"
)

var _ provider.Provider = &SlackProvider{}

type APIClient interface {
	GetUserByEmailContext(ctx context.Context, email string) (*slack.User, error)
	CreateUserGroupContext(ctx context.Context, userGroup slack.UserGroup) (slack.UserGroup, error)
	GetUserGroupsContext(ctx context.Context, opts ...slack.GetUserGroupsOption) ([]slack.UserGroup, error)
	UpdateUserGroupContext(ctx context.Context, userGroupID string, opts ...slack.UpdateUserGroupsOption) (slack.UserGroup, error)
	UpdateUserGroupMembersContext(ctx context.Context, userGroup string, members string) (slack.UserGroup, error)
	EnableUserGroupContext(ctx context.Context, userGroup string) (slack.UserGroup, error)
	DisableUserGroupContext(ctx context.Context, userGroup string) (slack.UserGroup, error)
}

type SlackProvider struct {
	client APIClient
}

type SlackProviderConfig struct {
	Token types.String `tfsdk:"token"`
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
				Required:  true,
				Sensitive: true,
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
	if resp.Diagnostics.HasError() {
		return
	}
	if m.client == nil {
		client := slack.New(cfg.Token.String())
		m.client = client
	}
	resp.DataSourceData = m.client
	resp.ResourceData = m.client
	tflog.Info(ctx, "configured slack-provider")
}

func (m *SlackProvider) Resources(_ context.Context) []func() resource.Resource {
	return []func() resource.Resource{
		NewUserGroupResource,
	}
}

func (m *SlackProvider) DataSources(_ context.Context) []func() datasource.DataSource {
	return []func() datasource.DataSource{
		NewUserDataSource,
		NewUserGroupDataSource,
	}
}
