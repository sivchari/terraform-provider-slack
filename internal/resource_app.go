package internal

import (
	"context"
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/slack-go/slack"
)

var (
	_ resource.Resource                = &ResourceApp{}
	_ resource.ResourceWithImportState = &ResourceApp{}
	_ resource.ResourceWithConfigure   = &ResourceApp{}
)

type ResourceApp struct {
	client APIClient
}

type ResourceAppState struct {
	ID                 types.String           `tfsdk:"id"`
	ClientID           types.String           `tfsdk:"client_id"`
	ClientSecret       types.String           `tfsdk:"client_secret"`
	VerificationToken  types.String           `tfsdk:"verification_token"`
	SigningSecret      types.String           `tfsdk:"signing_secret"`
	OAuthAuthorizeURL  types.String           `tfsdk:"oauth_authorize_url"`
	DisplayInformation *AppDisplayInformation `tfsdk:"display_information"`
	Features           *AppFeatures           `tfsdk:"features"`
	OAuthConfig        *AppOAuthConfig        `tfsdk:"oauth_config"`
	Settings           *AppSettings           `tfsdk:"settings"`
}

type AppDisplayInformation struct {
	Name            types.String `tfsdk:"name"`
	Description     types.String `tfsdk:"description"`
	LongDescription types.String `tfsdk:"long_description"`
	BackgroundColor types.String `tfsdk:"background_color"`
}

type AppFeatures struct {
	AppHome       *AppHome          `tfsdk:"app_home"`
	BotUser       *AppBotUser       `tfsdk:"bot_user"`
	Shortcuts     []AppShortcut     `tfsdk:"shortcuts"`
	SlashCommands []AppSlashCommand `tfsdk:"slash_commands"`
	WorkflowSteps []AppWorkflowStep `tfsdk:"workflow_steps"`
}

type AppHome struct {
	HomeTabEnabled             types.Bool `tfsdk:"home_tab_enabled"`
	MessagesTabEnabled         types.Bool `tfsdk:"messages_tab_enabled"`
	MessagesTabReadOnlyEnabled types.Bool `tfsdk:"messages_tab_read_only_enabled"`
}

type AppBotUser struct {
	DisplayName  types.String `tfsdk:"display_name"`
	AlwaysOnline types.Bool   `tfsdk:"always_online"`
}

type AppShortcut struct {
	Name        types.String `tfsdk:"name"`
	CallbackID  types.String `tfsdk:"callback_id"`
	Description types.String `tfsdk:"description"`
	Type        types.String `tfsdk:"type"`
}

type AppSlashCommand struct {
	Command      types.String `tfsdk:"command"`
	Description  types.String `tfsdk:"description"`
	ShouldEscape types.Bool   `tfsdk:"should_escape"`
	URL          types.String `tfsdk:"url"`
	UsageHint    types.String `tfsdk:"usage_hint"`
}

type AppWorkflowStep struct {
	Name       types.String `tfsdk:"name"`
	CallbackID types.String `tfsdk:"callback_id"`
}

type AppOAuthConfig struct {
	RedirectURLs types.List      `tfsdk:"redirect_urls"`
	Scopes       *AppOAuthScopes `tfsdk:"scopes"`
}

type AppOAuthScopes struct {
	Bot  types.List `tfsdk:"bot"`
	User types.List `tfsdk:"user"`
}

type AppSettings struct {
	AllowedIPAddressRanges types.List             `tfsdk:"allowed_ip_address_ranges"`
	EventSubscriptions     *AppEventSubscriptions `tfsdk:"event_subscriptions"`
	Interactivity          *AppInteractivity      `tfsdk:"interactivity"`
	OrgDeployEnabled       types.Bool             `tfsdk:"org_deploy_enabled"`
	SocketModeEnabled      types.Bool             `tfsdk:"socket_mode_enabled"`
}

type AppEventSubscriptions struct {
	RequestURL types.String `tfsdk:"request_url"`
	BotEvents  types.List   `tfsdk:"bot_events"`
	UserEvents types.List   `tfsdk:"user_events"`
}

type AppInteractivity struct {
	IsEnabled             types.Bool   `tfsdk:"is_enabled"`
	RequestURL            types.String `tfsdk:"request_url"`
	MessageMenuOptionsURL types.String `tfsdk:"message_menu_options_url"`
}

func NewResourceApp() resource.Resource {
	return &ResourceApp{}
}

