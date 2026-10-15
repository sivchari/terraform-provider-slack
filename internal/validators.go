package internal

import (
	"context"
	"fmt"
	"slices"
	"strconv"
	"strings"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

// uniqueStrings rejects a list of strings that holds the same value twice.
func uniqueStrings() validator.List {
	return uniqueStringsValidator{}
}

type uniqueStringsValidator struct{}

func (uniqueStringsValidator) Description(context.Context) string {
	return "elements must be unique"
}

func (v uniqueStringsValidator) MarkdownDescription(ctx context.Context) string {
	return v.Description(ctx)
}

func (uniqueStringsValidator) ValidateList(_ context.Context, req validator.ListRequest, resp *validator.ListResponse) {
	if req.ConfigValue.IsNull() || req.ConfigValue.IsUnknown() {
		return
	}
	seen := make(map[string]struct{})
	for _, elem := range req.ConfigValue.Elements() {
		s, ok := elem.(types.String)
		if !ok || s.IsNull() || s.IsUnknown() {
			continue
		}
		if _, dup := seen[s.ValueString()]; dup {
			resp.Diagnostics.AddAttributeError(
				req.Path,
				"Duplicate list element",
				fmt.Sprintf("The value %q appears more than once. Each element must be unique.", s.ValueString()),
			)
			return
		}
		seen[s.ValueString()] = struct{}{}
	}
}

// exactlyOneOf rejects a data source configuration unless exactly one of the
// named string attributes is set.
//
// The check is skipped while any of them is unknown, since it cannot be
// decided before apply.
func exactlyOneOf(names ...string) datasource.ConfigValidator {
	return exactlyOneOfValidator{names: names}
}

type exactlyOneOfValidator struct {
	names []string
}

func (v exactlyOneOfValidator) Description(context.Context) string {
	return "exactly one of " + strings.Join(v.names, ", ") + " must be set"
}

func (v exactlyOneOfValidator) MarkdownDescription(ctx context.Context) string {
	return v.Description(ctx)
}

func (v exactlyOneOfValidator) ValidateDataSource(ctx context.Context, req datasource.ValidateConfigRequest, resp *datasource.ValidateConfigResponse) {
	set := 0
	for _, name := range v.names {
		var value types.String
		resp.Diagnostics.Append(req.Config.GetAttribute(ctx, path.Root(name), &value)...)
		if resp.Diagnostics.HasError() {
			return
		}
		if value.IsUnknown() {
			return
		}
		if !value.IsNull() {
			set++
		}
	}
	if set == 1 {
		return
	}
	resp.Diagnostics.AddError(
		"Invalid attribute combination",
		fmt.Sprintf("%s, but %d are set.", v.Description(ctx), set),
	)
}

// oneOfString rejects string values other than the allowed ones.
func oneOfString(allowed ...string) validator.String {
	return oneOfStringValidator{allowed: allowed}
}

type oneOfStringValidator struct {
	allowed []string
}

func (v oneOfStringValidator) Description(context.Context) string {
	quoted := make([]string, len(v.allowed))
	for i, a := range v.allowed {
		quoted[i] = strconv.Quote(a)
	}
	return "value must be one of: " + strings.Join(quoted, ", ")
}

func (v oneOfStringValidator) MarkdownDescription(ctx context.Context) string {
	return v.Description(ctx)
}

func (v oneOfStringValidator) ValidateString(ctx context.Context, req validator.StringRequest, resp *validator.StringResponse) {
	if req.ConfigValue.IsNull() || req.ConfigValue.IsUnknown() {
		return
	}
	if slices.Contains(v.allowed, req.ConfigValue.ValueString()) {
		return
	}
	resp.Diagnostics.AddAttributeError(
		req.Path,
		"Invalid attribute value",
		fmt.Sprintf("%s, got %q.", v.Description(ctx), req.ConfigValue.ValueString()),
	)
}
