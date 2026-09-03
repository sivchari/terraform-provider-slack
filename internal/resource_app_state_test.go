package internal

import (
	"context"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/slack-go/slack"
)

func TestStateFromManifest_ZeroValuesBecomeNullWithoutPrior(t *testing.T) {
	t.Parallel()

	manifest := &slack.Manifest{
		Display: slack.Display{Name: "test"},
		Settings: slack.Settings{
			SocketModeEnabled: true,
			EventSubscriptions: slack.EventSubscriptions{
				BotEvents: []string{"app_mention"},
			},
			Interactivity: slack.Interactivity{IsEnabled: true},
		},
	}

	state, diags := stateFromManifest(context.Background(), manifest, ResourceAppState{ID: types.StringValue("A1")})
	if diags.HasError() {
		t.Errorf("stateFromManifest() diagnostics: %v", diags)
		return
	}

	if !state.DisplayInformation.Description.IsNull() {
		t.Errorf("Description = %v, want null", state.DisplayInformation.Description)
	}
	if state.Features != nil {
		t.Errorf("Features = %+v, want nil", state.Features)
	}
	if !state.Settings.EventSubscriptions.RequestURL.IsNull() {
		t.Errorf("EventSubscriptions.RequestURL = %v, want null", state.Settings.EventSubscriptions.RequestURL)
	}
	if !state.Settings.Interactivity.RequestURL.IsNull() {
		t.Errorf("Interactivity.RequestURL = %v, want null", state.Settings.Interactivity.RequestURL)
	}
	if !state.Settings.Interactivity.MessageMenuOptionsURL.IsNull() {
		t.Errorf("Interactivity.MessageMenuOptionsURL = %v, want null", state.Settings.Interactivity.MessageMenuOptionsURL)
	}
	if !state.Settings.Interactivity.IsEnabled.ValueBool() {
		t.Errorf("Interactivity.IsEnabled = %v, want true", state.Settings.Interactivity.IsEnabled)
	}
	if !state.Settings.OrgDeployEnabled.IsNull() {
		t.Errorf("OrgDeployEnabled = %v, want null", state.Settings.OrgDeployEnabled)
	}
	if !state.Settings.AllowedIPAddressRanges.IsNull() {
		t.Errorf("AllowedIPAddressRanges = %v, want null", state.Settings.AllowedIPAddressRanges)
	}
}

func TestStateFromManifest_PriorExplicitZeroValuesAreKept(t *testing.T) {
	t.Parallel()

	manifest := &slack.Manifest{
		Display: slack.Display{Name: "test"},
		Settings: slack.Settings{
			Interactivity: slack.Interactivity{IsEnabled: true},
		},
	}
	prior := ResourceAppState{
		ID: types.StringValue("A1"),
		Settings: &AppSettings{
			Interactivity: &AppInteractivity{
				IsEnabled:  types.BoolValue(true),
				RequestURL: types.StringValue(""),
			},
			SocketModeEnabled: types.BoolValue(false),
		},
	}

	state, diags := stateFromManifest(context.Background(), manifest, prior)
	if diags.HasError() {
		t.Errorf("stateFromManifest() diagnostics: %v", diags)
		return
	}

	if state.Settings.Interactivity.RequestURL.IsNull() || state.Settings.Interactivity.RequestURL.ValueString() != "" {
		t.Errorf("Interactivity.RequestURL = %v, want explicit empty string", state.Settings.Interactivity.RequestURL)
	}
	if state.Settings.SocketModeEnabled.IsNull() || state.Settings.SocketModeEnabled.ValueBool() {
		t.Errorf("SocketModeEnabled = %v, want explicit false", state.Settings.SocketModeEnabled)
	}
}

func TestStateFromManifest_RemoteDriftOverridesPrior(t *testing.T) {
	t.Parallel()

	manifest := &slack.Manifest{
		Display: slack.Display{Name: "renamed"},
		Features: slack.Features{
			BotUser: slack.BotUser{DisplayName: "bot", AlwaysOnline: true},
		},
	}
	prior := ResourceAppState{
		ID: types.StringValue("A1"),
		DisplayInformation: &AppDisplayInformation{
			Name: types.StringValue("test"),
		},
		Features: &AppFeatures{
			BotUser: &AppBotUser{
				DisplayName: types.StringValue("bot"),
			},
		},
	}

	state, diags := stateFromManifest(context.Background(), manifest, prior)
	if diags.HasError() {
		t.Errorf("stateFromManifest() diagnostics: %v", diags)
		return
	}

	if got := state.DisplayInformation.Name.ValueString(); got != "renamed" {
		t.Errorf("Name = %q, want %q", got, "renamed")
	}
	if !state.Features.BotUser.AlwaysOnline.ValueBool() {
		t.Errorf("AlwaysOnline = %v, want true", state.Features.BotUser.AlwaysOnline)
	}
	if state.Settings != nil {
		t.Errorf("Settings = %+v, want nil", state.Settings)
	}
}

func TestStateFromManifest_PriorBlockKeptWhenRemoteZero(t *testing.T) {
	t.Parallel()

	manifest := &slack.Manifest{
		Display: slack.Display{Name: "test"},
	}
	prior := ResourceAppState{
		ID: types.StringValue("A1"),
		Settings: &AppSettings{
			SocketModeEnabled: types.BoolValue(false),
		},
	}

	state, diags := stateFromManifest(context.Background(), manifest, prior)
	if diags.HasError() {
		t.Errorf("stateFromManifest() diagnostics: %v", diags)
		return
	}

	if state.Settings == nil {
		t.Error("Settings = nil, want the prior block kept")
		return
	}
	if state.Settings.SocketModeEnabled.IsNull() || state.Settings.SocketModeEnabled.ValueBool() {
		t.Errorf("SocketModeEnabled = %v, want explicit false", state.Settings.SocketModeEnabled)
	}
}
