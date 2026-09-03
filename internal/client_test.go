package internal

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/slack-go/slack"
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

func TestClientUpdateManifestContext(t *testing.T) {
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

	resp, err := client.UpdateManifestContext(context.Background(), &slack.Manifest{
		Display:  slack.Display{Name: "test"},
		Settings: slack.Settings{SocketModeEnabled: false},
	}, "", "A012345678")
	if err != nil {
		t.Errorf("UpdateManifestContext() error = %v", err)
		return
	}
	if !resp.PermissionsUpdated {
		t.Error("PermissionsUpdated = false, want true")
	}
	for _, key := range []string{"event_subscriptions", "interactivity", "bot_user"} {
		if strings.Contains(gotManifest, key) {
			t.Errorf("manifest %q contains empty %q object, want it omitted", gotManifest, key)
		}
	}
}

func TestClientUpdateManifestContext_Error(t *testing.T) {
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

	_, err := client.UpdateManifestContext(context.Background(), &slack.Manifest{
		Display: slack.Display{Name: "test"},
	}, "", "A012345678")
	if err == nil {
		t.Error("UpdateManifestContext() error = nil, want an error")
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
			want: `{"display_information":{"name":"test"},"features":{"bot_user":{"display_name":"bot"}},"settings":{"interactivity":{"is_enabled":true,"request_url":"https://example.com/i"},"socket_mode_enabled":true}}`,
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
