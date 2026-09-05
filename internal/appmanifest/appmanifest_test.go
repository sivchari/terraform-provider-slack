package appmanifest

import (
	"testing"

	"github.com/slack-go/slack"
)

func TestNewDocument(t *testing.T) {
	t.Parallel()

	doc, err := NewDocument(&slack.Manifest{
		Display: slack.Display{Name: "test"},
		Features: slack.Features{
			BotUser: slack.BotUser{DisplayName: "bot"},
		},
	})
	if err != nil {
		t.Errorf("NewDocument() error = %v", err)
		return
	}

	features, _ := doc["features"].(map[string]any)
	botUser, _ := features["bot_user"].(map[string]any)
	if got := botUser["display_name"]; got != "bot" {
		t.Errorf("features.bot_user.display_name = %v, want %q", got, "bot")
	}
}

func TestDocumentManifest(t *testing.T) {
	t.Parallel()

	doc := Document{
		"display_information": map[string]any{"name": "test"},
		"features": map[string]any{
			"bot_user":       map[string]any{"display_name": "bot", "always_online": true},
			"unfurl_domains": []any{"example.com"},
		},
		"settings": map[string]any{"token_rotation_enabled": true},
	}

	manifest, err := doc.Manifest()
	if err != nil {
		t.Errorf("Manifest() error = %v", err)
		return
	}

	if manifest.Display.Name != "test" {
		t.Errorf("Display.Name = %q, want %q", manifest.Display.Name, "test")
	}
	if !manifest.Features.BotUser.AlwaysOnline {
		t.Errorf("Features.BotUser = %+v, want always_online true", manifest.Features.BotUser)
	}
}
