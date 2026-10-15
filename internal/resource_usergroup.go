package internal

import (
	"context"
	"fmt"
	"strings"

	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/booldefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
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
		MarkdownDescription: "Manages a Slack user group. Slack has no API to delete a user group, so destroying " +
			"this resource disables the group, and creating a group with the name or handle of a disabled one " +
			"may fail with `name_already_exists` or `handle_already_exists`. Slack rejects an empty member list " +
			"(`no_users_provided`), so `users` cannot be changed to `[]`: set `enabled = false` to retire the " +
			"group or omit `users` to stop managing its members.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Computed:      true,
				Description:   "The ID of the user group, for example S0123456789.",
				PlanModifiers: []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
			},
			"name": schema.StringAttribute{
				Required:    true,
				Description: "The display name of the user group. Must be unique in the workspace.",
			},
			"channels": schema.ListAttribute{
				ElementType: types.StringType,
				Optional:    true,
				Description: "IDs of the default channels members of the group are added to. Replaces Slack's default channels entirely.",
			},
			"users": schema.ListAttribute{
				ElementType: types.StringType,
				Optional:    true,
				Description: "IDs of the members of the group. Replaces the whole member list; leave it unset to leave " +
					"membership unmanaged. Guests and bots cannot be members.",
			},
			"description": schema.StringAttribute{
				Optional:      true,
				Computed:      true,
				Description:   "A short description of the group. Slack reports an empty string when none is set.",
				PlanModifiers: []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
			},
			"handle": schema.StringAttribute{
				Optional: true,
				Computed: true,
				Description: "The mention handle of the group, unique across users, channels and user groups. " +
					"Slack generates one from the name when omitted.",
				PlanModifiers: []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
			},
			"team_id": schema.StringAttribute{
				Optional: true,
				Computed: true,
				Description: "The ID of the workspace the group belongs to. Only required for org-level (Enterprise Grid) " +
					"tokens. Changing it replaces the group.",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
					stringplanmodifier.RequiresReplaceIfConfigured(),
				},
			},
			"enabled": schema.BoolAttribute{
				Computed:    true,
				Optional:    true,
				Default:     booldefault.StaticBool(true),
				Description: "Whether the group is enabled. Setting this to false disables the group; Slack cannot delete it.",
			},
		},
	}
}

func (r *ResourceUserGroup) Configure(_ context.Context, req resource.ConfigureRequest, res *resource.ConfigureResponse) {
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
	r.client = client
}

func (r *ResourceUserGroup) ImportState(ctx context.Context, req resource.ImportStateRequest, res *resource.ImportStateResponse) {
	ug, found, err := r.findUserGroup(ctx, req.ID, "")
	if err != nil {
		res.Diagnostics.AddError("failed to list user groups", err.Error())
		return
	}
	if !found {
		res.Diagnostics.AddError(fmt.Sprintf("the usergroup that has the id %s does not exist", req.ID), "")
		return
	}

	state, diags := userGroupToState(ctx, ug, ResourceUserGroupState{}, true)
	res.Diagnostics.Append(diags...)
	if res.Diagnostics.HasError() {
		return
	}
	res.Diagnostics.Append(res.State.Set(ctx, &state)...)
}

func (r *ResourceUserGroup) Create(ctx context.Context, req resource.CreateRequest, res *resource.CreateResponse) {
	var plan ResourceUserGroupState
	res.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if res.Diagnostics.HasError() {
		return
	}

	channels, diags := stringsFromList(ctx, plan.Channels)
	res.Diagnostics.Append(diags...)
	users, diags := stringsFromList(ctx, plan.Users)
	res.Diagnostics.Append(diags...)
	if res.Diagnostics.HasError() {
		return
	}

	ug, err := r.client.CreateUserGroupContext(ctx, slack.UserGroup{
		Name:        plan.Name.ValueString(),
		Prefs:       slack.UserGroupPrefs{Channels: channels},
		Description: plan.Description.ValueString(),
		Handle:      plan.Handle.ValueString(),
		TeamID:      plan.TeamID.ValueString(),
	})
	if err != nil {
		res.Diagnostics.AddError("failed to create user group", err.Error())
		return
	}

	// The state is saved before the follow-up calls so a failure leaves the
	// group tracked (tainted) instead of orphaned in Slack.
	state := plan
	state.ID = types.StringValue(ug.ID)
	state.Handle = knownOr(plan.Handle, ug.Handle)
	state.Description = knownOr(plan.Description, ug.Description)
	state.TeamID = knownOr(plan.TeamID, ug.TeamID)
	res.Diagnostics.Append(res.State.Set(ctx, &state)...)
	if res.Diagnostics.HasError() {
		return
	}

	if len(users) > 0 {
		if _, err := r.client.UpdateUserGroupMembersContext(ctx, ug.ID, strings.Join(users, ",")); err != nil {
			res.Diagnostics.AddError("failed to update user group members", err.Error())
			return
		}
	}
	if !plan.Enabled.ValueBool() {
		if _, err := r.client.DisableUserGroupContext(ctx, ug.ID); err != nil {
			res.Diagnostics.AddError("failed to disable user group", err.Error())
			return
		}
	}
}