func (r *ResourceApp) Metadata(_ context.Context, req resource.MetadataRequest, res *resource.MetadataResponse) {
	res.TypeName = fmt.Sprintf("%s_app", req.ProviderTypeName)
}

func (r *ResourceApp) Schema(_ context.Context, _ resource.SchemaRequest, res *resource.SchemaResponse) {
	stringList := schema.ListAttribute{
		Optional:    true,
		ElementType: types.StringType,
	}

	res.Schema = schema.Schema{
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Computed:      true,
				PlanModifiers: []planmodifier.String{createOnlyString()},
			},
			"client_id": schema.StringAttribute{
				Computed:      true,
				PlanModifiers: []planmodifier.String{createOnlyString()},
			},
			"client_secret": schema.StringAttribute{
				Computed:      true,
				Sensitive:     true,
				PlanModifiers: []planmodifier.String{createOnlyString()},
			},
			"verification_token": schema.StringAttribute{
				Computed:      true,
				Sensitive:     true,
				PlanModifiers: []planmodifier.String{createOnlyString()},
			},
			"signing_secret": schema.StringAttribute{
				Computed:      true,
				Sensitive:     true,
				PlanModifiers: []planmodifier.String{createOnlyString()},
			},
			"oauth_authorize_url": schema.StringAttribute{
				Computed:      true,
				PlanModifiers: []planmodifier.String{createOnlyString()},
			},
			"display_information": schema.SingleNestedAttribute{
				Required: true,
				Attributes: map[string]schema.Attribute{
					"name": schema.StringAttribute{
						Required: true,
					},
					"description": schema.StringAttribute{
						Optional: true,
					},
					"long_description": schema.StringAttribute{
						Optional: true,
					},
					"background_color": schema.StringAttribute{
						Optional: true,
					},
				},
			},
			"features": schema.SingleNestedAttribute{
				Optional: true,
				Attributes: map[string]schema.Attribute{
					"app_home": schema.SingleNestedAttribute{
						Optional: true,
						Attributes: map[string]schema.Attribute{
							"home_tab_enabled": schema.BoolAttribute{
								Optional: true,
							},
							"messages_tab_enabled": schema.BoolAttribute{
								Optional: true,
							},
							"messages_tab_read_only_enabled": schema.BoolAttribute{
								Optional: true,
							},
						},
					},
					"bot_user": schema.SingleNestedAttribute{
						Optional: true,
						Attributes: map[string]schema.Attribute{
							"display_name": schema.StringAttribute{
								Required: true,
							},
							"always_online": schema.BoolAttribute{
								Optional: true,
							},
						},
					},
					"shortcuts": schema.ListNestedAttribute{
						Optional: true,
						NestedObject: schema.NestedAttributeObject{
							Attributes: map[string]schema.Attribute{
								"name": schema.StringAttribute{
									Required: true,
								},
								"callback_id": schema.StringAttribute{
									Required: true,
								},
								"description": schema.StringAttribute{
									Required: true,
								},
								"type": schema.StringAttribute{
									Required: true,
								},
							},
						},
					},
					"slash_commands": schema.ListNestedAttribute{
						Optional: true,
						NestedObject: schema.NestedAttributeObject{
							Attributes: map[string]schema.Attribute{
								"command": schema.StringAttribute{
									Required: true,
								},
								"description": schema.StringAttribute{
									Required: true,
								},
								"should_escape": schema.BoolAttribute{
									Optional: true,
								},
								"url": schema.StringAttribute{
									Optional: true,
								},
								"usage_hint": schema.StringAttribute{
									Optional: true,
								},
							},
						},
					},
					"workflow_steps": schema.ListNestedAttribute{
						Optional: true,
						NestedObject: schema.NestedAttributeObject{
							Attributes: map[string]schema.Attribute{
								"name": schema.StringAttribute{
									Required: true,
								},
								"callback_id": schema.StringAttribute{
									Required: true,
								},
							},
						},
					},
				},
			},
			"oauth_config": schema.SingleNestedAttribute{
				Optional: true,
				Attributes: map[string]schema.Attribute{
					"redirect_urls": stringList,
					"scopes": schema.SingleNestedAttribute{
						Optional: true,
						Attributes: map[string]schema.Attribute{
							"bot":  stringList,
							"user": stringList,
						},
					},
				},
			},
			"settings": schema.SingleNestedAttribute{
				Optional: true,
				Attributes: map[string]schema.Attribute{
					"allowed_ip_address_ranges": stringList,
					"event_subscriptions": schema.SingleNestedAttribute{
						Optional: true,
						Attributes: map[string]schema.Attribute{
							"request_url": schema.StringAttribute{
								Optional: true,
							},
							"bot_events":  stringList,
							"user_events": stringList,
						},
					},
					"interactivity": schema.SingleNestedAttribute{
						Optional: true,
						Attributes: map[string]schema.Attribute{
							"is_enabled": schema.BoolAttribute{
								Required: true,
							},
							"request_url": schema.StringAttribute{
								Optional: true,
							},
							"message_menu_options_url": schema.StringAttribute{
								Optional: true,
							},
						},
					},
					"org_deploy_enabled": schema.BoolAttribute{
						Optional: true,
					},
					"socket_mode_enabled": schema.BoolAttribute{
						Optional: true,
					},
				},
			},
		},
	}
}

