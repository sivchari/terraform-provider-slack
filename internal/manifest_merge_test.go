package internal

import (
	"context"
	"reflect"
	"slices"
	"strings"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/resource"

	"github.com/sivchari/terraform-provider-slack/internal/appmanifest"
)

func TestManagedManifestPaths(t *testing.T) {
	t.Parallel()

	var res resource.SchemaResponse
	(&ResourceApp{}).Schema(context.Background(), resource.SchemaRequest{}, &res)

	paths := managedManifestPaths(res.Schema.Attributes)
	got := make([]string, 0, len(paths))
	for _, path := range paths {
		got = append(got, strings.Join(path, "."))
	}

	for _, want := range []string{
		"display_information.name",
		"display_information.description",
		"features.bot_user.always_online",
		"features.slash_commands",
		"oauth_config.scopes.bot",
		"settings.interactivity.is_enabled",
		"settings.socket_mode_enabled",
	} {
		if !slices.Contains(got, want) {
			t.Errorf("managedManifestPaths() = %v, want it to contain %q", got, want)
		}
	}
	for _, unwanted := range []string{
		"id",
		"client_secret",
		"features",
		"features.slash_commands.command",
	} {
		if slices.Contains(got, unwanted) {
			t.Errorf("managedManifestPaths() = %v, want it not to contain %q", got, unwanted)
		}
	}
	if !slices.IsSorted(got) {
		t.Errorf("managedManifestPaths() = %v, want sorted", got)
	}
}

func TestMergeManifest(t *testing.T) {
	t.Parallel()

	managed := [][]string{
		{"display_information", "name"},
		{"display_information", "description"},
		{"features", "bot_user", "display_name"},
		{"features", "bot_user", "always_online"},
		{"features", "slash_commands"},
		{"settings", "interactivity", "is_enabled"},
		{"settings", "interactivity", "request_url"},
		{"settings", "socket_mode_enabled"},
	}

	tests := []struct {
		name    string
		current appmanifest.Document
		planned appmanifest.Document
		want    appmanifest.Document
	}{
		{
			name: "fields outside the schema survive",
			current: appmanifest.Document{
				"_metadata":           map[string]any{"major_version": float64(1)},
				"display_information": map[string]any{"name": "old"},
				"features": map[string]any{
					"assistant_view": map[string]any{"assistant_description": "Ask me anything"},
					"bot_user":       map[string]any{"display_name": "old-bot"},
				},
				"settings": map[string]any{"token_rotation_enabled": false},
			},
			planned: appmanifest.Document{
				"display_information": map[string]any{"name": "new"},
				"features": map[string]any{
					"bot_user": map[string]any{"display_name": "new-bot"},
				},
			},
			want: appmanifest.Document{
				"_metadata":           map[string]any{"major_version": float64(1)},
				"display_information": map[string]any{"name": "new"},
				"features": map[string]any{
					"assistant_view": map[string]any{"assistant_description": "Ask me anything"},
					"bot_user":       map[string]any{"display_name": "new-bot"},
				},
				"settings": map[string]any{"token_rotation_enabled": false},
			},
		},
		{
			name: "managed fields absent from the plan are removed",
			current: appmanifest.Document{
				"display_information": map[string]any{"name": "test", "description": "old"},
				"features": map[string]any{
					"bot_user":       map[string]any{"display_name": "bot", "always_online": true},
					"slash_commands": []any{map[string]any{"command": "/old", "description": "old"}},
				},
				"settings": map[string]any{
					"interactivity":       map[string]any{"is_enabled": true, "request_url": "https://example.com/i"},
					"socket_mode_enabled": true,
				},
			},
			planned: appmanifest.Document{
				"display_information": map[string]any{"name": "test"},
				"features": map[string]any{
					"bot_user": map[string]any{"display_name": "bot"},
				},
				"settings": map[string]any{
					"interactivity": map[string]any{"is_enabled": false},
				},
			},
			want: appmanifest.Document{
				"display_information": map[string]any{"name": "test"},
				"features": map[string]any{
					"bot_user": map[string]any{"display_name": "bot"},
				},
				"settings": map[string]any{
					"interactivity": map[string]any{"is_enabled": false},
				},
			},
		},
		{
			name: "list attributes are replaced as a whole",
			current: appmanifest.Document{
				"display_information": map[string]any{"name": "test"},
				"features": map[string]any{
					"slash_commands": []any{
						map[string]any{"command": "/a", "description": "a"},
						map[string]any{"command": "/b", "description": "b"},
					},
				},
			},
			planned: appmanifest.Document{
				"display_information": map[string]any{"name": "test"},
				"features": map[string]any{
					"slash_commands": []any{map[string]any{"command": "/c", "description": "c"}},
				},
			},
			want: appmanifest.Document{
				"display_information": map[string]any{"name": "test"},
				"features": map[string]any{
					"slash_commands": []any{map[string]any{"command": "/c", "description": "c"}},
				},
			},
		},
		{
			name: "missing parent objects are created",
			current: appmanifest.Document{
				"display_information": map[string]any{"name": "test"},
			},
			planned: appmanifest.Document{
				"display_information": map[string]any{"name": "test"},
				"settings":            map[string]any{"socket_mode_enabled": true},
			},
			want: appmanifest.Document{
				"display_information": map[string]any{"name": "test"},
				"settings":            map[string]any{"socket_mode_enabled": true},
			},
		},
		{
			name: "objects emptied by the merge are dropped",
			current: appmanifest.Document{
				"display_information": map[string]any{"name": "test"},
				"settings": map[string]any{
					"interactivity": map[string]any{"is_enabled": true, "request_url": "https://example.com/i"},
				},
			},
			planned: appmanifest.Document{
				"display_information": map[string]any{"name": "test"},
			},
			want: appmanifest.Document{
				"display_information": map[string]any{"name": "test"},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got := mergeManifest(tt.current, tt.planned, managed)
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("mergeManifest() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestMergeManifest_DoesNotMutateInputs(t *testing.T) {
	t.Parallel()

	current := appmanifest.Document{
		"display_information": map[string]any{"name": "old", "description": "old"},
	}
	planned := appmanifest.Document{
		"display_information": map[string]any{"name": "new"},
	}

	mergeManifest(current, planned, [][]string{{"display_information", "name"}, {"display_information", "description"}})

	if got := current["display_information"].(map[string]any); got["name"] != "old" || got["description"] != "old" {
		t.Errorf("current = %v, want it untouched", current)
	}
}
