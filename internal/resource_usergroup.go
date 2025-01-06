package internal

import (
	"context"
	"fmt"
	"strings"

	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/booldefault"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/slack-go/slack"
)

var (
	_ resource.Resource                = &UserGroupResource{}
	_ resource.ResourceWithImportState = &UserGroupResource{}
	_ resource.ResourceWithConfigure   = &UserGroupResource{}
)

type UserGroupResource struct {
	client APIClient
}

type UserGroupResourceState struct {
	ID          types.String `tfsdk:"id"`
	Name        types.String `tfsdk:"name"`
	Channels    types.List   `tfsdk:"channels"`
	Users       types.List   `tfsdk:"users"`
	Description types.String `tfsdk:"description"`
	Handle      types.String `tfsdk:"handle"`
	TeamID      types.String `tfsdk:"team_id"`
	Enabled     types.Bool   `tfsdk:"enabled"`
}

func NewUserGroupResource() resource.Resource {
	return &UserGroupResource{}
}

func (u *UserGroupResource) Metadata(_ context.Context, req resource.MetadataRequest, res *resource.MetadataResponse) {
	res.TypeName = fmt.Sprintf("%s_usergroup", req.ProviderTypeName)
}

func (u *UserGroupResource) Schema(_ context.Context, _ resource.SchemaRequest, res *resource.SchemaResponse) {
	res.Schema = schema.Schema{
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Computed: true,
			},
			"name": schema.StringAttribute{
				Required: true,
			},
			"channels": schema.ListAttribute{
				ElementType: types.StringType,
				Optional:    true,
			},
			"users": schema.ListAttribute{
				ElementType: types.StringType,
				Optional:    true,
			},
			"description": schema.StringAttribute{
				Optional: true,
			},
			"handle": schema.StringAttribute{
				Optional: true,
			},
			"team_id": schema.StringAttribute{
				Optional: true,
			},
			"enabled": schema.BoolAttribute{
				Computed: true,
				Optional: true,
				Default:  booldefault.StaticBool(true),
			},
		},
	}
}

func (u *UserGroupResource) ImportState(ctx context.Context, req resource.ImportStateRequest, res *resource.ImportStateResponse) {
	userGroups, err := u.client.GetUserGroupsContext(ctx,
		slack.GetUserGroupsOptionIncludeUsers(true),
		slack.GetUserGroupsOptionIncludeCount(true),
		slack.GetUserGroupsOptionIncludeDisabled(true),
	)
	if err != nil {
		res.Diagnostics.AddError(
			fmt.Sprintf("the usergroup that has the id %s does not exist", req.ID),
			err.Error(),
		)
	}

	var userGroup slack.UserGroup
	for _, ug := range userGroups {
		if userGroup.ID == req.ID {
			userGroup = ug
			break
		}
	}

	if userGroup.ID == "" {
		res.Diagnostics.AddError(
			fmt.Sprintf("the usergroup that has the id %s does not exist", req.ID),
			"",
		)
		return
	}

	channels := make([]attr.Value, 0, len(userGroup.Prefs.Channels))
	for _, channel := range userGroup.Prefs.Channels {
		channels = append(channels, types.StringValue(channel))
	}
	channelList, diags := types.ListValue(types.StringType, channels)
	res.Diagnostics.Append(diags...)

	users := make([]attr.Value, 0, len(userGroup.Users))
	for _, user := range userGroup.Users {
		users = append(users, types.StringValue(user))
	}
	userList, diags := types.ListValue(types.StringType, users)

	state := UserGroupResourceState{
		ID:          types.StringValue(userGroup.ID),
		Name:        types.StringValue(userGroup.Name),
		Channels:    channelList,
		Users:       userList,
		Description: types.StringValue(userGroup.Description),
		Handle:      types.StringValue(userGroup.Handle),
		TeamID:      types.StringValue(userGroup.TeamID),
	}
	diags = res.State.Set(ctx, &state)
	res.Diagnostics.Append(diags...)
}