func (r *ResourceUserGroup) Read(ctx context.Context, req resource.ReadRequest, res *resource.ReadResponse) {
	var state ResourceUserGroupState
	res.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if res.Diagnostics.HasError() {
		return
	}

	ug, found, err := r.findUserGroup(ctx, state.ID.ValueString(), state.TeamID.ValueString())
	if err != nil {
		res.Diagnostics.AddError("failed to list user groups", err.Error())
		return
	}
	if !found {
		res.State.RemoveResource(ctx)
		return
	}

	newState, diags := userGroupToState(ctx, ug, state, !state.Users.IsNull())
	res.Diagnostics.Append(diags...)
	if res.Diagnostics.HasError() {
		return
	}
	res.Diagnostics.Append(res.State.Set(ctx, &newState)...)
}

func (r *ResourceUserGroup) Update(ctx context.Context, req resource.UpdateRequest, res *resource.UpdateResponse) {
	var plan, prior ResourceUserGroupState
	res.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	res.Diagnostics.Append(req.State.Get(ctx, &prior)...)
	if res.Diagnostics.HasError() {
		return
	}

	id := prior.ID.ValueString()
	enable := plan.Enabled.ValueBool() && !prior.Enabled.ValueBool()
	disable := !plan.Enabled.ValueBool() && prior.Enabled.ValueBool()

	channels, diags := stringsFromList(ctx, plan.Channels)
	res.Diagnostics.Append(diags...)
	users, diags := stringsFromList(ctx, plan.Users)
	res.Diagnostics.Append(diags...)
	if res.Diagnostics.HasError() {
		return
	}

	updateUsers := !plan.Users.IsNull() && !plan.Users.Equal(prior.Users)
	if updateUsers && len(users) == 0 {
		res.Diagnostics.AddAttributeError(path.Root("users"), "Cannot remove every member of the usergroup",
			"Slack cannot remove every member of a usergroup (no_users_provided). "+
				"Set enabled = false to retire the group or omit users to stop managing membership.")
		return
	}

	// A disabled group may not accept edits, so it is enabled before and
	// disabled after everything else.
	if enable {
		if _, err := r.client.EnableUserGroupContext(ctx, id); err != nil {
			res.Diagnostics.AddError("failed to enable user group", err.Error())
			return
		}
	}

	current := slack.UserGroup{
		Handle:      prior.Handle.ValueString(),
		Description: prior.Description.ValueString(),
		TeamID:      prior.TeamID.ValueString(),
	}
	var opts []slack.UpdateUserGroupsOption
	if !plan.Name.Equal(prior.Name) {
		opts = append(opts, slack.UpdateUserGroupsOptionName(plan.Name.ValueString()))
	}
	if !plan.Handle.IsUnknown() && !plan.Handle.Equal(prior.Handle) {
		opts = append(opts, slack.UpdateUserGroupsOptionHandle(plan.Handle.ValueString()))
	}
	if !plan.Description.IsUnknown() && !plan.Description.Equal(prior.Description) {
		opts = append(opts, slack.UpdateUserGroupsOptionDescription(plan.Description.ValueStringPointer()))
	}
	if !plan.Channels.Equal(prior.Channels) {
		if channels == nil {
			channels = []string{}
		}
		opts = append(opts, slack.UpdateUserGroupsOptionChannels(channels))
	}
	if len(opts) > 0 {
		updated, err := r.client.UpdateUserGroupContext(ctx, id, opts...)
		if err != nil {
			res.Diagnostics.AddError("failed to update user group", err.Error())
			return
		}
		current = updated
	}

	if updateUsers {
		if _, err := r.client.UpdateUserGroupMembersContext(ctx, id, strings.Join(users, ",")); err != nil {
			res.Diagnostics.AddError("failed to update user group members", err.Error())
			return
		}
	}

	if disable {
		if _, err := r.client.DisableUserGroupContext(ctx, id); err != nil {
			res.Diagnostics.AddError("failed to disable user group", err.Error())
			return
		}
	}

	state := plan
	state.ID = prior.ID
	state.Handle = knownOr(plan.Handle, current.Handle)
	state.Description = knownOr(plan.Description, current.Description)
	state.TeamID = knownOr(plan.TeamID, current.TeamID)
	res.Diagnostics.Append(res.State.Set(ctx, &state)...)
}

