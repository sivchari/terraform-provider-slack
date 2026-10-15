package internal

import (
	"context"
	"fmt"
	"slices"
	"strconv"
	"strings"

	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
)

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