func (r *ResourceApp) Configure(ctx context.Context, req resource.ConfigureRequest, res *resource.ConfigureResponse) {
	if req.ProviderData == nil {
		return
	}
	r.client = req.ProviderData.(APIClient)
}

func (r *ResourceApp) Create(ctx context.Context, req resource.CreateRequest, res *resource.CreateResponse) {
	var plan ResourceAppState
	diags := req.Plan.Get(ctx, &plan)
	res.Diagnostics.Append(diags...)
	if res.Diagnostics.HasError() {
		return
	}

	manifest, diags := manifestFromState(ctx, plan)
	res.Diagnostics.Append(diags...)
	if res.Diagnostics.HasError() {
		return
	}

	created, err := r.client.CreateAppManifest(ctx, manifest, "")
	if err != nil {
		res.Diagnostics.AddError("failed to create app", err.Error())
		return
	}

	state := plan
	state.ID = types.StringValue(created.AppID)
	state.ClientID = types.StringValue(created.Credentials.ClientID)
	state.ClientSecret = types.StringValue(created.Credentials.ClientSecret)
	state.VerificationToken = types.StringValue(created.Credentials.VerificationToken)
	state.SigningSecret = types.StringValue(created.Credentials.SigningSecret)
	state.OAuthAuthorizeURL = types.StringValue(created.OAuthAuthorizeURL)

	diags = res.State.Set(ctx, &state)
	res.Diagnostics.Append(diags...)
}

func (r *ResourceApp) Read(ctx context.Context, req resource.ReadRequest, res *resource.ReadResponse) {
	var state ResourceAppState
	diags := req.State.Get(ctx, &state)
	res.Diagnostics.Append(diags...)
	if res.Diagnostics.HasError() {
		return
	}
	diags = res.State.Set(ctx, &state)
	res.Diagnostics.Append(diags...)
}

func (r *ResourceApp) Update(ctx context.Context, req resource.UpdateRequest, res *resource.UpdateResponse) {
	var plan ResourceAppState
	diags := req.Plan.Get(ctx, &plan)
	res.Diagnostics.Append(diags...)
	if res.Diagnostics.HasError() {
		return
	}

	// The app ID must come from state: computed attributes without a plan
	// modifier are unknown in the plan, and an unknown types.String yields ""
	// which Slack rejects with invalid_arguments.
	var state ResourceAppState
	diags = req.State.Get(ctx, &state)
	res.Diagnostics.Append(diags...)
	if res.Diagnostics.HasError() {
		return
	}

	manifest, diags := manifestFromState(ctx, plan)
	res.Diagnostics.Append(diags...)
	if res.Diagnostics.HasError() {
		return
	}

	if _, err := r.client.UpdateManifestContext(ctx, manifest, "", state.ID.ValueString()); err != nil {
		res.Diagnostics.AddError("failed to update app", err.Error())
		return
	}

	plan.ID = state.ID
	diags = res.State.Set(ctx, &plan)
	res.Diagnostics.Append(diags...)
}

func (r *ResourceApp) Delete(ctx context.Context, req resource.DeleteRequest, res *resource.DeleteResponse) {
	var state ResourceAppState
	diags := req.State.Get(ctx, &state)
	res.Diagnostics.Append(diags...)
	if res.Diagnostics.HasError() {
		return
	}

	if _, err := r.client.DeleteManifestContext(ctx, "", state.ID.ValueString()); err != nil {
		res.Diagnostics.AddError("failed to delete app", err.Error())
		return
	}
}

