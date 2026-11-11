package internal

import (
	"context"
	"reflect"
	"slices"
	"strings"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"

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
		"features.unfurl_domains",
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

func TestManagedManifestPaths_RestrictsToManifestGroups(t *testing.T) {
	t.Parallel()

	// A synthetic schema with a top-level attribute that isn't one of the
	// manifest groups the resource models (e.g. a future timeouts block):
	// it must never leak into the payload just because it isn't computed.
	attrs := map[string]schema.Attribute{
		"not_a_manifest_key": schema.StringAttribute{Optional: true},
		"display_information": schema.SingleNestedAttribute{
			Optional: true,
			Attributes: map[string]schema.Attribute{
				"name": schema.StringAttribute{Optional: true},
			},
		},
	}

	paths := managedManifestPaths(attrs)
	got := make([]string, 0, len(paths))
	for _, path := range paths {
		got = append(got, strings.Join(path, "."))
	}

	if slices.Contains(got, "not_a_manifest_key") {
		t.Errorf("managedManifestPaths() = %v, want it not to contain %q", got, "not_a_manifest_key")
	}
	if !slices.Contains(got, "display_information.name") {
		t.Errorf("managedManifestPaths() = %v, want it to contain %q", got, "display_information.name")
	}
}

func TestManifestGroups_MatchSchema(t *testing.T) {
	t.Parallel()

	var res resource.SchemaResponse
	(&ResourceApp{}).Schema(context.Background(), resource.SchemaRequest{}, &res)

	for name, attr := range res.Schema.Attributes {
		configurable := attr.IsOptional() || attr.IsRequired()
		_, isGroup := manifestGroups[name]
		if configurable && !isGroup {
			t.Errorf("top-level attribute %q is configurable but not in manifestGroups, so managedManifestPaths silently drops it", name)
		}
		if !configurable && isGroup {
			t.Errorf("manifestGroups contains %q but the schema's %q is computed-only", name, name)
		}
	}
	for name := range manifestGroups {
		if _, ok := res.Schema.Attributes[name]; !ok {
			t.Errorf("manifestGroups contains %q, which is not a top-level schema attribute", name)
		}
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
		{"settings", "event_subscriptions", "request_url"},
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
			// Note: settings disappears entirely here, not just
			// interactivity. socket_mode_enabled is also absent from
			// planned's settings block, so it is removed by the overlay
			// step; interactivity is left in its zero form ({"is_enabled":
			// false}) by the overlay and dropped by the zero-form rule,
			// which empties settings and drops it too.
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
		{
			// functions and features.search are not on any managed path, so
			// pruning must never descend into them even though they nest
			// empty objects the way marshalManifest's zero-form structs do.
			name: "unmanaged nested empty objects survive",
			current: appmanifest.Document{
				"display_information": map[string]any{"name": "test"},
				"functions": map[string]any{
					"f": map[string]any{
						"output_parameters": map[string]any{"properties": map[string]any{}, "required": []any{}},
					},
				},
				"features": map[string]any{
					"search": map[string]any{"slackbot_metadata": map[string]any{}},
				},
			},
			planned: appmanifest.Document{
				"display_information": map[string]any{"name": "test"},
			},
			want: appmanifest.Document{
				"display_information": map[string]any{"name": "test"},
				"functions": map[string]any{
					"f": map[string]any{
						"output_parameters": map[string]any{"properties": map[string]any{}, "required": []any{}},
					},
				},
				"features": map[string]any{
					"search": map[string]any{"slackbot_metadata": map[string]any{}},
				},
			},
		},
		{
			// "bot_user" is a managed key under features, but this bot_user
			// lives under functions, which is not a managed path at all:
			// isZeroManifestObject's key-name match must never be applied
			// off a managed path, however it is contrived to look.
			name: "unmanaged bot_user-named object outside the schema path is untouched",
			current: appmanifest.Document{
				"display_information": map[string]any{"name": "test"},
				"functions": map[string]any{
					"bot_user": map[string]any{"display_name": ""},
				},
			},
			planned: appmanifest.Document{
				"display_information": map[string]any{"name": "test"},
			},
			want: appmanifest.Document{
				"display_information": map[string]any{"name": "test"},
				"functions": map[string]any{
					"bot_user": map[string]any{"display_name": ""},
				},
			},
		},
		{
			// current has settings.event_subscriptions as an empty object
			// (Slack rejects that shape when Socket Mode is off); planned
			// has no settings at all, so the merge must remove it along
			// the ancestor chain, leaving no "settings" key.
			name: "empty export-only object on a managed path is dropped",
			current: appmanifest.Document{
				"display_information": map[string]any{"name": "test"},
				"settings":            map[string]any{"event_subscriptions": map[string]any{}},
			},
			planned: appmanifest.Document{
				"display_information": map[string]any{"name": "test"},
			},
			want: appmanifest.Document{
				"display_information": map[string]any{"name": "test"},
			},
		},
		{
			// planned drops the bot_user block entirely; the merge deletes
			// its only field (display_name) and the absent always_online is
			// a no-op, leaving an empty bot_user that must be pruned along
			// its ancestor chain without also dropping features, which
			// still has slash_commands.
			name: "zero-form bot_user left by the merge is dropped",
			current: appmanifest.Document{
				"display_information": map[string]any{"name": "test"},
				"features": map[string]any{
					"bot_user":       map[string]any{"display_name": "x"},
					"slash_commands": []any{map[string]any{"command": "/a", "description": "a"}},
				},
			},
			planned: appmanifest.Document{
				"display_information": map[string]any{"name": "test"},
				"features": map[string]any{
					"slash_commands": []any{map[string]any{"command": "/a", "description": "a"}},
				},
			},
			want: appmanifest.Document{
				"display_information": map[string]any{"name": "test"},
				"features": map[string]any{
					"slash_commands": []any{map[string]any{"command": "/a", "description": "a"}},
				},
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

func TestManagedPathsForPlan(t *testing.T) {
	t.Parallel()

	domains := types.ListValueMust(types.StringType, []attr.Value{types.StringValue("example.com")})
	tests := []struct {
		name        string
		features    *AppFeatures
		wantManaged bool
	}{
		{name: "features null", features: nil, wantManaged: false},
		{name: "unfurl_domains null", features: &AppFeatures{UnfurlDomains: types.ListNull(types.StringType)}, wantManaged: false},
		{name: "unfurl_domains empty", features: &AppFeatures{UnfurlDomains: types.ListValueMust(types.StringType, []attr.Value{})}, wantManaged: true},
		{name: "unfurl_domains set", features: &AppFeatures{UnfurlDomains: domains}, wantManaged: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			paths := managedPathsForPlan(ResourceAppState{Features: tt.features})
			got := slices.ContainsFunc(paths, func(path []string) bool {
				return slices.Equal(path, unfurlDomainsPath)
			})
			if got != tt.wantManaged {
				t.Errorf("managedPathsForPlan() manages features.unfurl_domains = %v, want %v", got, tt.wantManaged)
			}
			// the other managed paths are unaffected
			if want := len(managedManifestPathsOnce()); len(paths) != want && len(paths) != want-1 {
				t.Errorf("len(managedPathsForPlan()) = %d, want %d or %d", len(paths), want, want-1)
			}
		})
	}

	// the shared, cached slice must not be modified
	if !slices.ContainsFunc(managedManifestPathsOnce(), func(path []string) bool {
		return slices.Equal(path, unfurlDomainsPath)
	}) {
		t.Error("managedManifestPathsOnce() lost features.unfurl_domains")
	}
}

func TestMergeManifest_UnfurlDomains(t *testing.T) {
	t.Parallel()

	current := appmanifest.Document{
		"display_information": map[string]any{"name": "test"},
		"features": map[string]any{
			"bot_user":       map[string]any{"display_name": "bot"},
			"unfurl_domains": []any{"admin.example.com"},
		},
	}
	unset := managedPathsForPlan(ResourceAppState{})
	set := managedPathsForPlan(ResourceAppState{Features: &AppFeatures{
		UnfurlDomains: types.ListValueMust(types.StringType, []attr.Value{}),
	}})

	tests := []struct {
		name    string
		planned appmanifest.Document
		managed [][]string
		want    any
		wantOK  bool
	}{
		{
			name:    "unset keeps the exported domains",
			planned: appmanifest.Document{"display_information": map[string]any{"name": "test"}},
			managed: unset,
			want:    []any{"admin.example.com"},
			wantOK:  true,
		},
		{
			name: "set replaces the exported domains",
			planned: appmanifest.Document{
				"display_information": map[string]any{"name": "test"},
				"features":            map[string]any{"unfurl_domains": []any{"example.com"}},
			},
			managed: set,
			want:    []any{"example.com"},
			wantOK:  true,
		},
		{
			// [] is omitted from the planned document, so the managed path
			// is absent and the merge removes it.
			name:    "empty list removes the exported domains",
			planned: appmanifest.Document{"display_information": map[string]any{"name": "test"}},
			managed: set,
			wantOK:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			merged := mergeManifest(current, tt.planned, tt.managed)
			got, ok := lookupPath(merged, unfurlDomainsPath)
			if ok != tt.wantOK || !reflect.DeepEqual(got, tt.want) {
				t.Errorf("merged features.unfurl_domains = %v (present %v), want %v (present %v)", got, ok, tt.want, tt.wantOK)
			}
		})
	}
}
