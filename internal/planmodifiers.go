package internal

import (
	"context"

	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
)

// createOnlyString carries the state value of a computed attribute into the
// plan even when that value is null. stringplanmodifier.UseStateForUnknown
// does nothing for null state values, so attributes that only
// apps.manifest.create returns (and that are therefore null after an import)
// would otherwise show as (known after apply) on every plan.
func createOnlyString() planmodifier.String {
	return createOnlyStringModifier{}
}

type createOnlyStringModifier struct{}

func (createOnlyStringModifier) Description(context.Context) string {
	return "The value is determined once at create time and never changes afterwards."
}

func (m createOnlyStringModifier) MarkdownDescription(ctx context.Context) string {
	return m.Description(ctx)
}

func (createOnlyStringModifier) PlanModifyString(_ context.Context, req planmodifier.StringRequest, resp *planmodifier.StringResponse) {
	if req.State.Raw.IsNull() || req.Plan.Raw.IsNull() {
		return
	}
	if !req.PlanValue.IsUnknown() {
		return
	}
	resp.PlanValue = req.StateValue
}
