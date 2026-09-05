package internal

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/slack-go/slack"

	"github.com/sivchari/terraform-provider-slack/internal/appmanifest"
)

func TestClientCreateAppManifest(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if err := r.ParseForm(); err != nil {
			t.Errorf("failed to parse form: %v", err)
		}
		if got := r.FormValue("token"); got != "config-token" {
			t.Errorf("token = %q, want %q", got, "config-token")
		}

		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{
			"ok": true,
			"app_id": "A012345678",
			"credentials": {
				"client_id": "1234567890.1234567890123",
				"client_secret": "abcdefghijklmnopqrstuvwxyz012345",
				"verification_token": "abcdefghijklmnopqrstuvwx",
				"signing_secret": "0123456789abcdef0123456789abcdef"
			},
			"oauth_authorize_url": "https://slack.com/oauth/v2/authorize?client_id=1234567890.1234567890123"
		}`))
	}))
	t.Cleanup(server.Close)

	client := NewClient("bot-token", "")
	client.apiURL = server.URL + "/"

	resp, err := client.CreateAppManifest(context.Background(), &slack.Manifest{
		Display: slack.Display{Name: "test"},
	}, "config-token")
	if err != nil {
		t.Errorf("CreateAppManifest() error = %v", err)
		return
	}

	if resp.AppID != "A012345678" {
		t.Errorf("AppID = %q, want %q", resp.AppID, "A012345678")
	}
	if resp.Credentials.ClientSecret != "abcdefghijklmnopqrstuvwxyz012345" {
		t.Errorf("ClientSecret = %q, want %q", resp.Credentials.ClientSecret, "abcdefghijklmnopqrstuvwxyz012345")
	}
	if resp.OAuthAuthorizeURL != "https://slack.com/oauth/v2/authorize?client_id=1234567890.1234567890123" {
		t.Errorf("OAuthAuthorizeURL = %q", resp.OAuthAuthorizeURL)
	}
}

func TestClientCreateAppManifest_Error(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{
			"ok": false,
			"error": "invalid_manifest",
			"errors": [{"message": "name is required", "pointer": "/display_information/name"}]
		}`))
	}))
	t.Cleanup(server.Close)

	client := NewClient("bot-token", "config-token")
	client.apiURL = server.URL + "/"

	_, err := client.CreateAppManifest(context.Background(), &slack.Manifest{}, "")
	if err == nil {
		t.Error("CreateAppManifest() error = nil, want an error")
		return
	}
	if !strings.Contains(err.Error(), "invalid_manifest") ||
		!strings.Contains(err.Error(), "/display_information/name: name is required") {
		t.Errorf("error = %q, want the code and pointer: message details", err.Error())
	}
}

func TestClientUpdateAppManifest(t *testing.T) {
	t.Parallel()

	var gotManifest string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if err := r.ParseForm(); err != nil {
			t.Errorf("failed to parse form: %v", err)
		}
		if got := r.FormValue("app_id"); got != "A012345678" {
			t.Errorf("app_id = %q, want %q", got, "A012345678")
		}
		if got := r.FormValue("token"); got != "config-token" {
			t.Errorf("token = %q, want %q", got, "config-token")
		}
		gotManifest = r.FormValue("manifest")

		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"ok": true, "app_id": "A012345678", "permissions_updated": true}`))
	}))
	t.Cleanup(server.Close)

	client := NewClient("bot-token", "config-token")
	client.apiURL = server.URL + "/"

	resp, err := client.UpdateAppManifest(context.Background(), appmanifest.Document{
		"display_information": map[string]any{"name": "test"},
		"features": map[string]any{
			"unfurl_domains": []any{"example.com"},
			"bot_user":       map[string]any{"display_name": ""},
		},
		"settings": map[string]any{
			"event_subscriptions": map[string]any{},
			"interactivity":       map[string]any{"is_enabled": false},
		},
	}, "", "A012345678")
	if err != nil {
		t.Errorf("UpdateAppManifest() error = %v", err)
		return
	}
	if !resp.PermissionsUpdated {
		t.Error("PermissionsUpdated = false, want true")
	}
	want := `{"display_information":{"name":"test"},"features":{"unfurl_domains":["example.com"]}}`
	if gotManifest != want {
		t.Errorf("manifest = %s, want %s", gotManifest, want)
	}
}

func TestClientUpdateAppManifest_Error(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{
			"ok": false,
			"error": "invalid_manifest",
			"errors": [
				{"message": "Event Subscription requires a Request URL", "pointer": "/settings/event_subscriptions"},
				{"message": "Interactivity requires Socket Mode if no Request URL is provided", "pointer": "/settings/interactivity"}
			]
		}`))
	}))
	t.Cleanup(server.Close)

	client := NewClient("bot-token", "config-token")
	client.apiURL = server.URL + "/"

	_, err := client.UpdateAppManifest(context.Background(), appmanifest.Document{
		"display_information": map[string]any{"name": "test"},
	}, "", "A012345678")
	if err == nil {
		t.Error("UpdateAppManifest() error = nil, want an error")
		return
	}
	want := "apps.manifest.update: invalid_manifest\n" +
		"/settings/event_subscriptions: Event Subscription requires a Request URL\n" +
		"/settings/interactivity: Interactivity requires Socket Mode if no Request URL is provided"
	if err.Error() != want {
		t.Errorf("error = %q, want %q", err.Error(), want)
	}
}

