package internal

import (
	"slices"
	"strings"

	"github.com/hashicorp/terraform-plugin-framework/resource/schema"

	"github.com/sivchari/terraform-provider-slack/internal/appmanifest"
)

// manifestGroups lists the top-level manifest keys the resource schema
// models. managedManifestPaths only walks attributes nested under these
// groups, so adding a schema attribute that models a new top-level manifest
// key requires adding it here too, and a future non-manifest top-level
// attribute (for example a timeouts block) never leaks into the payload
// just because it happens not to be computed-only.
var manifestGroups = map[string]struct{}{
	"display_information": {},
	"features":            {},
	"oauth_config":        {},
	"settings":            {},
}

// managedManifestPaths lists the manifest key paths the resource schema
// manages, derived from the schema so that adding an attribute automatically
// makes its manifest field managed. Attribute names match manifest keys.
// Only attributes nested under manifestGroups are considered; computed-only
// attributes (id, credentials) are not manifest fields; single nested
// attributes are walked into; every other attribute, including list nested
// attributes, is a leaf that is replaced as a whole.
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
// current: a managed path present in planned takes the planned value, a
// managed path absent from planned is removed, and every subtree of current
// outside the managed paths is copied into the result byte-for-byte,
// untouched. This is what lets apps.manifest.update, which replaces the
// whole manifest, be called without dropping the fields this provider does
// not model.
//
// Cleanup of objects the merge left empty only ever walks the ancestor
// chain of a managed path (see pruneManagedAncestors): an object made empty
// by the merge is removed (Slack treats an absent object and an empty one
// the same, and rejects some empty ones, see UpdateAppManifest), and a
// features.bot_user or settings.interactivity left in its zero form (see
// isZeroManifestObject) is removed the same way. A key that is not on any
// managed path is never visited, so an unmanaged object that happens to be
// empty, or named "bot_user" / "interactivity" at some unrelated depth,
// survives untouched.
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

// pruneManagedAncestors walks path's ancestor objects in obj as far as they
// exist and, on the way back up, drops one the merge left empty or in its
// zero form (isZeroManifestObject). It only ever looks at keys that are on
// path, so it can never reach into a subtree the merge did not touch.
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
