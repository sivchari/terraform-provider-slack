package internal

import (
	"context"
	"fmt"
	"sort"

	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/booldefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringdefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/slack-go/slack"
)

const (
	actionOnDestroyArchive = "archive"
	actionOnDestroyNone    = "none"

	privacyImmutableDetail = "Slack has no API to convert channel privacy between public and private. " +
		"Recreating the channel fails with name_taken because an archived channel keeps its name. " +
		"Convert the channel to the desired privacy in Slack, then re-import it with terraform import."
)

var (
	_ resource.Resource                = &ResourceConversation{}
	_ resource.ResourceWithImportState = &ResourceConversation{}
	_ resource.ResourceWithConfigure   = &ResourceConversation{}
)

type ResourceConversation struct {
	client APIClient
}

type ResourceConversationState struct {
	ID                   types.String `tfsdk:"id"`
	Name                 types.String `tfsdk:"name"`
	Topic                types.String `tfsdk:"topic"`
	Purpose              types.String `tfsdk:"purpose"`
	IsPrivate            types.Bool   `tfsdk:"is_private"`
	Members              types.List   `tfsdk:"members"`
	ActionOnDestroy      types.String `tfsdk:"action_on_destroy"`
	AdoptExistingChannel types.Bool   `tfsdk:"adopt_existing_channel"`
}

func NewResourceConversation() resource.Resource {
	return &ResourceConversation{}
}

func (r *ResourceConversation) Metadata(_ context.Context, req resource.MetadataRequest, res *resource.MetadataResponse) {
	res.TypeName = fmt.Sprintf("%s_conversation", req.ProviderTypeName)
}

func (r *ResourceConversation) Schema(_ context.Context, _ resource.SchemaRequest, res *resource.SchemaResponse) {
	res.Schema = schema.Schema{
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Computed:      true,
				Description:   "The Slack channel ID, for example C0123456789.",
				PlanModifiers: []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
			},
			"name": schema.StringAttribute{
				Required: true,
				Description: "The channel name: lowercase letters, numbers, hyphens and underscores, up to 80 characters. " +
					"Changing it renames the channel.",
			},
			"topic": schema.StringAttribute{
				Optional:    true,
				Description: "The channel topic. Removing it from the configuration clears the topic in Slack.",
			},
			"purpose": schema.StringAttribute{
				Optional:    true,
				Description: "The channel purpose. Removing it from the configuration clears the purpose in Slack.",
			},
			"is_private": schema.BoolAttribute{
				Computed: true,
				Optional: true,
				Default:  booldefault.StaticBool(false),
				Description: "Whether the channel is private. Slack has no API to convert a channel between public and " +
					"private, so changing this on an existing channel is rejected.",
				PlanModifiers: []planmodifier.Bool{immutableBool(privacyImmutableDetail)},
			},
			"members": schema.ListAttribute{
				Optional:    true,
				ElementType: types.StringType,
				Description: "User IDs of the channel members. When set, the channel is made to contain exactly these users: " +
					"missing users are invited and other users are removed. The bot that owns the provider token is never " +
					"invited or removed. When unset, membership is not managed.",
			},
			"action_on_destroy": schema.StringAttribute{
				Optional: true,
				Computed: true,
				Default:  stringdefault.StaticString(actionOnDestroyArchive),
				Description: `What destroying the resource does: "archive" (default) archives the channel and "none" leaves ` +
					"it untouched in Slack and only removes it from state. Slack has no API to delete a channel, and " +
					"unarchiving requires a user token, so archived channels cannot be restored by this provider.",
				Validators: []validator.String{oneOfString(actionOnDestroyArchive, actionOnDestroyNone)},
			},
			"adopt_existing_channel": schema.BoolAttribute{
				Optional: true,
				Computed: true,
				Default:  booldefault.StaticBool(false),
				Description: "Whether to manage a channel that already has this name instead of failing with name_taken " +
					"on create. Archived channels cannot be adopted. After adopting, members (when set) is enforced on the " +
					"existing channel, so users not listed are removed, and topic and purpose from the configuration " +
					"overwrite the existing ones.",
			},
		},
	}
}

