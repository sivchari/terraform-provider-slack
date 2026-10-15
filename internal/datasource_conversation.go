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

	_ datasource.DataSourceWithConfigValidators = &DataSourceConversation{}
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

func (d *DataSourceConversation) Metadata(_ context.Context, req datasource.MetadataRequest, res *datasource.MetadataResponse) {
	res.TypeName = fmt.Sprintf("%s_conversation", req.ProviderTypeName)
}

func (d *DataSourceConversation) Schema(_ context.Context, _ datasource.SchemaRequest, res *datasource.SchemaResponse) {
	res.Schema = schema.Schema{
		Description: "Looks up a Slack public or private channel by ID or name. Exactly one of id or name must be set. " +
			"Archived channels are found too; check is_archived.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Optional:    true,
				Computed:    true,
				Description: "ID of the channel, for example C0123456789. Conflicts with name.",
			},
			"name": schema.StringAttribute{
				Optional: true,
				Computed: true,
				Description: "Name of the channel without the leading #. Conflicts with id. Slack cannot filter " +
					"by name, so every channel is listed to find it; prefer id on large workspaces.",
			},
			"creator": schema.StringAttribute{
				Computed:    true,
				Description: "ID of the user who created the channel.",
			},
			"is_archived": schema.BoolAttribute{
				Computed:    true,
				Description: "Whether the channel is archived.",
			},
			"members": schema.ListAttribute{
				ElementType: types.StringType,
				Computed:    true,
				Description: "IDs of the users in the channel, including the provider's own bot user.",
			},
			"topic": schema.SingleNestedAttribute{
				Computed:    true,
				Description: "Topic of the channel.",
				Attributes: map[string]schema.Attribute{
					"value": schema.StringAttribute{
						Computed:    true,
						Description: "Text of the topic.",
					},
					"creator": schema.StringAttribute{
						Computed:    true,
						Description: "ID of the user who set the topic.",
					},
				},
			},
			"purpose": schema.SingleNestedAttribute{
				Computed:    true,
				Description: "Purpose of the channel.",
				Attributes: map[string]schema.Attribute{
					"value": schema.StringAttribute{
						Computed:    true,
						Description: "Text of the purpose.",
					},
					"creator": schema.StringAttribute{
						Computed:    true,
						Description: "ID of the user who set the purpose.",
					},
				},
			},
			"is_private": schema.BoolAttribute{
				Computed:    true,
				Description: "Whether the channel is private.",
			},
			"user": schema.StringAttribute{
				Computed:    true,
				Description: "ID of the other user in a direct message. Empty for public and private channels.",
			},
			"connected_team_ids": schema.ListAttribute{
				ElementType: types.StringType,
				Computed:    true,
				Description: "IDs of the teams that Slack reports as connected to the channel.",
			},
			"shared_team_ids": schema.ListAttribute{
				ElementType: types.StringType,
				Computed:    true,
				Description: "IDs of the teams that Slack reports the channel as shared with.",
			},
			"internal_team_ids": schema.ListAttribute{
				ElementType: types.StringType,
				Computed:    true,
				Description: "IDs of the teams within the same organization that Slack reports for the channel.",
			},
		},
	}
}

func (d *DataSourceConversation) ConfigValidators(context.Context) []datasource.ConfigValidator {
	return []datasource.ConfigValidator{exactlyOneOf("id", "name")}
}

func (d *DataSourceConversation) Configure(ctx context.Context, req datasource.ConfigureRequest, res *datasource.ConfigureResponse) {
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
	d.client = client
}

func (d *DataSourceConversation) Read(ctx context.Context, req datasource.ReadRequest, res *datasource.ReadResponse) {
	var state DataSourceConversationState
	diags := req.Config.Get(ctx, &state)
	res.Diagnostics.Append(diags...)
	if res.Diagnostics.HasError() {
		return
	}
	channelID := state.ID.ValueString()
	if state.ID.IsNull() {
		found, err := findConversationByName(ctx, d.client, state.Name.ValueString())
		if err != nil {
			res.Diagnostics.AddError(
				fmt.Sprintf("Failed to list the conversations to find the name %q", state.Name.ValueString()),
				err.Error(),
			)
			return
		}
		if found == nil {
			res.Diagnostics.AddError(
				"Conversation not found",
				fmt.Sprintf("No public or private channel is named %q.", state.Name.ValueString()),
			)
			return
		}
		channelID = found.ID
	}
	channel, err := d.client.GetConversationInfoContext(ctx, &slack.GetConversationInfoInput{
		ChannelID: channelID,
	})
	if err != nil {
		res.Diagnostics.AddError(
			fmt.Sprintf("the conversation with the id %q could not be retrieved", channelID),
			err.Error(),
		)
		return
	}
	users, err := allConversationMembers(ctx, d.client, channelID)
	if err != nil {
		res.Diagnostics.AddError(
			fmt.Sprintf("failed to get users in conversation with id %q", channelID),
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
