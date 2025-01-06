package internal

import (
	"context"
	"fmt"
	"math/big"

	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/slack-go/slack"
)

var (
	_ datasource.DataSource              = &UserGroupDataSource{}
	_ datasource.DataSourceWithConfigure = &UserGroupDataSource{}
)

type UserGroupDataSource struct {
	client APIClient
}

type UserGroupDataSourceState struct {
	ID          types.String              `tfsdk:"id"`
	TeamID      types.String              `tfsdk:"team_id"`
	IsUserGroup types.Bool                `tfsdk:"is_user_group"`
	Name        types.String              `tfsdk:"name"`
	Description types.String              `tfsdk:"description"`
	Handle      types.String              `tfsdk:"handle"`
	IsExternal  types.Bool                `tfsdk:"is_external"`
	AutoType    types.String              `tfsdk:"auto_type"`
	CreatedBy   types.String              `tfsdk:"created_by"`
	UpdatedBy   types.String              `tfsdk:"updated_by"`
	DeletedBy   types.String              `tfsdk:"deleted_by"`
	Prefs       *UserGroupDataSourcePrefs `tfsdk:"prefs"`
	UserCount   types.Number              `tfsdk:"user_count"`
	Users       types.List                `tfsdk:"users"`
}

type UserGroupDataSourcePrefs struct {
	Channels types.List `tfsdk:"channels"`
	Groups   types.List `tfsdk:"groups"`
}

func NewUserGroupDataSource() datasource.DataSource {
	return &UserGroupDataSource{}
}

func (u *UserGroupDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, res *datasource.MetadataResponse) {
	res.TypeName = fmt.Sprintf("%s_usergroup", req.ProviderTypeName)
}

func (u *UserGroupDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, res *datasource.SchemaResponse) {
	res.Schema = schema.Schema{
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Required: true,
			},
			"team_id": schema.StringAttribute{
				Computed: true,
			},
			"is_user_group": schema.BoolAttribute{
				Computed: true,
			},
			"name": schema.StringAttribute{
				Computed: true,
			},
			"description": schema.StringAttribute{
				Computed: true,
			},
			"handle": schema.StringAttribute{
				Computed: true,
			},
			"is_external": schema.BoolAttribute{
				Computed: true,
			},
			"auto_type": schema.StringAttribute{
				Computed: true,
			},
			"created_by": schema.StringAttribute{
				Computed: true,
			},
			"updated_by": schema.StringAttribute{
				Computed: true,
			},
			"deleted_by": schema.StringAttribute{
				Computed: true,
			},
			"prefs": schema.SingleNestedAttribute{
				Computed: true,
				Attributes: map[string]schema.Attribute{
					"channels": schema.ListAttribute{
						ElementType: types.StringType,
						Computed:    true,
					},
					"groups": schema.ListAttribute{
						ElementType: types.StringType,
						Computed:    true,
					},
				},
			},
			"user_count": schema.NumberAttribute{
				Computed: true,
			},
			"users": schema.ListAttribute{
				ElementType: types.StringType,
				Computed:    true,
			},
		},
	}
}

func (u *UserGroupDataSource) Configure(ctx context.Context, req datasource.ConfigureRequest, res *datasource.ConfigureResponse) {
	if req.ProviderData == nil {
		return
	}
	u.client = req.ProviderData.(APIClient)
}

func (u *UserGroupDataSource) Read(ctx context.Context, req datasource.ReadRequest, res *datasource.ReadResponse) {
	var state UserGroupDataSourceState
	diags := req.Config.Get(ctx, &state)
	res.Diagnostics.Append(diags...)
	if res.Diagnostics.HasError() {
		return
	}
	userGroups, err := u.client.GetUserGroupsContext(ctx,
		slack.GetUserGroupsOptionIncludeCount(true),
		slack.GetUserGroupsOptionIncludeUsers(true),
		slack.GetUserGroupsOptionIncludeDisabled(true),
	)
	if err != nil {
		res.Diagnostics.AddError(
			fmt.Sprintf("the usergroup that has the id %s does not exist", state.TeamID.String()),
			err.Error(),
		)
	}
	var userGroup slack.UserGroup
	for _, ug := range userGroups {
		if ug.ID == state.ID.ValueString() {
			userGroup = ug
			break
		}
	}

	if userGroup.ID == "" {
		res.Diagnostics.AddError(
			fmt.Sprintf("the usergroup that has the id %s does not exist", state.ID.String()),
			"",
		)
	}

	channels := make([]attr.Value, 0, len(userGroup.Prefs.Channels))
	for _, channel := range userGroup.Prefs.Channels {
		channels = append(channels, types.StringValue(channel))
	}
	channelList, diags := types.ListValue(types.StringType, channels)
	res.Diagnostics.Append(diags...)
	if res.Diagnostics.HasError() {
		return
	}

	groups := make([]attr.Value, 0, len(userGroup.Prefs.Groups))
	for _, group := range userGroup.Prefs.Groups {
		groups = append(groups, types.StringValue(group))
	}
	groupList, diags := types.ListValue(types.StringType, groups)
	res.Diagnostics.Append(diags...)
	if res.Diagnostics.HasError() {
		return
	}

	users := make([]attr.Value, 0, len(userGroup.Users))
	for _, user := range userGroup.Users {
		users = append(users, types.StringValue(user))
	}
	userList, diags := types.ListValue(types.StringType, users)
	res.Diagnostics.Append(diags...)
	if res.Diagnostics.HasError() {
		return
	}

	state = UserGroupDataSourceState{
		ID:          types.StringValue(userGroup.ID),
		TeamID:      types.StringValue(userGroup.TeamID),
		IsUserGroup: types.BoolValue(userGroup.IsUserGroup),
		Name:        types.StringValue(userGroup.Name),
		Description: types.StringValue(userGroup.Description),
		Handle:      types.StringValue(userGroup.Handle),
		IsExternal:  types.BoolValue(userGroup.IsExternal),
		AutoType:    types.StringValue(userGroup.AutoType),
		CreatedBy:   types.StringValue(userGroup.CreatedBy),
		UpdatedBy:   types.StringValue(userGroup.UpdatedBy),
		DeletedBy:   types.StringValue(userGroup.DeletedBy),
		Prefs: &UserGroupDataSourcePrefs{
			Channels: channelList,
			Groups:   groupList,
		},
		UserCount: types.NumberValue(big.NewFloat(float64(userGroup.UserCount))),
		Users:     userList,
	}
	diags = res.State.Set(ctx, &state)
	res.Diagnostics.Append(diags...)
}