func (r *ResourceConversation) Configure(ctx context.Context, req resource.ConfigureRequest, res *resource.ConfigureResponse) {
	if req.ProviderData == nil {
		return
	}
	client := req.ProviderData.(APIClient)
	if !client.HasBotToken() {
		res.Diagnostics.AddError(
			"Missing bot token",
			`The provider attribute "token" is required to use slack_conversation. Set token in the provider configuration.`,
		)
		return
	}
	r.client = client
}

func (r *ResourceConversation) ImportState(ctx context.Context, req resource.ImportStateRequest, res *resource.ImportStateResponse) {
	id := req.ID
	channel, err := r.client.GetConversationInfoContext(ctx, &slack.GetConversationInfoInput{
		ChannelID: id,
	})
	if err != nil {
		res.Diagnostics.AddError(
			fmt.Sprintf("the conversation with the id %s does not exist", id),
			err.Error(),
		)
		return
	}

	botID, err := r.botUserID(ctx)
	if err != nil {
		res.Diagnostics.AddError("failed to identify the bot user", err.Error())
		return
	}
	users, err := allConversationMembers(ctx, r.client, id)
	if err != nil {
		res.Diagnostics.AddError(
			fmt.Sprintf("failed to get users in conversation with the id %s", id),
			err.Error(),
		)
		return
	}
	memberList, diags := types.ListValueFrom(ctx, types.StringType, mergeMemberOrder(nil, users, botID))
	res.Diagnostics.Append(diags...)
	if res.Diagnostics.HasError() {
		return
	}

	state := ResourceConversationState{
		ID:                   types.StringValue(channel.ID),
		Name:                 types.StringValue(channel.Name),
		Topic:                remoteString(types.StringNull(), channel.Topic.Value),
		Purpose:              remoteString(types.StringNull(), channel.Purpose.Value),
		IsPrivate:            types.BoolValue(channel.IsPrivate),
		Members:              memberList,
		ActionOnDestroy:      types.StringValue(actionOnDestroyArchive),
		AdoptExistingChannel: types.BoolValue(false),
	}
	res.Diagnostics.Append(res.State.Set(ctx, &state)...)
}

func (r *ResourceConversation) Create(ctx context.Context, req resource.CreateRequest, res *resource.CreateResponse) {
	var plan ResourceConversationState
	res.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if res.Diagnostics.HasError() {
		return
	}

	channel, adopted := r.createOrAdopt(ctx, plan, &res.Diagnostics)
	if res.Diagnostics.HasError() {
		return
	}

	if !plan.Topic.IsNull() {
		if _, err := r.client.SetTopicOfConversationContext(ctx, channel.ID, plan.Topic.ValueString()); err != nil {
			res.Diagnostics.AddError("failed to set topic of conversation", err.Error())
			return
		}
	}

	if !plan.Purpose.IsNull() {
		if _, err := r.client.SetPurposeOfConversationContext(ctx, channel.ID, plan.Purpose.ValueString()); err != nil {
			res.Diagnostics.AddError("failed to set purpose of conversation", err.Error())
			return
		}
	}

	if !plan.Members.IsNull() {
		// A channel created just now only holds its creator, which is the bot.
		var current []string
		if adopted {
			var err error
			if current, err = allConversationMembers(ctx, r.client, channel.ID); err != nil {
				res.Diagnostics.AddError("failed to get users in conversation", err.Error())
				return
			}
		}
		if err := r.syncMembers(ctx, channel.ID, plan.Members, current); err != nil {
			res.Diagnostics.AddError("failed to set members of conversation", err.Error())
			return
		}
	}

	state := ResourceConversationState{
		ID:                   types.StringValue(channel.ID),
		Name:                 types.StringValue(channel.Name),
		Topic:                plan.Topic,
		Purpose:              plan.Purpose,
		IsPrivate:            types.BoolValue(channel.IsPrivate),
		Members:              plan.Members,
		ActionOnDestroy:      plan.ActionOnDestroy,
		AdoptExistingChannel: plan.AdoptExistingChannel,
	}
	res.Diagnostics.Append(res.State.Set(ctx, &state)...)
}