func (r *ResourceApp) ImportState(ctx context.Context, req resource.ImportStateRequest, res *resource.ImportStateResponse) {
	manifest, err := r.client.ExportManifestContext(ctx, "", req.ID)
	if err != nil {
		res.Diagnostics.AddError("failed to export app manifest", err.Error())
		return
	}

	state, diags := stateFromManifest(ctx, manifest, ResourceAppState{ID: types.StringValue(req.ID)})
	res.Diagnostics.Append(diags...)
	if res.Diagnostics.HasError() {
		return
	}
	// credentials are only returned by apps.manifest.create and cannot be
	// recovered through import
	state.ClientID = types.StringNull()
	state.ClientSecret = types.StringNull()
	state.VerificationToken = types.StringNull()
	state.SigningSecret = types.StringNull()
	state.OAuthAuthorizeURL = types.StringNull()

	diags = res.State.Set(ctx, &state)
	res.Diagnostics.Append(diags...)
}

func manifestFromState(ctx context.Context, state ResourceAppState) (*slack.Manifest, diag.Diagnostics) {
	var diags diag.Diagnostics

	manifest := &slack.Manifest{
		Display: slack.Display{
			Name:            state.DisplayInformation.Name.ValueString(),
			Description:     state.DisplayInformation.Description.ValueString(),
			LongDescription: state.DisplayInformation.LongDescription.ValueString(),
			BackgroundColor: state.DisplayInformation.BackgroundColor.ValueString(),
		},
	}

	if state.Features != nil {
		manifest.Features = slack.Features{
			Shortcuts:     shortcutsFromState(state.Features.Shortcuts),
			SlashCommands: slashCommandsFromState(state.Features.SlashCommands),
			WorkflowSteps: workflowStepsFromState(state.Features.WorkflowSteps),
		}
		if state.Features.AppHome != nil {
			manifest.Features.AppHome = slack.AppHome{
				HomeTabEnabled:             state.Features.AppHome.HomeTabEnabled.ValueBool(),
				MessagesTabEnabled:         state.Features.AppHome.MessagesTabEnabled.ValueBool(),
				MessagesTabReadOnlyEnabled: state.Features.AppHome.MessagesTabReadOnlyEnabled.ValueBool(),
			}
		}
		if state.Features.BotUser != nil {
			manifest.Features.BotUser = slack.BotUser{
				DisplayName:  state.Features.BotUser.DisplayName.ValueString(),
				AlwaysOnline: state.Features.BotUser.AlwaysOnline.ValueBool(),
			}
		}
	}

	if state.OAuthConfig != nil {
		redirectURLs, d := stringsFromList(ctx, state.OAuthConfig.RedirectURLs)
		diags.Append(d...)
		manifest.OAuthConfig.RedirectUrls = redirectURLs
		if state.OAuthConfig.Scopes != nil {
			bot, d := stringsFromList(ctx, state.OAuthConfig.Scopes.Bot)
			diags.Append(d...)
			user, d := stringsFromList(ctx, state.OAuthConfig.Scopes.User)
			diags.Append(d...)
			manifest.OAuthConfig.Scopes = slack.OAuthScopes{
				Bot:  bot,
				User: user,
			}
		}
	}

	if state.Settings != nil {
		allowedIPs, d := stringsFromList(ctx, state.Settings.AllowedIPAddressRanges)
		diags.Append(d...)
		manifest.Settings.AllowedIPAddressRanges = allowedIPs
		manifest.Settings.OrgDeployEnabled = state.Settings.OrgDeployEnabled.ValueBool()
		manifest.Settings.SocketModeEnabled = state.Settings.SocketModeEnabled.ValueBool()
		if state.Settings.EventSubscriptions != nil {
			botEvents, d := stringsFromList(ctx, state.Settings.EventSubscriptions.BotEvents)
			diags.Append(d...)
			userEvents, d := stringsFromList(ctx, state.Settings.EventSubscriptions.UserEvents)
			diags.Append(d...)
			manifest.Settings.EventSubscriptions = slack.EventSubscriptions{
				RequestUrl: state.Settings.EventSubscriptions.RequestURL.ValueString(),
				BotEvents:  botEvents,
				UserEvents: userEvents,
			}
		}
		if state.Settings.Interactivity != nil {
			manifest.Settings.Interactivity = slack.Interactivity{
				IsEnabled:             state.Settings.Interactivity.IsEnabled.ValueBool(),
				RequestUrl:            state.Settings.Interactivity.RequestURL.ValueString(),
				MessageMenuOptionsUrl: state.Settings.Interactivity.MessageMenuOptionsURL.ValueString(),
			}
		}
	}

	return manifest, diags
}

