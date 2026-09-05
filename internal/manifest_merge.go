package internal

import (
	"slices"
	"strings"

	"github.com/hashicorp/terraform-plugin-framework/resource/schema"

	"github.com/sivchari/terraform-provider-slack/internal/appmanifest"
)

// managedManifestPaths lists the manifest key paths the resource schema
// manages, derived from the schema so that adding an attribute automatically
// makes its manifest field managed. Attribute names match manifest keys.
// Computed-only attributes (id, credentials) are not manifest fields;
// single nested attributes are walked into; every other attribute, including
// list nested attributes, is a leaf that is replaced as a whole.
func managedManifestPaths(attrs map[string]schema.Attribute) [][]string {
	var paths [][]string
	var walk func(attrs map[string]schema.Attribute, prefix []string)
	walk = func(attrs map[string]schema.Attribute, prefix []string) {
		for name, attr := range attrs {
			if attr.IsComputed() && !attr.IsOptional() && !attr.IsRequired() {
				continue
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
// managed path absent from planned is removed, and everything else in
// current is left untouched. This is what lets apps.manifest.update, which
// replaces the whole manifest, be called without dropping the fields this
// provider does not model. Objects left empty by the merge are removed, as
// Slack treats an absent object and an empty one the same (and rejects some
// empty ones, see marshalDocument).
func mergeManifest(current, planned appmanifest.Document, managed [][]string) appmanifest.Document {
	merged := cloneObject(current)
	for _, path := range managed {
		if value, ok := lookupPath(planned, path); ok {
			setPath(merged, path, value)
		} else {
			deletePath(merged, path)
		}
	}
	pruneEmptyObjects(merged)
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

func pruneEmptyObjects(obj map[string]any) {
	for key, value := range obj {
		child, ok := value.(map[string]any)
		if !ok {
			continue
		}
		pruneEmptyObjects(child)
		if len(child) == 0 {
			delete(obj, key)
		}
	}
}
