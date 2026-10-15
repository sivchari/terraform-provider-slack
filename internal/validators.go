package internal

import (
	"context"
	"fmt"
	"slices"
	"strconv"
	"strings"

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