// createOrAdopt creates the channel, or with adopt_existing_channel returns the
// existing channel when the name is taken. adopted reports the latter.
func (r *ResourceConversation) createOrAdopt(
	ctx context.Context,
	plan ResourceConversationState,
	diags *diag.Diagnostics,
) (channel *slack.Channel, adopted bool) {
	channel, err := r.client.CreateConversationContext(ctx, slack.CreateConversationParams{
		ChannelName: plan.Name.ValueString(),
		IsPrivate:   plan.IsPrivate.ValueBool(),
	})
	if err == nil {
		return channel, false
	}
	if !isSlackError(err, "name_taken") {
		diags.AddError("failed to create conversation", err.Error())
		return nil, false
	}
	if !plan.AdoptExistingChannel.ValueBool() {
		diags.AddError("failed to create conversation", err.Error()+
			": set adopt_existing_channel = true to manage the existing channel. "+
			"Archived channels keep their name, so an archived channel also causes this error.")
		return nil, false
	}

	existing, findErr := findConversationByName(ctx, r.client, plan.Name.ValueString())
	switch {
	case findErr != nil:
		diags.AddError("failed to look up the existing conversation", findErr.Error())
	case existing == nil:
		diags.AddError("failed to create conversation", err.Error())
	case existing.IsArchived:
		diags.AddError(
			"cannot adopt an archived conversation",
			fmt.Sprintf("The conversation %s is archived. Unarchiving requires a user token, which the bot token "+
				"used by this provider cannot do. Unarchive it in Slack or choose another name.", existing.ID),
		)
	case existing.IsPrivate != plan.IsPrivate.ValueBool():
		diags.AddError(
			"cannot adopt a conversation with different privacy",
			fmt.Sprintf("The conversation %s has is_private = %t but the configuration says %t. %s",
				existing.ID, existing.IsPrivate, plan.IsPrivate.ValueBool(), privacyImmutableDetail),
		)
	default:
		return existing, true
	}
	return nil, false
}

func (r *ResourceConversation) Read(ctx context.Context, req resource.ReadRequest, res *resource.ReadResponse) {
	var state ResourceConversationState
	res.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if res.Diagnostics.HasError() {
		return
	}

	channel, err := r.client.GetConversationInfoContext(ctx, &slack.GetConversationInfoInput{
		ChannelID: state.ID.ValueString(),
	})
	if err != nil {
		if isSlackError(err, "channel_not_found") {
			res.State.RemoveResource(ctx)
			return
		}
		res.Diagnostics.AddError("failed to get conversation", err.Error())
		return
	}
	// Archiving is how this resource deletes a channel, so an archived
	// channel counts as gone.
	if channel.IsArchived {
		res.State.RemoveResource(ctx)
		return
	}

	state.Name = types.StringValue(channel.Name)
	state.Topic = remoteString(state.Topic, channel.Topic.Value)
	state.Purpose = remoteString(state.Purpose, channel.Purpose.Value)
	state.IsPrivate = types.BoolValue(channel.IsPrivate)

	if !state.Members.IsNull() {
		var prior []string
		res.Diagnostics.Append(state.Members.ElementsAs(ctx, &prior, false)...)
		if res.Diagnostics.HasError() {
			return
		}
		botID, err := r.botUserID(ctx)
		if err != nil {
			res.Diagnostics.AddError("failed to identify the bot user", err.Error())
			return
		}
		remote, err := allConversationMembers(ctx, r.client, state.ID.ValueString())
		if err != nil {
			res.Diagnostics.AddError("failed to get users in conversation", err.Error())
			return
		}
		members, diags := types.ListValueFrom(ctx, types.StringType, mergeMemberOrder(prior, remote, botID))
		res.Diagnostics.Append(diags...)
		if res.Diagnostics.HasError() {
			return
		}
		state.Members = members
	}

	res.Diagnostics.Append(res.State.Set(ctx, &state)...)
}

