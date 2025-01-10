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
	_ resource.Resource                = &ResourceConversation{}
	_ resource.ResourceWithImportState = &ResourceConversation{}
	_ resource.ResourceWithConfigure   = &ResourceConversation{}
)

type ResourceConversation struct {
	client APIClient
}

type ResourceConversationState struct {
	ID        types.String `tfsdk:"id"`
	Name      types.String `tfsdk:"name"`
	Topic     types.String `tfsdk:"topic"`
	Purpose   types.String `tfsdk:"purpose"`
	IsPrivate types.Bool   `tfsdk:"is_private"`
	Members   types.List   `tfsdk:"members"`
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
				Computed: true,
			},
			"name": schema.StringAttribute{
				Required: true,
			},
			"topic": schema.StringAttribute{
				Optional: true,
			},
			"purpose": schema.StringAttribute{
				Optional: true,
			},
			"is_private": schema.BoolAttribute{
				Computed: true,
				Optional: true,
				Default:  booldefault.StaticBool(false),
			},
			"members": schema.ListAttribute{
				Optional:    true,
				ElementType: types.StringType,
			},
		},
	}
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

	users, _, err := r.client.GetUsersInConversationContext(ctx, &slack.GetUsersInConversationParameters{
		ChannelID: id,
	})
	if err != nil {
		res.Diagnostics.AddError(
			fmt.Sprintf("failed to get users in conversation with the id %s", id),
			err.Error(),
		)
		return
	}

	members := make([]attr.Value, 0, len(users))
	for _, user := range users {
		members = append(members, types.StringValue(user))
	}
	memberList, diags := types.ListValue(types.StringType, members)
	res.Diagnostics.Append(diags...)
	if res.Diagnostics.HasError() {
		return
	}

	state := ResourceConversationState{
		ID:        types.StringValue(channel.ID),
		Name:      types.StringValue(channel.Name),
		Topic:     types.StringValue(channel.Topic.Value),
		Purpose:   types.StringValue(channel.Purpose.Value),
		IsPrivate: types.BoolValue(channel.IsPrivate),
		Members:   memberList,
	}
	diags = res.State.Set(ctx, &state)
	res.Diagnostics.Append(diags...)
}

func (r *ResourceConversation) Configure(ctx context.Context, req resource.ConfigureRequest, res *resource.ConfigureResponse) {
	if req.ProviderData == nil {
		return
	}
	r.client = req.ProviderData.(APIClient)
}

func (r *ResourceConversation) Create(ctx context.Context, req resource.CreateRequest, res *resource.CreateResponse) {
	var plan ResourceConversationState
	diags := req.Plan.Get(ctx, &plan)
	res.Diagnostics.Append(diags...)
	if res.Diagnostics.HasError() {
		return
	}

	channel, err := r.client.CreateConversationContext(ctx, slack.CreateConversationParams{
		ChannelName: plan.Name.ValueString(),
		IsPrivate:   plan.IsPrivate.ValueBool(),
	})
	if err != nil {
		res.Diagnostics.AddError("failed to create conversation", err.Error())
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
		var commaSeparatedMembers string
		for _, member := range plan.Members.Elements() {
			var str string
			val, err := member.ToTerraformValue(ctx)
			if err != nil {
				res.Diagnostics.AddError("failed to convert member to terraform value", err.Error())
				return
			}
			if err := val.As(&str); err != nil {
				res.Diagnostics.AddError("failed to convert member to string", err.Error())
				return
			}
			commaSeparatedMembers += str + ","
		}
		if _, err := r.client.InviteUsersToConversationContext(ctx, channel.ID, strings.TrimRight(commaSeparatedMembers, ",")); err != nil {
			res.Diagnostics.AddError("failed to invite users to conversation", err.Error())
			return
		}
	}

	state := ResourceConversationState{
		ID:        types.StringValue(channel.ID),
		Name:      types.StringValue(channel.Name),
		Topic:     types.StringValue(channel.Topic.Value),
		Purpose:   types.StringValue(channel.Purpose.Value),
		IsPrivate: types.BoolValue(channel.IsPrivate),
		Members:   plan.Members,
	}

	diags = res.State.Set(ctx, &state)
	res.Diagnostics.Append(diags...)
}

