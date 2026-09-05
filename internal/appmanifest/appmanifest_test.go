package appmanifest

import (
	"testing"

	"github.com/slack-go/slack"
)

func TestNewDocument(t *testing.T) {
	t.Parallel()

	manifest := &Manifest{
		Manifest: slack.Manifest{
			Display: slack.Display{Name: "test"},
		},
		Features: Features{
			Features: slack.Features{
				BotUser: slack.BotUser{DisplayName: "bot"},
			},
			AssistantView: &AssistantView{
				AssistantDescription: "Ask me anything",
				SuggestedPrompts: []SuggestedPrompt{
					{Title: "Summarize", Message: "Summarize this thread"},
				},
				Actions: []AssistantAction{
					{Name: "summarize", Description: "Summarize the selected message"},
				},
			},
		},
	}

	doc, err := NewDocument(manifest)
	if err != nil {
		t.Errorf("NewDocument() error = %v", err)
		return
	}

	features, _ := doc["features"].(map[string]any)
	botUser, _ := features["bot_user"].(map[string]any)
	if got := botUser["display_name"]; got != "bot" {
		t.Errorf("features.bot_user.display_name = %v, want %q", got, "bot")
	}
	assistantView, _ := features["assistant_view"].(map[string]any)
	if got := assistantView["assistant_description"]; got != "Ask me anything" {
		t.Errorf("features.assistant_view.assistant_description = %v, want %q", got, "Ask me anything")
	}
	prompts, _ := assistantView["suggested_prompts"].([]any)
	if len(prompts) != 1 {
		t.Errorf("features.assistant_view.suggested_prompts = %v, want one prompt", prompts)
	}
	actions, _ := assistantView["actions"].([]any)
	if len(actions) != 1 {
		t.Errorf("features.assistant_view.actions = %v, want one action", actions)
	}
}

func TestNewDocument_OmitsNilAssistantView(t *testing.T) {
	t.Parallel()

	manifest := &Manifest{
		Manifest: slack.Manifest{Display: slack.Display{Name: "test"}},
	}

	doc, err := NewDocument(manifest)
	if err != nil {
		t.Errorf("NewDocument() error = %v", err)
		return
	}

	features, _ := doc["features"].(map[string]any)
	if _, ok := features["assistant_view"]; ok {
		t.Errorf("features = %v, want no assistant_view key", features)
	}
}

func TestDocumentManifest(t *testing.T) {
	t.Parallel()

	doc := Document{
		"display_information": map[string]any{"name": "test"},
		"features": map[string]any{
			"bot_user": map[string]any{"display_name": "bot", "always_online": true},
			"assistant_view": map[string]any{
				"assistant_description": "Ask me anything",
				"suggested_prompts": []any{
					map[string]any{"title": "Summarize", "message": "Summarize this thread"},
				},
			},
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
	if manifest.Features.AssistantView == nil {
		t.Error("Features.AssistantView = nil, want it decoded")
		return
	}
	if manifest.Features.AssistantView.AssistantDescription != "Ask me anything" {
		t.Errorf("AssistantDescription = %q", manifest.Features.AssistantView.AssistantDescription)
	}
	if len(manifest.Features.AssistantView.SuggestedPrompts) != 1 ||
		manifest.Features.AssistantView.SuggestedPrompts[0].Message != "Summarize this thread" {
		t.Errorf("SuggestedPrompts = %+v", manifest.Features.AssistantView.SuggestedPrompts)
	}
}
