package internal

import (
	"context"
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/slack-go/slack"
)

var (
	_ datasource.DataSource              = &DataSourceUser{}
	_ datasource.DataSourceWithConfigure = &DataSourceUser{}

	_ datasource.DataSourceWithConfigValidators = &DataSourceUser{}
)

type DataSourceUser struct {
	client APIClient
}

type DataSourceUserState struct {
	ID                types.String `tfsdk:"id"`
	Email             types.String `tfsdk:"email"`
	TeamID            types.String `tfsdk:"team_id"`
	Name              types.String `tfsdk:"name"`
	Delete            types.Bool   `tfsdk:"deleted"`
	RealName          types.String `tfsdk:"real_name"`
	IsBot             types.Bool   `tfsdk:"is_bot"`
	IsAdmin           types.Bool   `tfsdk:"is_admin"`
	IsOwner           types.Bool   `tfsdk:"is_owner"`
	IsPrimaryOwner    types.Bool   `tfsdk:"is_primary_owner"`
	IsRestricted      types.Bool   `tfsdk:"is_restricted"`
	IsUltraRestricted types.Bool   `tfsdk:"is_ultra_restricted"`
	IsStranger        types.Bool   `tfsdk:"is_stranger"`
	IsAppUser         types.Bool   `tfsdk:"is_app_user"`
	IsInvitedUser     types.Bool   `tfsdk:"is_invited_user"`
	Has2FA            types.Bool   `tfsdk:"has_2fa"`
	TwoFactorType     types.String `tfsdk:"two_factor_type"`
	HasFiles          types.Bool   `tfsdk:"has_files"`
	Presence          types.String `tfsdk:"presence"`
	Locale            types.String `tfsdk:"locale"`
}

func NewDataSourceUser() datasource.DataSource {
	return &DataSourceUser{}
}

func (d *DataSourceUser) Metadata(_ context.Context, req datasource.MetadataRequest, res *datasource.MetadataResponse) {
	res.TypeName = fmt.Sprintf("%s_user", req.ProviderTypeName)
}

func (d *DataSourceUser) Schema(_ context.Context, _ datasource.SchemaRequest, res *datasource.SchemaResponse) {
	res.Schema = schema.Schema{
		Description: "Looks up a Slack user by ID or email address. Exactly one of id or email must be set. " +
			"Looking up by email requires the users:read and users:read.email scopes, and does not find deactivated users.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Optional:    true,
				Computed:    true,
				Description: "ID of the user, for example U0123456789. Looked up with users.info. Conflicts with email.",
			},
			"email": schema.StringAttribute{
				Optional: true,
				Computed: true,
				Description: "Email address of the user. Looked up with users.lookupByEmail. Conflicts with id. " +
					"Read-only when the user is looked up by id, and empty if the token lacks the users:read.email scope.",
			},
			"team_id": schema.StringAttribute{
				Computed:    true,
				Description: "ID of the workspace the user belongs to.",
			},
			"name": schema.StringAttribute{
				Computed:    true,
				Description: "Username (handle) of the user.",
			},
			"deleted": schema.BoolAttribute{
				Computed:    true,
				Description: "Whether the user has been deactivated.",
			},
			"real_name": schema.StringAttribute{
				Computed:    true,
				Sensitive:   true,
				Description: "Real name of the user.",
			},
			"is_bot": schema.BoolAttribute{
				Computed:    true,
				Description: "Whether the user is a bot.",
			},
			"is_admin": schema.BoolAttribute{
				Computed:    true,
				Description: "Whether the user is a workspace admin.",
			},
			"is_owner": schema.BoolAttribute{
				Computed:    true,
				Description: "Whether the user is a workspace owner.",
			},
			"is_primary_owner": schema.BoolAttribute{
				Computed:    true,
				Description: "Whether the user is the primary owner of the workspace.",
			},
			"is_restricted": schema.BoolAttribute{
				Computed:    true,
				Description: "Whether the user is a guest (multi-channel or single-channel).",
			},
			"is_ultra_restricted": schema.BoolAttribute{
				Computed:    true,
				Description: "Whether the user is a single-channel guest.",
			},
			"is_stranger": schema.BoolAttribute{
				Computed:    true,
				Description: "Whether the user belongs to another workspace and is reached through Slack Connect.",
			},
			"is_app_user": schema.BoolAttribute{
				Computed:    true,
				Description: "Whether the user is an app user.",
			},
			"is_invited_user": schema.BoolAttribute{
				Computed:    true,
				Description: "Whether the user has been invited but has not yet joined.",
			},
			"has_2fa": schema.BoolAttribute{
				Computed:    true,
				Description: "Whether the user has two-factor authentication enabled.",
			},
			"two_factor_type": schema.StringAttribute{
				Computed:    true,
				Description: "Two-factor authentication method of the user, such as app or sms. Null when two-factor authentication is off.",
			},
			"has_files": schema.BoolAttribute{
				Computed:    true,
				Description: "Whether the user has uploaded files.",
			},
			"presence": schema.StringAttribute{
				Computed: true,
				Description: "Presence of the user as reported by Slack. Slack has deprecated this field in the user object, " +
					"so it is usually empty.",
			},
			"locale": schema.StringAttribute{
				Computed:    true,
				Description: "Locale of the user, for example en-US.",
			},
		},
	}
}

