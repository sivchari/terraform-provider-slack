package internal

import (
	"context"
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/slack-go/slack"
)

var (
	_ datasource.DataSource              = &DataSourceConversation{}
	_ datasource.DataSourceWithConfigure = &DataSourceConversation{}
)

type DataSourceConversation struct {
	client APIClient
}

type DataSourceConversationState struct {
	ID               types.String         `tfsdk:"id"`
	Name             types.String         `tfsdk:"name"`
	Creator          types.String         `tfsdk:"creator"`
	IsArchived       types.Bool           `tfsdk:"is_archived"`
	Members          types.List           `tfsdk:"members"`
	Topic            *ConversationTopic   `tfsdk:"topic"`
	Purpose          *ConversationPurpose `tfsdk:"purpose"`
	IsPrivate        types.Bool           `tfsdk:"is_private"`
	User             types.String         `tfsdk:"user"`
	ConnectedTeamIDs types.List           `tfsdk:"connected_team_ids"`
	SharedTeamIDs    types.List           `tfsdk:"shared_team_ids"`
	InternalTeamIDs  types.List           `tfsdk:"internal_team_ids"`
}

type ConversationTopic struct {
	Value   string `tfsdk:"value"`
	Creator string `tfsdk:"creator"`
}

type ConversationPurpose struct {
	Value   string `tfsdk:"value"`
	Creator string `tfsdk:"creator"`
}

func NewDataSourceConversation() datasource.DataSource {
	return &DataSourceConversation{}
}

func (u *DataSourceConversation) Metadata(_ context.Context, req datasource.MetadataRequest, res *datasource.MetadataResponse) {
	res.TypeName = fmt.Sprintf("%s_conversation", req.ProviderTypeName)
}

func (u *DataSourceConversation) Schema(_ context.Context, _ datasource.SchemaRequest, res *datasource.SchemaResponse) {
	res.Schema = schema.Schema{
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Required: true,
			},
			"name": schema.StringAttribute{
				Computed: true,
			},
			"creator": schema.StringAttribute{
				Computed: true,
			},
			"is_archived": schema.BoolAttribute{
				Computed: true,
			},
			"members": schema.ListAttribute{
				ElementType: types.StringType,
				Computed:    true,
			},
			"topic": schema.SingleNestedAttribute{
				Computed: true,
				Attributes: map[string]schema.Attribute{
					"value": schema.StringAttribute{
						Computed: true,
					},
					"creator": schema.StringAttribute{
						Computed: true,
					},
				},
			},
			"purpose": schema.SingleNestedAttribute{
				Computed: true,
				Attributes: map[string]schema.Attribute{
					"value": schema.StringAttribute{
						Computed: true,
					},
					"creator": schema.StringAttribute{
						Computed: true,
					},
				},
			},
			"is_private": schema.BoolAttribute{
				Computed: true,
			},
			"user": schema.StringAttribute{
				Computed: true,
				Optional: true,
			},
			"connected_team_ids": schema.ListAttribute{
				ElementType: types.StringType,
				Computed:    true,
			},
			"shared_team_ids": schema.ListAttribute{
				ElementType: types.StringType,
				Computed:    true,
			},
			"internal_team_ids": schema.ListAttribute{
				ElementType: types.StringType,
				Computed:    true,
			},
		},
	}
}

func (u *DataSourceConversation) Configure(ctx context.Context, req datasource.ConfigureRequest, res *datasource.ConfigureResponse) {
	if req.ProviderData == nil {
		return
	}
	u.client = req.ProviderData.(APIClient)
}

func (u *DataSourceConversation) Read(ctx context.Context, req datasource.ReadRequest, res *datasource.ReadResponse) {
	var state DataSourceConversationState
	diags := req.Config.Get(ctx, &state)
	res.Diagnostics.Append(diags...)
	if res.Diagnostics.HasError() {
		return
	}
	channel, err := u.client.GetConversationInfoContext(ctx, &slack.GetConversationInfoInput{
		ChannelID: state.ID.ValueString(),
	})
	if err != nil {
		res.Diagnostics.AddError(
			fmt.Sprintf("the conversation with the id %s does not exist", state.ID.String()),
			err.Error(),
		)
		return
	}
	users, _, err := u.client.GetUsersInConversationContext(ctx, &slack.GetUsersInConversationParameters{
		ChannelID: state.ID.ValueString(),
	})
	if err != nil {
		res.Diagnostics.AddError(
			fmt.Sprintf("failed to get users in conversation with id %s", state.ID.String()),
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

	connectedTeamIDs := make([]attr.Value, 0, len(channel.ConnectedTeamIDs))
	for _, teamID := range channel.ConnectedTeamIDs {
		connectedTeamIDs = append(connectedTeamIDs, types.StringValue(teamID))
	}
	connectedTeamIDList, diags := types.ListValue(types.StringType, connectedTeamIDs)
	res.Diagnostics.Append(diags...)
	if res.Diagnostics.HasError() {
		return
	}

	sharedTeamIDs := make([]attr.Value, 0, len(channel.SharedTeamIDs))
	for _, teamID := range channel.SharedTeamIDs {
		sharedTeamIDs = append(sharedTeamIDs, types.StringValue(teamID))
	}
	sharedTeamIDList, diags := types.ListValue(types.StringType, sharedTeamIDs)
	res.Diagnostics.Append(diags...)
	if res.Diagnostics.HasError() {
		return
	}

	internalTeamIDs := make([]attr.Value, 0, len(channel.InternalTeamIDs))
	for _, teamID := range channel.InternalTeamIDs {
		internalTeamIDs = append(internalTeamIDs, types.StringValue(teamID))
	}
	internalTeamIDList, diags := types.ListValue(types.StringType, internalTeamIDs)
	res.Diagnostics.Append(diags...)
	if res.Diagnostics.HasError() {
		return
	}

	state = DataSourceConversationState{
		ID:         types.StringValue(channel.ID),
		Name:       types.StringValue(channel.Name),
		Creator:    types.StringValue(channel.Creator),
		IsArchived: types.BoolValue(channel.IsArchived),
		Members:    memberList,
		Topic: &ConversationTopic{
			Value:   channel.Topic.Value,
			Creator: channel.Topic.Creator,
		},
		Purpose: &ConversationPurpose{
			Value:   channel.Purpose.Value,
			Creator: channel.Purpose.Creator,
		},
		IsPrivate:        types.BoolValue(channel.IsPrivate),
		User:             types.StringValue(channel.User),
		ConnectedTeamIDs: connectedTeamIDList,
		SharedTeamIDs:    sharedTeamIDList,
		InternalTeamIDs:  internalTeamIDList,
	}
	diags = res.State.Set(ctx, state)
	res.Diagnostics.Append(diags...)
}