func (r *ResourceConversation) Read(ctx context.Context, req resource.ReadRequest, res *resource.ReadResponse) {
	var state ResourceConversationState
	diags := req.State.Get(ctx, &state)
	res.Diagnostics.Append(diags...)
	if res.Diagnostics.HasError() {
		return
	}
	diags = res.State.Set(ctx, &state)
	res.Diagnostics.Append(diags...)
}

func (r *ResourceConversation) Update(ctx context.Context, req resource.UpdateRequest, res *resource.UpdateResponse) {
	var plan ResourceConversationState
	diags := req.Plan.Get(ctx, &plan)
	res.Diagnostics.Append(diags...)
	if res.Diagnostics.HasError() {
		return
	}

	if _, err := r.client.SetTopicOfConversationContext(ctx, plan.ID.ValueString(), plan.Topic.ValueString()); err != nil {
		res.Diagnostics.AddError("failed to set topic of conversation", err.Error())
		return
	}

	if _, err := r.client.SetPurposeOfConversationContext(ctx, plan.ID.ValueString(), plan.Purpose.ValueString()); err != nil {
		res.Diagnostics.AddError("failed to set purpose of conversation", err.Error())
		return
	}

	existingUsers, _, err := r.client.GetUsersInConversationContext(ctx, &slack.GetUsersInConversationParameters{
		ChannelID: plan.ID.ValueString(),
	})
	if err != nil {
		res.Diagnostics.AddError("failed to get users in conversation", err.Error())
		return
	}
	existingUsersMap := make(map[string]struct{}, len(existingUsers))
	for _, user := range existingUsers {
		existingUsersMap[user] = struct{}{}
	}

	members := make(map[string]struct{}, len(plan.Members.Elements()))
	for _, member := range plan.Members.Elements() {
		var str string
		val, err := member.ToTerraformValue(ctx)
		if err != nil {
			res.Diagnostics.AddError("failed to convert member to terraform value", err.Error())
			return
		}
		if err := val.As(&str); err != nil {
			res.Diagnostics.AddError("failed to convert member to string", err.Error())
			return
		}
		members[str] = struct{}{}
	}

	var commaSeparatedMembers string
	for member := range members {
		if _, ok := existingUsersMap[member]; !ok {
			commaSeparatedMembers += member + ","
		}
	}

	var removedMembers []string
	for member := range existingUsersMap {
		if _, ok := members[member]; !ok {
			removedMembers = append(removedMembers, member)
		}
	}

	if commaSeparatedMembers != "" {
		if _, err := r.client.InviteUsersToConversationContext(ctx, plan.ID.ValueString(), strings.TrimRight(commaSeparatedMembers, ",")); err != nil {
			res.Diagnostics.AddError("failed to invite users to conversation", err.Error())
			return
		}
	}

	for _, member := range removedMembers {
		if err := r.client.KickUserFromConversationContext(ctx, plan.ID.ValueString(), member); err != nil {
			res.Diagnostics.AddError("failed to kick user from conversation", err.Error())
			return
		}
	}

	state := ResourceConversationState{
		ID:        plan.ID,
		Name:      plan.Name,
		Topic:     plan.Topic,
		Purpose:   plan.Purpose,
		IsPrivate: plan.IsPrivate,
		Members:   plan.Members,
	}

	diags = res.State.Set(ctx, &state)
	res.Diagnostics.Append(diags...)
}

func (r *ResourceConversation) Delete(ctx context.Context, req resource.DeleteRequest, res *resource.DeleteResponse) {
	var state ResourceConversationState
	diags := req.State.Get(ctx, &state)
	res.Diagnostics.Append(diags...)
	if res.Diagnostics.HasError() {
		return
	}

	channel, err := r.client.GetConversationInfoContext(ctx, &slack.GetConversationInfoInput{
		ChannelID: state.ID.ValueString(),
	})
	if err != nil {
		res.Diagnostics.AddError(
			fmt.Sprintf("the conversation with the id %s does not exist", state.ID.String()),
			err.Error(),
		)
		return
	}

	user, mpim := channel.User, channel.IsMpIM
	if user != "" || mpim {
		if _, closed, err := r.client.CloseConversationContext(ctx, state.ID.ValueString()); err != nil {
			if !closed {
				res.Diagnostics.AddError("failed to close conversation", err.Error())
				return
			}
		}
	} else {
		if err := r.client.ArchiveConversationContext(ctx, state.ID.ValueString()); err != nil {
			res.Diagnostics.AddError("failed to archive conversation", err.Error())
			return
		}
	}
}
