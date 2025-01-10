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
	_ resource.Resource                = &ResourceUserGroup{}
	_ resource.ResourceWithImportState = &ResourceUserGroup{}
	_ resource.ResourceWithConfigure   = &ResourceUserGroup{}
)

type ResourceUserGroup struct {
	client APIClient
}

type ResourceUserGroupState struct {
	ID          types.String `tfsdk:"id"`
	Name        types.String `tfsdk:"name"`
	Channels    types.List   `tfsdk:"channels"`
	Users       types.List   `tfsdk:"users"`
	Description types.String `tfsdk:"description"`
	Handle      types.String `tfsdk:"handle"`
	TeamID      types.String `tfsdk:"team_id"`
	Enabled     types.Bool   `tfsdk:"enabled"`
}

func NewResourceUserGroup() resource.Resource {
	return &ResourceUserGroup{}
}

func (r *ResourceUserGroup) Metadata(_ context.Context, req resource.MetadataRequest, res *resource.MetadataResponse) {
	res.TypeName = fmt.Sprintf("%s_usergroup", req.ProviderTypeName)
}

func (r *ResourceUserGroup) Schema(_ context.Context, _ resource.SchemaRequest, res *resource.SchemaResponse) {
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

func (r *ResourceUserGroup) ImportState(ctx context.Context, req resource.ImportStateRequest, res *resource.ImportStateResponse) {
	userGroups, err := r.client.GetUserGroupsContext(ctx,
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

	state := ResourceUserGroupState{
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

func (r *ResourceUserGroup) Configure(ctx context.Context, req resource.ConfigureRequest, res *resource.ConfigureResponse) {
	if req.ProviderData == nil {
		return
	}
	r.client = req.ProviderData.(APIClient)
}

func (r *ResourceUserGroup) Create(ctx context.Context, req resource.CreateRequest, res *resource.CreateResponse) {
	var plan ResourceUserGroupState
	diags := req.Plan.Get(ctx, &plan)
	res.Diagnostics.Append(diags...)
	if res.Diagnostics.HasError() {
		return
	}

	channels := make([]string, 0, len(plan.Channels.Elements()))
	for _, channel := range plan.Channels.Elements() {
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

	userGroup, err := r.client.CreateUserGroupContext(ctx, slack.UserGroup{
		Name: plan.Name.ValueString(),
		Prefs: slack.UserGroupPrefs{
			Channels: channels,
		},
		Description: plan.Description.ValueString(),
		Handle:      plan.Handle.ValueString(),
		TeamID:      plan.TeamID.ValueString(),
	})
	if err != nil {
		res.Diagnostics.AddError("failed to create user group", err.Error())
		return
	}

	if !plan.Enabled.ValueBool() {
		if _, err := r.client.DisableUserGroupContext(ctx, userGroup.ID); err != nil {
			res.Diagnostics.AddError("failed to disable user group", err.Error())
			return
		}
		// If the user group is disabled, we don't need to update the users
		return
	}

	users := make([]string, 0, len(plan.Users.Elements()))
	for _, user := range plan.Users.Elements() {
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
	userGroup, err = r.client.UpdateUserGroupMembersContext(ctx, userGroup.ID, stringedUsers)
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

	state := ResourceUserGroupState{
		ID:          types.StringValue(userGroup.ID),
		Name:        types.StringValue(userGroup.Name),
		Channels:    stateChannelList,
		Users:       stateUserList,
		Description: types.StringValue(userGroup.Description),
		Handle:      types.StringValue(userGroup.Handle),
		TeamID:      types.StringValue(userGroup.TeamID),
		Enabled:     plan.Enabled,
	}

	diags = res.State.Set(ctx, &state)
	res.Diagnostics.Append(diags...)
}

func (r *ResourceUserGroup) Read(ctx context.Context, req resource.ReadRequest, res *resource.ReadResponse) {
	var state ResourceUserGroupState
	diags := req.State.Get(ctx, &state)
	res.Diagnostics.Append(diags...)
	if res.Diagnostics.HasError() {
		return
	}
	diags = res.State.Set(ctx, &state)
	res.Diagnostics.Append(diags...)
}

func (r *ResourceUserGroup) Update(ctx context.Context, req resource.UpdateRequest, res *resource.UpdateResponse) {
	var plan ResourceUserGroupState
	diags := req.Plan.Get(ctx, &plan)
	res.Diagnostics.Append(diags...)
	if res.Diagnostics.HasError() {
		return
	}

	if plan.Enabled.ValueBool() {
		if _, err := r.client.EnableUserGroupContext(ctx, plan.ID.ValueString()); err != nil {
			res.Diagnostics.AddError("failed to enable user group", err.Error())
			return
		}
	} else {
		if _, err := r.client.DisableUserGroupContext(ctx, plan.ID.ValueString()); err != nil {
			res.Diagnostics.AddError("failed to disable user group", err.Error())
			return
		}
		// If the user group is disabled, we don't need following process anymore
		return
	}

	channels := make([]string, 0, len(plan.Channels.Elements()))
	for _, channel := range plan.Channels.Elements() {
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

	if _, err := r.client.UpdateUserGroupContext(ctx, plan.ID.ValueString(), slack.UpdateUserGroupsOptionName(plan.Name.ValueString()),
		slack.UpdateUserGroupsOptionHandle(plan.Handle.ValueString()),
		slack.UpdateUserGroupsOptionChannels(channels),
		slack.UpdateUserGroupsOptionDescription(plan.Description.ValueStringPointer()),
	); err != nil {
		res.Diagnostics.AddError("failed to update user group", err.Error())
		return
	}

	users := make([]string, 0, len(plan.Users.Elements()))
	for _, user := range plan.Users.Elements() {
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

	userGroup, err := r.client.UpdateUserGroupMembersContext(ctx, plan.ID.ValueString(), stringedUsers)
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

	state := ResourceUserGroupState{
		ID:          types.StringValue(userGroup.ID),
		Name:        types.StringValue(userGroup.Name),
		Channels:    stateChannelList,
		Users:       stateUserList,
		Description: types.StringValue(userGroup.Description),
		Handle:      types.StringValue(userGroup.Handle),
		TeamID:      types.StringValue(userGroup.TeamID),
		Enabled:     plan.Enabled,
	}

	diags = res.State.Set(ctx, &state)
	res.Diagnostics.Append(diags...)
}

func (r *ResourceUserGroup) Delete(ctx context.Context, req resource.DeleteRequest, res *resource.DeleteResponse) {
	var state ResourceUserGroupState
	diags := req.State.Get(ctx, &state)
	res.Diagnostics.Append(diags...)
	if res.Diagnostics.HasError() {
		return
	}

	if _, err := r.client.DisableUserGroupContext(ctx, state.ID.ValueString()); err != nil {
		res.Diagnostics.AddError("failed to delete user group", err.Error())
		return
	}
}