func (r *ResourceConversation) Update(ctx context.Context, req resource.UpdateRequest, res *resource.UpdateResponse) {
	var plan, state ResourceConversationState
	res.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	res.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if res.Diagnostics.HasError() {
		return
	}
	id := state.ID.ValueString()

	if !plan.Name.Equal(state.Name) {
		if _, err := r.client.RenameConversationContext(ctx, id, plan.Name.ValueString()); err != nil {
			res.Diagnostics.AddError("failed to rename conversation", err.Error())
			return
		}
	}

	if !plan.Topic.Equal(state.Topic) {
		if _, err := r.client.SetTopicOfConversationContext(ctx, id, plan.Topic.ValueString()); err != nil {
			res.Diagnostics.AddError("failed to set topic of conversation", err.Error())
			return
		}
	}

	if !plan.Purpose.Equal(state.Purpose) {
		if _, err := r.client.SetPurposeOfConversationContext(ctx, id, plan.Purpose.ValueString()); err != nil {
			res.Diagnostics.AddError("failed to set purpose of conversation", err.Error())
			return
		}
	}

	if !plan.Members.IsNull() && !plan.Members.Equal(state.Members) {
		current, err := allConversationMembers(ctx, r.client, id)
		if err != nil {
			res.Diagnostics.AddError("failed to get users in conversation", err.Error())
			return
		}
		if err := r.syncMembers(ctx, id, plan.Members, current); err != nil {
			res.Diagnostics.AddError("failed to set members of conversation", err.Error())
			return
		}
	}

	plan.ID = state.ID
	res.Diagnostics.Append(res.State.Set(ctx, &plan)...)
}

func (r *ResourceConversation) Delete(ctx context.Context, req resource.DeleteRequest, res *resource.DeleteResponse) {
	var state ResourceConversationState
	res.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if res.Diagnostics.HasError() {
		return
	}

	if state.ActionOnDestroy.ValueString() == actionOnDestroyNone {
		return
	}

	err := r.client.ArchiveConversationContext(ctx, state.ID.ValueString())
	if err != nil && !isSlackError(err, "already_archived") && !isSlackError(err, "channel_not_found") {
		res.Diagnostics.AddError("failed to archive conversation", err.Error())
	}
}

// syncMembers makes the channel hold exactly the desired users, apart from the
// bot, given the users current in it.
func (r *ResourceConversation) syncMembers(ctx context.Context, channelID string, desired types.List, current []string) error {
	var users []string
	if diags := desired.ElementsAs(ctx, &users, false); diags.HasError() {
		return fmt.Errorf("read members from the plan: %s", diags)
	}
	botID, err := r.botUserID(ctx)
	if err != nil {
		return err
	}

	invite, kick := diffMembers(users, current, botID)
	if err := inviteMembers(ctx, r.client, channelID, invite); err != nil {
		return fmt.Errorf("invite users: %w", err)
	}
	sort.Strings(kick)
	for _, user := range kick {
		if err := r.client.KickUserFromConversationContext(ctx, channelID, user); err != nil {
			return fmt.Errorf("kick user %s: %w", user, err)
		}
	}
	return nil
}

// botUserID returns the user ID of the token the provider runs as.
func (r *ResourceConversation) botUserID(ctx context.Context) (string, error) {
	resp, err := r.client.AuthTestContext(ctx)
	if err != nil {
		return "", fmt.Errorf("auth.test: %w", err)
	}
	return resp.UserID, nil
}

// remoteString converts a topic or purpose read from Slack to state, keeping
// a null prior value null when Slack holds the empty string.
func remoteString(prior types.String, remote string) types.String {
	if prior.IsNull() && remote == "" {
		return prior
	}
	return types.StringValue(remote)
}