func TestMarshalManifest(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		manifest *slack.Manifest
		want     string
	}{
		{
			name: "empty value structs are omitted",
			manifest: &slack.Manifest{
				Display:  slack.Display{Name: "test"},
				Settings: slack.Settings{SocketModeEnabled: false},
			},
			want: `{"display_information":{"name":"test"}}`,
		},
		{
			name: "disabled-only interactivity is omitted",
			manifest: &slack.Manifest{
				Display: slack.Display{Name: "test"},
				Settings: slack.Settings{
					Interactivity: slack.Interactivity{IsEnabled: false},
					EventSubscriptions: slack.EventSubscriptions{
						RequestUrl: "https://example.com/events",
					},
				},
			},
			want: `{"display_information":{"name":"test"},"settings":{"event_subscriptions":{"request_url":"https://example.com/events"}}}`,
		},
		{
			name: "configured blocks are kept",
			manifest: &slack.Manifest{
				Display: slack.Display{Name: "test"},
				Features: slack.Features{
					BotUser: slack.BotUser{DisplayName: "bot"},
				},
				Settings: slack.Settings{
					Interactivity:     slack.Interactivity{IsEnabled: true, RequestUrl: "https://example.com/i"},
					SocketModeEnabled: true,
				},
			},
			want: `{"display_information":{"name":"test"},"features":{"bot_user":{"display_name":"bot"}},` +
				`"settings":{"interactivity":{"is_enabled":true,"request_url":"https://example.com/i"},"socket_mode_enabled":true}}`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got, err := marshalManifest(tt.manifest)
			if err != nil {
				t.Errorf("marshalManifest() error = %v", err)
				return
			}
			if string(got) != tt.want {
				t.Errorf("marshalManifest() = %s, want %s", got, tt.want)
			}
		})
	}
}

func TestClientExportAppManifest(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if err := r.ParseForm(); err != nil {
			t.Errorf("failed to parse form: %v", err)
		}
		if got := r.FormValue("app_id"); got != "A012345678" {
			t.Errorf("app_id = %q, want %q", got, "A012345678")
		}
		if got := r.FormValue("token"); got != "config-token" {
			t.Errorf("token = %q, want %q", got, "config-token")
		}

		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{
			"ok": true,
			"manifest": {
				"_metadata": {"major_version": 1, "minor_version": 1},
				"display_information": {"name": "test"},
				"features": {
					"unfurl_domains": ["example.com"],
					"bot_user": {"display_name": "bot", "always_online": false}
				},
				"settings": {"token_rotation_enabled": false}
			}
		}`))
	}))
	t.Cleanup(server.Close)

	client := NewClient("bot-token", "config-token")
	client.apiURL = server.URL + "/"

	doc, err := client.ExportAppManifest(context.Background(), "", "A012345678")
	if err != nil {
		t.Errorf("ExportAppManifest() error = %v", err)
		return
	}

	// Keys outside slack.Manifest must survive: they are what the
	// read-merge-write update relies on.
	metadata, _ := doc["_metadata"].(map[string]any)
	if got := metadata["major_version"]; got != float64(1) {
		t.Errorf("_metadata.major_version = %v, want 1", got)
	}
	features, _ := doc["features"].(map[string]any)
	if _, ok := features["unfurl_domains"]; !ok {
		t.Errorf("features = %v, want unfurl_domains kept", features)
	}
	settings, _ := doc["settings"].(map[string]any)
	if got, ok := settings["token_rotation_enabled"]; !ok || got != false {
		t.Errorf("settings = %v, want token_rotation_enabled kept", settings)
	}
}

func TestClientExportAppManifest_NotFound(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"ok": false, "error": "app_not_found"}`))
	}))
	t.Cleanup(server.Close)

	client := NewClient("bot-token", "config-token")
	client.apiURL = server.URL + "/"

	_, err := client.ExportAppManifest(context.Background(), "", "A012345678")
	var apiErr *appmanifest.Error
	if !errors.As(err, &apiErr) {
		t.Errorf("ExportAppManifest() error = %v, want *appmanifest.Error", err)
		return
	}
	if apiErr.Code != "app_not_found" {
		t.Errorf("Code = %q, want %q", apiErr.Code, "app_not_found")
	}
	if got := err.Error(); got != "apps.manifest.export: app_not_found" {
		t.Errorf("error = %q, want %q", got, "apps.manifest.export: app_not_found")
	}
}
