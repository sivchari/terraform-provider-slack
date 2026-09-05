package internal

import (
	"slices"
	"strings"

	"github.com/hashicorp/terraform-plugin-framework/resource/schema"

	"github.com/sivchari/terraform-provider-slack/internal/appmanifest"
)

// manifestGroups are the top-level manifest keys the schema models. A new
// top-level group must be added here; any other top-level attribute (a
// timeouts block, say) is not a manifest field and stays out of the payload.
var manifestGroups = map[string]struct{}{
	"display_information": {},
	"features":            {},
	"oauth_config":        {},
	"settings":            {},
}

// managedManifestPaths derives the manifest key paths the schema manages,
// so adding an attribute makes its manifest field managed. Attribute names
// match manifest keys; computed-only attributes (id, credentials) are
// skipped; single nested attributes are walked into; everything else,
// including list nested attributes, is a leaf replaced as a whole.
func managedManifestPaths(attrs map[string]schema.Attribute) [][]string {
	var paths [][]string
	var walk func(attrs map[string]schema.Attribute, prefix []string)
	walk = func(attrs map[string]schema.Attribute, prefix []string) {
		for name, attr := range attrs {
			if attr.IsComputed() && !attr.IsOptional() && !attr.IsRequired() {
				continue
			}
			if len(prefix) == 0 {
				if _, ok := manifestGroups[name]; !ok {
					continue
				}
			}
			path := append(slices.Clone(prefix), name)
			if nested, ok := attr.(schema.SingleNestedAttribute); ok {
				walk(nested.Attributes, path)
				continue
			}
			paths = append(paths, path)
		}
	}
	walk(attrs, nil)
	slices.SortFunc(paths, func(a, b []string) int {
		return strings.Compare(strings.Join(a, "."), strings.Join(b, "."))
	})
	return paths
}

// mergeManifest overlays the managed paths of planned onto a copy of
// current: a managed path present in planned takes the planned value, one
// absent from planned is removed, and everything else in current is left
// untouched. This is what lets apps.manifest.update, which replaces the
// whole manifest, be called without dropping the fields this provider does
// not model. Objects the merge left empty or in their zero form are dropped
// along the managed paths only, so unmanaged subtrees are never visited.
func mergeManifest(current, planned appmanifest.Document, managed [][]string) appmanifest.Document {
	merged := cloneObject(current)
	for _, path := range managed {
		if value, ok := lookupPath(planned, path); ok {
			setPath(merged, path, value)
		} else {
			deletePath(merged, path)
		}
	}
	for _, path := range managed {
		pruneManagedAncestors(merged, path)
	}
	return merged
}

func cloneObject(obj map[string]any) map[string]any {
	out := make(map[string]any, len(obj))
	for key, value := range obj {
		out[key] = cloneValue(value)
	}
	return out
}

func cloneValue(value any) any {
	switch v := value.(type) {
	case map[string]any:
		return cloneObject(v)
	case []any:
		out := make([]any, len(v))
		for i, item := range v {
			out[i] = cloneValue(item)
		}
		return out
	default:
		return v
	}
}

func lookupPath(obj map[string]any, path []string) (any, bool) {
	for i, key := range path {
		value, ok := obj[key]
		if !ok {
			return nil, false
		}
		if i == len(path)-1 {
			return value, true
		}
		if obj, ok = value.(map[string]any); !ok {
			return nil, false
		}
	}
	return nil, false
}

func setPath(obj map[string]any, path []string, value any) {
	for _, key := range path[:len(path)-1] {
		child, ok := obj[key].(map[string]any)
		if !ok {
			child = map[string]any{}
			obj[key] = child
		}
		obj = child
	}
	obj[path[len(path)-1]] = cloneValue(value)
}

func deletePath(obj map[string]any, path []string) {
	for _, key := range path[:len(path)-1] {
		child, ok := obj[key].(map[string]any)
		if !ok {
			return
		}
		obj = child
	}
	delete(obj, path[len(path)-1])
}

// pruneManagedAncestors walks path as far as it exists and, on the way back
// up, drops an object the merge left empty or in its zero form.
func pruneManagedAncestors(obj map[string]any, path []string) {
	if len(path) <= 1 {
		return
	}
	key := path[0]
	child, ok := obj[key].(map[string]any)
	if !ok {
		return
	}
	pruneManagedAncestors(child, path[1:])
	if isZeroManifestObject(key, child) {
		delete(obj, key)
	}
}
