package internal

import (
	"context"
	"net/http"
	"net/http/httptest"
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
	}
}