func (r *ResourceUserGroup) Delete(ctx context.Context, req resource.DeleteRequest, res *resource.DeleteResponse) {
	var state ResourceUserGroupState
	res.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if res.Diagnostics.HasError() {
		return
	}

	// Disabling is the only teardown Slack offers, so a disabled group is already done.
	if !state.Enabled.ValueBool() {
		return
	}
	if _, err := r.client.DisableUserGroupContext(ctx, state.ID.ValueString()); err != nil {
		res.Diagnostics.AddError("failed to disable user group", err.Error())
	}
}

// findUserGroup looks the group up in usergroups.list, which includes disabled
// groups and their members. teamID is only sent when non-empty.
func (r *ResourceUserGroup) findUserGroup(ctx context.Context, id, teamID string) (slack.UserGroup, bool, error) {
	opts := []slack.GetUserGroupsOption{
		slack.GetUserGroupsOptionIncludeUsers(true),
		slack.GetUserGroupsOptionIncludeCount(true),
		slack.GetUserGroupsOptionIncludeDisabled(true),
	}
	if teamID != "" {
		opts = append(opts, slack.GetUserGroupsOptionWithTeamID(teamID))
	}
	userGroups, err := r.client.GetUserGroupsContext(ctx, opts...)
	if err != nil {
		return slack.UserGroup{}, false, err
	}
	for _, ug := range userGroups {
		if ug.ID == id {
			return ug, true, nil
		}
	}
	return slack.UserGroup{}, false, nil
}

// userGroupToState builds the resource state from ug, ordering channels and
// users like prior.
//
// Channels and users stay null when Slack reports none and prior was null,
// since an empty list never matches an omitted attribute. Users are only
// refreshed when refreshUsers is set, so an unmanaged membership stays null.
func userGroupToState(ctx context.Context, ug slack.UserGroup, prior ResourceUserGroupState, refreshUsers bool) (ResourceUserGroupState, diag.Diagnostics) {
	var diags diag.Diagnostics

	channels, d := refreshedList(ctx, prior.Channels, ug.Prefs.Channels)
	diags.Append(d...)
	users := prior.Users
	if refreshUsers {
		users, d = refreshedList(ctx, prior.Users, ug.Users)
		diags.Append(d...)
	}

	return ResourceUserGroupState{
		ID:          types.StringValue(ug.ID),
		Name:        types.StringValue(ug.Name),
		Channels:    channels,
		Users:       users,
		Description: types.StringValue(ug.Description),
		Handle:      types.StringValue(ug.Handle),
		TeamID:      types.StringValue(ug.TeamID),
		Enabled:     types.BoolValue(ug.DateDelete == 0),
	}, diags
}

func refreshedList(ctx context.Context, prior types.List, remote []string) (types.List, diag.Diagnostics) {
	if prior.IsNull() && len(remote) == 0 {
		return types.ListNull(types.StringType), nil
	}
	priorValues, diags := stringsFromList(ctx, prior)
	if diags.HasError() {
		return types.ListNull(types.StringType), diags
	}
	list, d := types.ListValueFrom(ctx, types.StringType, mergeOrder(priorValues, remote))
	diags.Append(d...)
	return list, diags
}

// mergeOrder returns remote ordered like prior: elements of prior that are
// still in remote keep their position and new remote elements follow.
//
// Slack returns members and channels in its own order, which would otherwise
// show up as a diff against the order written in the configuration.
func mergeOrder(prior, remote []string) []string {
	inRemote := make(map[string]struct{}, len(remote))
	for _, v := range remote {
		inRemote[v] = struct{}{}
	}

	merged := make([]string, 0, len(remote))
	seen := make(map[string]struct{}, len(remote))
	for _, v := range prior {
		if _, ok := inRemote[v]; !ok {
			continue
		}
		if _, ok := seen[v]; ok {
			continue
		}
		seen[v] = struct{}{}
		merged = append(merged, v)
	}
	for _, v := range remote {
		if _, ok := seen[v]; ok {
			continue
		}
		seen[v] = struct{}{}
		merged = append(merged, v)
	}
	return merged
}

// knownOr returns v when it holds a value and fallback otherwise.
//
// Configured values win over Slack's response because Terraform rejects a
// result that differs from the plan; a later refresh reports any difference.
func knownOr(v types.String, fallback string) types.String {
	if v.IsNull() || v.IsUnknown() {
		return types.StringValue(fallback)
	}
	return v
}
