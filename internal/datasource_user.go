package internal

import (
	"context"
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

var (
	_ datasource.DataSource              = &DataSourceUser{}
	_ datasource.DataSourceWithConfigure = &DataSourceUser{}
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

func (u *DataSourceUser) Metadata(_ context.Context, req datasource.MetadataRequest, res *datasource.MetadataResponse) {
	res.TypeName = fmt.Sprintf("%s_user", req.ProviderTypeName)
}

func (u *DataSourceUser) Schema(_ context.Context, _ datasource.SchemaRequest, res *datasource.SchemaResponse) {
	res.Schema = schema.Schema{
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Computed: true,
			},
			"email": schema.StringAttribute{
				Required: true,
			},
			"team_id": schema.StringAttribute{
				Computed: true,
			},
			"name": schema.StringAttribute{
				Computed: true,
			},
			"deleted": schema.BoolAttribute{
				Computed: true,
			},
			"real_name": schema.StringAttribute{
				Computed:  true,
				Sensitive: true,
			},
			"is_bot": schema.BoolAttribute{
				Computed: true,
			},
			"is_admin": schema.BoolAttribute{
				Computed: true,
			},
			"is_owner": schema.BoolAttribute{
				Computed: true,
			},
			"is_primary_owner": schema.BoolAttribute{
				Computed: true,
			},
			"is_restricted": schema.BoolAttribute{
				Computed: true,
			},
			"is_ultra_restricted": schema.BoolAttribute{
				Computed: true,
			},
			"is_stranger": schema.BoolAttribute{
				Computed: true,
			},
			"is_app_user": schema.BoolAttribute{
				Computed: true,
			},
			"is_invited_user": schema.BoolAttribute{
				Computed: true,
			},
			"has_2fa": schema.BoolAttribute{
				Computed: true,
			},
			"two_factor_type": schema.StringAttribute{
				Computed: true,
				Optional: true,
			},
			"has_files": schema.BoolAttribute{
				Computed: true,
			},
			"presence": schema.StringAttribute{
				Computed: true,
			},
			"locale": schema.StringAttribute{
				Computed: true,
			},
		},
	}
}

func (u *DataSourceUser) Configure(ctx context.Context, req datasource.ConfigureRequest, res *datasource.ConfigureResponse) {
	if req.ProviderData == nil {
		return
	}
	u.client = req.ProviderData.(APIClient)
}

func (u *DataSourceUser) Read(ctx context.Context, req datasource.ReadRequest, res *datasource.ReadResponse) {
	var state DataSourceUserState
	diags := req.Config.Get(ctx, &state)
	res.Diagnostics.Append(diags...)
	if res.Diagnostics.HasError() {
		return
	}
	user, err := u.client.GetUserByEmailContext(ctx, state.Email.ValueString())
	if err != nil {
		res.Diagnostics.AddError(
			fmt.Sprintf("the user that has the email %s does not exist", state.Email.String()),
			err.Error(),
		)
	}
	state = DataSourceUserState{
		ID:                types.StringValue(user.ID),
		Email:             types.StringValue(user.Profile.Email),
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
