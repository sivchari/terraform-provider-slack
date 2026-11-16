package internal

import (
	"context"
	"fmt"
	"slices"
	"strconv"
	"strings"
	"unicode/utf8"

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

// nonEmptyList rejects a list without elements, which Slack cannot apply as a
// member list; omitting the attribute leaves membership unmanaged instead.
func nonEmptyList() validator.List {
	return nonEmptyListValidator{}
}

type nonEmptyListValidator struct{}

func (nonEmptyListValidator) Description(context.Context) string {
	return "list must not be empty"
}

func (v nonEmptyListValidator) MarkdownDescription(ctx context.Context) string {
	return v.Description(ctx)
}

func (nonEmptyListValidator) ValidateList(_ context.Context, req validator.ListRequest, resp *validator.ListResponse) {
	if req.ConfigValue.IsNull() || req.ConfigValue.IsUnknown() {
		return
	}
	if len(req.ConfigValue.Elements()) > 0 {
		return
	}
	resp.Diagnostics.AddAttributeError(
		req.Path,
		"Empty list",
		fmt.Sprintf("Omit %s to leave membership unmanaged. Slack cannot apply an empty member list.", req.Path),
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

// stringLengthBetween rejects a string whose length is outside
// [minLength, maxLength]. A minLength of 0 sets only an upper bound.
//
// The length is counted in Unicode code points rather than bytes, as Slack
// does for app manifest fields: a 109-character Japanese long_description,
// 253 bytes in UTF-8, was rejected as shorter than 175 characters.
func stringLengthBetween(minLength, maxLength int) validator.String {
	return stringLengthBetweenValidator{minLength: minLength, maxLength: maxLength}
}

type stringLengthBetweenValidator struct {
	minLength, maxLength int
}

func (v stringLengthBetweenValidator) requirement() string {
	if v.minLength == 0 {
		return fmt.Sprintf("at most %d characters", v.maxLength)
	}
	return fmt.Sprintf("between %d and %d characters", v.minLength, v.maxLength)
}

func (v stringLengthBetweenValidator) Description(context.Context) string {
	return "length must be " + v.requirement()
}

func (v stringLengthBetweenValidator) MarkdownDescription(ctx context.Context) string {
	return v.Description(ctx)
}

func (v stringLengthBetweenValidator) ValidateString(_ context.Context, req validator.StringRequest, resp *validator.StringResponse) {
	if req.ConfigValue.IsNull() || req.ConfigValue.IsUnknown() {
		return
	}
	length := utf8.RuneCountInString(req.ConfigValue.ValueString())
	if length >= v.minLength && length <= v.maxLength {
		return
	}
	resp.Diagnostics.AddAttributeError(
		req.Path,
		"Invalid attribute value length",
		fmt.Sprintf("The value has %d characters, but Slack accepts %s.", length, v.requirement()),
	)
}