func (d *DataSourceUser) ConfigValidators(context.Context) []datasource.ConfigValidator {
	return []datasource.ConfigValidator{exactlyOneOf("id", "email")}
}

func (d *DataSourceUser) Configure(ctx context.Context, req datasource.ConfigureRequest, res *datasource.ConfigureResponse) {
	if req.ProviderData == nil {
		return
	}
	client := req.ProviderData.(APIClient)
	if !client.HasBotToken() {
		res.Diagnostics.AddError(
			"Missing bot token",
			`The provider attribute "token" is required to use slack_user. Set token in the provider configuration.`,
		)
		return
	}
	d.client = client
}

func (d *DataSourceUser) Read(ctx context.Context, req datasource.ReadRequest, res *datasource.ReadResponse) {
	var state DataSourceUserState
	diags := req.Config.Get(ctx, &state)
	res.Diagnostics.Append(diags...)
	if res.Diagnostics.HasError() {
		return
	}
	user, err := d.lookup(ctx, state)
	if err != nil {
		res.Diagnostics.AddError("Failed to look up the Slack user", err.Error())
		return
	}
	email := user.Profile.Email
	if !state.Email.IsNull() {
		// Terraform rejects a result that differs from a configured value, and
		// users.lookupByEmail need not echo the address back byte for byte.
		email = state.Email.ValueString()
	}
	state = DataSourceUserState{
		ID:                types.StringValue(user.ID),
		Email:             types.StringValue(email),
		TeamID:            types.StringValue(user.TeamID),
		Name:              types.StringValue(user.Name),
		Delete:            types.BoolValue(user.Deleted),
		RealName:          types.StringValue(user.RealName),
		IsBot:             types.BoolValue(user.IsBot),
		IsAdmin:           types.BoolValue(user.IsAdmin),
		IsOwner:           types.BoolValue(user.IsOwner),
		IsPrimaryOwner:    types.BoolValue(user.IsPrimaryOwner),
		IsRestricted:      types.BoolValue(user.IsRestricted),
		IsUltraRestricted: types.BoolValue(user.IsUltraRestricted),
		IsStranger:        types.BoolValue(user.IsStranger),
		IsAppUser:         types.BoolValue(user.IsAppUser),
		IsInvitedUser:     types.BoolValue(user.IsInvitedUser),
		Has2FA:            types.BoolValue(user.Has2FA),
		HasFiles:          types.BoolValue(user.HasFiles),
		Presence:          types.StringValue(user.Presence),
		Locale:            types.StringValue(user.Locale),
	}
	if user.TwoFactorType != nil {
		state.TwoFactorType = types.StringValue(*user.TwoFactorType)
	}
	diags = res.State.Set(ctx, &state)
	res.Diagnostics.Append(diags...)
}

func (d *DataSourceUser) lookup(ctx context.Context, state DataSourceUserState) (*slack.User, error) {
	if !state.ID.IsNull() {
		user, err := d.client.GetUserInfoContext(ctx, state.ID.ValueString())
		if err != nil {
			return nil, fmt.Errorf("the user with the id %q could not be retrieved: %w", state.ID.ValueString(), err)
		}
		return user, nil
	}
	user, err := d.client.GetUserByEmailContext(ctx, state.Email.ValueString())
	if err != nil {
		return nil, fmt.Errorf("the user with the email %q could not be retrieved: %w", state.Email.ValueString(), err)
	}
	return user, nil
}