func stateFromManifest(ctx context.Context, manifest *slack.Manifest, existing ResourceAppState) (ResourceAppState, diag.Diagnostics) {
	var diags diag.Diagnostics

	state := existing
	state.DisplayInformation = &AppDisplayInformation{
		Name:            types.StringValue(manifest.Display.Name),
		Description:     types.StringValue(manifest.Display.Description),
		LongDescription: types.StringValue(manifest.Display.LongDescription),
		BackgroundColor: types.StringValue(manifest.Display.BackgroundColor),
	}

	features := &AppFeatures{
		Shortcuts:     shortcutsToState(manifest.Features.Shortcuts),
		SlashCommands: slashCommandsToState(manifest.Features.SlashCommands),
		WorkflowSteps: workflowStepsToState(manifest.Features.WorkflowSteps),
	}
	if !isZeroAppHome(manifest.Features.AppHome) {
		features.AppHome = &AppHome{
			HomeTabEnabled:             types.BoolValue(manifest.Features.AppHome.HomeTabEnabled),
			MessagesTabEnabled:         types.BoolValue(manifest.Features.AppHome.MessagesTabEnabled),
			MessagesTabReadOnlyEnabled: types.BoolValue(manifest.Features.AppHome.MessagesTabReadOnlyEnabled),
		}
	}
	if !isZeroBotUser(manifest.Features.BotUser) {
		features.BotUser = &AppBotUser{
			DisplayName:  types.StringValue(manifest.Features.BotUser.DisplayName),
			AlwaysOnline: types.BoolValue(manifest.Features.BotUser.AlwaysOnline),
		}
	}
	state.Features = features

	if len(manifest.OAuthConfig.RedirectUrls) == 0 && isZeroOAuthScopes(manifest.OAuthConfig.Scopes) {
		state.OAuthConfig = nil
	} else {
		redirectURLs, d := listFromStrings(ctx, manifest.OAuthConfig.RedirectUrls)
		diags.Append(d...)
		oauthConfig := &AppOAuthConfig{RedirectURLs: redirectURLs}
		if !isZeroOAuthScopes(manifest.OAuthConfig.Scopes) {
			bot, d := listFromStrings(ctx, manifest.OAuthConfig.Scopes.Bot)
			diags.Append(d...)
			user, d := listFromStrings(ctx, manifest.OAuthConfig.Scopes.User)
			diags.Append(d...)
			oauthConfig.Scopes = &AppOAuthScopes{Bot: bot, User: user}
		}
		state.OAuthConfig = oauthConfig
	}

	eventsZero := isZeroEventSubscriptions(manifest.Settings.EventSubscriptions)
	interactivityZero := isZeroInteractivity(manifest.Settings.Interactivity)
	if len(manifest.Settings.AllowedIPAddressRanges) == 0 && !manifest.Settings.OrgDeployEnabled &&
		!manifest.Settings.SocketModeEnabled && eventsZero && interactivityZero {
		state.Settings = nil
	} else {
		allowedIPs, d := listFromStrings(ctx, manifest.Settings.AllowedIPAddressRanges)
		diags.Append(d...)
		settings := &AppSettings{
			AllowedIPAddressRanges: allowedIPs,
			OrgDeployEnabled:       types.BoolValue(manifest.Settings.OrgDeployEnabled),
			SocketModeEnabled:      types.BoolValue(manifest.Settings.SocketModeEnabled),
		}
		if !eventsZero {
			botEvents, d := listFromStrings(ctx, manifest.Settings.EventSubscriptions.BotEvents)
			diags.Append(d...)
			userEvents, d := listFromStrings(ctx, manifest.Settings.EventSubscriptions.UserEvents)
			diags.Append(d...)
			settings.EventSubscriptions = &AppEventSubscriptions{
				RequestURL: types.StringValue(manifest.Settings.EventSubscriptions.RequestUrl),
				BotEvents:  botEvents,
				UserEvents: userEvents,
			}
		}
		if !interactivityZero {
			settings.Interactivity = &AppInteractivity{
				IsEnabled:             types.BoolValue(manifest.Settings.Interactivity.IsEnabled),
				RequestURL:            types.StringValue(manifest.Settings.Interactivity.RequestUrl),
				MessageMenuOptionsURL: types.StringValue(manifest.Settings.Interactivity.MessageMenuOptionsUrl),
			}
		}
		state.Settings = settings
	}

	return state, diags
}