func (u *UserGroupResource) Configure(ctx context.Context, req resource.ConfigureRequest, res *resource.ConfigureResponse) {
	if req.ProviderData == nil {
		return
	}
	u.client = req.ProviderData.(APIClient)
}

func (u *UserGroupResource) Create(ctx context.Context, req resource.CreateRequest, res *resource.CreateResponse) {
	var state UserGroupResourceState
	diags := req.Plan.Get(ctx, &state)
	res.Diagnostics.Append(diags...)
	if res.Diagnostics.HasError() {
		return
	}

	channels := make([]string, 0, len(state.Channels.Elements()))
	for _, channel := range state.Channels.Elements() {
		var str string
		val, err := channel.ToTerraformValue(ctx)
		if err != nil {
			res.Diagnostics.AddError("failed to convert channel to terraform value", err.Error())
			return
		}
		if err := val.As(&str); err != nil {
			res.Diagnostics.AddError("failed to convert channel to string", err.Error())
			return
		}
		channels = append(channels, str)
	}

	users := make([]string, 0, len(state.Users.Elements()))
	for _, user := range state.Users.Elements() {
		var str string
		val, err := user.ToTerraformValue(ctx)
		if err != nil {
			res.Diagnostics.AddError("failed to convert user to terraform value", err.Error())
			return
		}
		if err := val.As(&str); err != nil {
			res.Diagnostics.AddError("failed to convert user to string", err.Error())
			return
		}
		users = append(users, str)
	}

	userGroup, err := u.client.CreateUserGroupContext(ctx, slack.UserGroup{
		Name: state.Name.ValueString(),
		Prefs: slack.UserGroupPrefs{
			Channels: channels,
		},
		Users:       users,
		Description: state.Description.ValueString(),
		Handle:      state.Handle.ValueString(),
		TeamID:      state.TeamID.ValueString(),
	})
	if err != nil {
		res.Diagnostics.AddError("failed to create user group", err.Error())
		return
	}

	if !state.Enabled.ValueBool() {
		if _, err := u.client.DisableUserGroupContext(ctx, userGroup.ID); err != nil {
			res.Diagnostics.AddError("failed to disable user group", err.Error())
			return
		}
	}

	stateChannels := make([]attr.Value, 0, len(userGroup.Prefs.Channels))
	for _, channel := range userGroup.Prefs.Channels {
		stateChannels = append(stateChannels, types.StringValue(channel))
	}
	stateChannelList, diags := types.ListValue(types.StringType, stateChannels)
	res.Diagnostics.Append(diags...)
	if res.Diagnostics.HasError() {
		return
	}

	stateUsers := make([]attr.Value, 0, len(userGroup.Users))
	for _, user := range userGroup.Users {
		stateUsers = append(stateUsers, types.StringValue(user))
	}
	stateUserList, diags := types.ListValue(types.StringType, stateUsers)
	res.Diagnostics.Append(diags...)
	if res.Diagnostics.HasError() {
		return
	}

	state = UserGroupResourceState{
		ID:          types.StringValue(userGroup.ID),
		Name:        types.StringValue(userGroup.Name),
		Channels:    stateChannelList,
		Users:       stateUserList,
		Description: types.StringValue(userGroup.Description),
		Handle:      types.StringValue(userGroup.Handle),
		TeamID:      types.StringValue(userGroup.TeamID),
		Enabled:     state.Enabled,
	}

	diags = res.State.Set(ctx, &state)
	res.Diagnostics.Append(diags...)
	if res.Diagnostics.HasError() {
		return
	}
}

func (u *UserGroupResource) Read(ctx context.Context, req resource.ReadRequest, res *resource.ReadResponse) {
	var state UserGroupResourceState
	diags := req.State.Get(ctx, &state)
	res.Diagnostics.Append(diags...)
	if res.Diagnostics.HasError() {
		return
	}
	diags = res.State.Set(ctx, &state)
	res.Diagnostics.Append(diags...)
	if res.Diagnostics.HasError() {
		return
	}
}

