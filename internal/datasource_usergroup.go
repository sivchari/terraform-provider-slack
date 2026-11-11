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
	_ datasource.DataSource              = &DataSourceUserGroup{}
	_ datasource.DataSourceWithConfigure = &DataSourceUserGroup{}

	_ datasource.DataSourceWithConfigValidators = &DataSourceUserGroup{}
)

type DataSourceUserGroup struct {
	client APIClient
}

type DataSourceUserGroupState struct {
	ID          types.String              `tfsdk:"id"`
	TeamID      types.String              `tfsdk:"team_id"`
	IsUserGroup types.Bool                `tfsdk:"is_user_group"`
	Name        types.String              `tfsdk:"name"`
	Description types.String              `tfsdk:"description"`
	Handle      types.String              `tfsdk:"handle"`
	Enabled     types.Bool                `tfsdk:"enabled"`
	IsExternal  types.Bool                `tfsdk:"is_external"`
	AutoType    types.String              `tfsdk:"auto_type"`
	CreatedBy   types.String              `tfsdk:"created_by"`
	UpdatedBy   types.String              `tfsdk:"updated_by"`
	DeletedBy   types.String              `tfsdk:"deleted_by"`
	Prefs       *DataSourceUserGroupPrefs `tfsdk:"prefs"`
	UserCount   types.Number              `tfsdk:"user_count"`
	Users       types.List                `tfsdk:"users"`
}

type DataSourceUserGroupPrefs struct {
	Channels types.List `tfsdk:"channels"`
	Groups   types.List `tfsdk:"groups"`
}

func NewDataSourceUserGroup() datasource.DataSource {
	return &DataSourceUserGroup{}
}

func (d *DataSourceUserGroup) Metadata(_ context.Context, req datasource.MetadataRequest, res *datasource.MetadataResponse) {
	res.TypeName = fmt.Sprintf("%s_usergroup", req.ProviderTypeName)
}

func (d *DataSourceUserGroup) Schema(_ context.Context, _ datasource.SchemaRequest, res *datasource.SchemaResponse) {
	res.Schema = schema.Schema{
		Description: "Looks up a Slack user group by ID, name or handle. Exactly one of id, name or handle must be set. " +
			"Disabled user groups are found too; check enabled.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Optional:    true,
				Computed:    true,
				Description: "ID of the user group, for example S0123456789. Conflicts with name and handle.",
			},
			"name": schema.StringAttribute{
				Optional:    true,
				Computed:    true,
				Description: "Name of the user group. Conflicts with id and handle.",
			},
			"handle": schema.StringAttribute{
				Optional:    true,
				Computed:    true,
				Description: "Handle of the user group, the part after the @ used to mention it. Conflicts with id and name.",
			},
			"enabled": schema.BoolAttribute{
				Computed:    true,
				Description: "Whether the user group is enabled. A disabled group has a deletion date.",
			},
			"team_id": schema.StringAttribute{
				Computed:    true,
				Description: "ID of the workspace the user group belongs to.",
			},
			"is_user_group": schema.BoolAttribute{
				Computed:    true,
				Description: "Whether the object is a user group.",
			},
			"description": schema.StringAttribute{
				Computed:    true,
				Description: "Description of the user group.",
			},
			"is_external": schema.BoolAttribute{
				Computed:    true,
				Description: "Whether the user group is shared from another workspace.",
			},
			"auto_type": schema.StringAttribute{
				Computed:    true,
				Description: "Kind of automatic user group, such as admin or owner. Empty for a custom user group.",
			},
			"created_by": schema.StringAttribute{
				Computed:    true,
				Description: "ID of the user who created the user group.",
			},
			"updated_by": schema.StringAttribute{
				Computed:    true,
				Description: "ID of the user who last updated the user group.",
			},
			"deleted_by": schema.StringAttribute{
				Computed:    true,
				Description: "ID of the user who disabled the user group. Empty while it is enabled.",
			},
			"prefs": schema.SingleNestedAttribute{
				Computed:    true,
				Description: "Default channels and groups the members of the user group are added to.",
				Attributes: map[string]schema.Attribute{
					"channels": schema.ListAttribute{
						ElementType: types.StringType,
						Computed:    true,
						Description: "IDs of the channels every member of the user group joins.",
					},
					"groups": schema.ListAttribute{
						ElementType: types.StringType,
						Computed:    true,
						Description: "IDs of the private channels every member of the user group joins.",
					},
				},
			},
			"user_count": schema.NumberAttribute{
				Computed:    true,
				Description: "Number of users in the user group.",
			},
			"users": schema.ListAttribute{
				ElementType: types.StringType,
				Computed:    true,
				Description: "IDs of the users in the user group.",
			},
		},
	}
}