func isZeroAppHome(h slack.AppHome) bool {
	return !h.HomeTabEnabled && !h.MessagesTabEnabled && !h.MessagesTabReadOnlyEnabled
}

func isZeroBotUser(b slack.BotUser) bool {
	return b.DisplayName == "" && !b.AlwaysOnline
}

func isZeroOAuthScopes(s slack.OAuthScopes) bool {
	return len(s.Bot) == 0 && len(s.User) == 0
}

func isZeroEventSubscriptions(e slack.EventSubscriptions) bool {
	return e.RequestUrl == "" && len(e.BotEvents) == 0 && len(e.UserEvents) == 0
}

func isZeroInteractivity(i slack.Interactivity) bool {
	return !i.IsEnabled && i.RequestUrl == "" && i.MessageMenuOptionsUrl == ""
}

func shortcutsFromState(shortcuts []AppShortcut) []slack.Shortcut {
	out := make([]slack.Shortcut, 0, len(shortcuts))
	for _, s := range shortcuts {
		out = append(out, slack.Shortcut{
			Name:        s.Name.ValueString(),
			CallbackID:  s.CallbackID.ValueString(),
			Description: s.Description.ValueString(),
			Type:        slack.ShortcutType(s.Type.ValueString()),
		})
	}
	return out
}

func shortcutsToState(shortcuts []slack.Shortcut) []AppShortcut {
	out := make([]AppShortcut, 0, len(shortcuts))
	for _, s := range shortcuts {
		out = append(out, AppShortcut{
			Name:        types.StringValue(s.Name),
			CallbackID:  types.StringValue(s.CallbackID),
			Description: types.StringValue(s.Description),
			Type:        types.StringValue(string(s.Type)),
		})
	}
	return out
}

func slashCommandsFromState(commands []AppSlashCommand) []slack.ManifestSlashCommand {
	out := make([]slack.ManifestSlashCommand, 0, len(commands))
	for _, c := range commands {
		out = append(out, slack.ManifestSlashCommand{
			Command:      c.Command.ValueString(),
			Description:  c.Description.ValueString(),
			ShouldEscape: c.ShouldEscape.ValueBool(),
			Url:          c.URL.ValueString(),
			UsageHint:    c.UsageHint.ValueString(),
		})
	}
	return out
}

func slashCommandsToState(commands []slack.ManifestSlashCommand) []AppSlashCommand {
	out := make([]AppSlashCommand, 0, len(commands))
	for _, c := range commands {
		out = append(out, AppSlashCommand{
			Command:      types.StringValue(c.Command),
			Description:  types.StringValue(c.Description),
			ShouldEscape: types.BoolValue(c.ShouldEscape),
			URL:          types.StringValue(c.Url),
			UsageHint:    types.StringValue(c.UsageHint),
		})
	}
	return out
}

func workflowStepsFromState(steps []AppWorkflowStep) []slack.WorkflowStep {
	out := make([]slack.WorkflowStep, 0, len(steps))
	for _, s := range steps {
		out = append(out, slack.WorkflowStep{
			Name:       s.Name.ValueString(),
			CallbackID: s.CallbackID.ValueString(),
		})
	}
	return out
}

func workflowStepsToState(steps []slack.WorkflowStep) []AppWorkflowStep {
	out := make([]AppWorkflowStep, 0, len(steps))
	for _, s := range steps {
		out = append(out, AppWorkflowStep{
			Name:       types.StringValue(s.Name),
			CallbackID: types.StringValue(s.CallbackID),
		})
	}
	return out
}

func stringsFromList(ctx context.Context, list types.List) ([]string, diag.Diagnostics) {
	if list.IsNull() || list.IsUnknown() {
		return nil, nil
	}
	var out []string
	diags := list.ElementsAs(ctx, &out, false)
	return out, diags
}

func listFromStrings(ctx context.Context, values []string) (types.List, diag.Diagnostics) {
	return types.ListValueFrom(ctx, types.StringType, values)
}