func (u *UserGroupResource) Update(ctx context.Context, req resource.UpdateRequest, res *resource.UpdateResponse) {
	var state UserGroupResourceState
	diags := req.Plan.Get(ctx, &state)
	res.Diagnostics.Append(diags...)
	if res.Diagnostics.HasError() {
		return
	}

	channels := make([]string, 0, len(state.Channels.Elements()))
	for _, channel := range state.Channels.Elements() {
		var str string
		val, err := channel.ToTerraformValue(ctx)
		if err != nil {
			res.Diagnostics.AddError("failed to convert channel to terraform value", err.Error())
			return
		}
		if err := val.As(&str); err != nil {
			res.Diagnostics.AddError("failed to convert channel to string", err.Error())
			return
		}
		channels = append(channels, str)
	}

	if _, err := u.client.UpdateUserGroupContext(ctx, state.ID.ValueString(), slack.UpdateUserGroupsOptionName(state.Name.ValueString()),
		slack.UpdateUserGroupsOptionHandle(state.Handle.ValueString()),
		slack.UpdateUserGroupsOptionChannels(channels),
		slack.UpdateUserGroupsOptionDescription(state.Description.ValueStringPointer()),
	); err != nil {
		res.Diagnostics.AddError("failed to update user group", err.Error())
		return
	}

	users := make([]string, 0, len(state.Users.Elements()))
	for _, user := range state.Users.Elements() {
		var str string
		val, err := user.ToTerraformValue(ctx)
		if err != nil {
			res.Diagnostics.AddError("failed to convert user to terraform value", err.Error())
			return
		}
		if err := val.As(&str); err != nil {
			res.Diagnostics.AddError("failed to convert user to string", err.Error())
			return
		}
		users = append(users, str)
	}
	stringedUsers := strings.Join(users, ",")

	userGroup, err := u.client.UpdateUserGroupMembersContext(ctx, state.ID.ValueString(), stringedUsers)
	if err != nil {
		res.Diagnostics.AddError("failed to update user group members", err.Error())
		return
	}

	stateChannels := make([]attr.Value, 0, len(userGroup.Prefs.Channels))
	for _, channel := range userGroup.Prefs.Channels {
		stateChannels = append(stateChannels, types.StringValue(channel))
	}
	stateChannelList, diags := types.ListValue(types.StringType, stateChannels)
	res.Diagnostics.Append(diags...)
	if res.Diagnostics.HasError() {
		return
	}

	stateUsers := make([]attr.Value, 0, len(userGroup.Users))
	for _, user := range userGroup.Users {
		stateUsers = append(stateUsers, types.StringValue(user))
	}
	stateUserList, diags := types.ListValue(types.StringType, stateUsers)
	res.Diagnostics.Append(diags...)
	if res.Diagnostics.HasError() {
		return
	}

	if state.Enabled.ValueBool() {
		if _, err := u.client.EnableUserGroupContext(ctx, userGroup.ID); err != nil {
			res.Diagnostics.AddError("failed to enable user group", err.Error())
			return
		}
	} else {
		if _, err := u.client.DisableUserGroupContext(ctx, userGroup.ID); err != nil {
			res.Diagnostics.AddError("failed to disable user group", err.Error())
			return
		}
	}

	state = UserGroupResourceState{
		ID:          types.StringValue(userGroup.ID),
		Name:        types.StringValue(userGroup.Name),
		Channels:    stateChannelList,
		Users:       stateUserList,
		Description: types.StringValue(userGroup.Description),
		Handle:      types.StringValue(userGroup.Handle),
		TeamID:      types.StringValue(userGroup.TeamID),
		Enabled:     state.Enabled,
	}
}

func (u *UserGroupResource) Delete(ctx context.Context, req resource.DeleteRequest, res *resource.DeleteResponse) {
	var state UserGroupResourceState
	diags := req.State.Get(ctx, &state)
	res.Diagnostics.Append(diags...)
	if res.Diagnostics.HasError() {
		return
	}

	if _, err := u.client.DisableUserGroupContext(ctx, state.ID.ValueString()); err != nil {
		res.Diagnostics.AddError("failed to delete user group", err.Error())
		return
	}
	res.State.RemoveResource(ctx)
}