func (d *DataSourceUserGroup) ConfigValidators(context.Context) []datasource.ConfigValidator {
	return []datasource.ConfigValidator{exactlyOneOf("id", "name", "handle")}
}

func (d *DataSourceUserGroup) Configure(ctx context.Context, req datasource.ConfigureRequest, res *datasource.ConfigureResponse) {
	if req.ProviderData == nil {
		return
	}
	client := req.ProviderData.(APIClient)
	if !client.HasBotToken() {
		res.Diagnostics.AddError(
			"Missing bot token",
			`The provider attribute "token" is required to use slack_usergroup. Set token in the provider configuration.`,
		)
		return
	}
	d.client = client
}

func (d *DataSourceUserGroup) Read(ctx context.Context, req datasource.ReadRequest, res *datasource.ReadResponse) {
	var state DataSourceUserGroupState
	diags := req.Config.Get(ctx, &state)
	res.Diagnostics.Append(diags...)
	if res.Diagnostics.HasError() {
		return
	}
	userGroups, err := d.client.GetUserGroupsContext(ctx,
		slack.GetUserGroupsOptionIncludeCount(true),
		slack.GetUserGroupsOptionIncludeUsers(true),
		slack.GetUserGroupsOptionIncludeDisabled(true),
	)
	if err != nil {
		res.Diagnostics.AddError("Failed to list the Slack user groups", err.Error())
		return
	}
	key, value, field := userGroupLookup(state)
	userGroup, found := matchUserGroup(userGroups, value, field)
	if !found {
		res.Diagnostics.AddError(
			"Usergroup not found",
			fmt.Sprintf("No user group has the %s %q.", key, value),
		)
		return
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

	state = DataSourceUserGroupState{
		ID:          types.StringValue(userGroup.ID),
		TeamID:      types.StringValue(userGroup.TeamID),
		IsUserGroup: types.BoolValue(userGroup.IsUserGroup),
		Name:        types.StringValue(userGroup.Name),
		Description: types.StringValue(userGroup.Description),
		Handle:      types.StringValue(userGroup.Handle),
		Enabled:     types.BoolValue(userGroup.DateDelete == 0),
		IsExternal:  types.BoolValue(userGroup.IsExternal),
		AutoType:    types.StringValue(userGroup.AutoType),
		CreatedBy:   types.StringValue(userGroup.CreatedBy),
		UpdatedBy:   types.StringValue(userGroup.UpdatedBy),
		DeletedBy:   types.StringValue(userGroup.DeletedBy),
		Prefs: &DataSourceUserGroupPrefs{
			Channels: channelList,
			Groups:   groupList,
		},
		UserCount: types.NumberValue(big.NewFloat(float64(userGroup.UserCount))),
		Users:     userList,
	}
	diags = res.State.Set(ctx, &state)
	res.Diagnostics.Append(diags...)
}

// userGroupLookup returns the lookup key set in the configuration, its value
// and the user group field it is compared with.
func userGroupLookup(state DataSourceUserGroupState) (key, value string, field func(slack.UserGroup) string) {
	switch {
	case !state.ID.IsNull():
		return "id", state.ID.ValueString(), func(ug slack.UserGroup) string { return ug.ID }
	case !state.Name.IsNull():
		return "name", state.Name.ValueString(), func(ug slack.UserGroup) string { return ug.Name }
	default:
		return "handle", state.Handle.ValueString(), func(ug slack.UserGroup) string { return ug.Handle }
	}
}

// matchUserGroup returns the user group whose field equals value, preferring an
// enabled one because a disabled group may keep a name or handle that was
// reused later.
func matchUserGroup(userGroups []slack.UserGroup, value string, field func(slack.UserGroup) string) (slack.UserGroup, bool) {
	var disabled *slack.UserGroup
	for i := range userGroups {
		if field(userGroups[i]) != value {
			continue
		}
		if userGroups[i].DateDelete == 0 {
			return userGroups[i], true
		}
		if disabled == nil {
			disabled = &userGroups[i]
		}
	}
	if disabled != nil {
		return *disabled, true
	}
	return slack.UserGroup{}, false
}
