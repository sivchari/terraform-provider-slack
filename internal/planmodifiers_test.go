package internal

import (
	"context"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/tfsdk"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-go/tftypes"
)

func TestCreateOnlyString_PlanModifyString(t *testing.T) {
	t.Parallel()

	objType := tftypes.Object{AttributeTypes: map[string]tftypes.Type{"id": tftypes.String}}
	nonNullRaw := tftypes.NewValue(objType, map[string]tftypes.Value{
		"id": tftypes.NewValue(tftypes.String, "x"),
	})
	nullRaw := tftypes.NewValue(objType, nil)

	tests := []struct {
		name     string
		stateRaw tftypes.Value
		planRaw  tftypes.Value
		stateVal types.String
		planVal  types.String
		wantPlan types.String
	}{
		{
			name:     "create keeps unknown",
			stateRaw: nullRaw,
			planRaw:  nonNullRaw,
			stateVal: types.StringNull(),
			planVal:  types.StringUnknown(),
			wantPlan: types.StringUnknown(),
		},
		{
			name:     "destroy keeps plan value",
			stateRaw: nonNullRaw,
			planRaw:  nullRaw,
			stateVal: types.StringValue("A123"),
			planVal:  types.StringNull(),
			wantPlan: types.StringNull(),
		},
		{
			name:     "update carries known state value",
			stateRaw: nonNullRaw,
			planRaw:  nonNullRaw,
			stateVal: types.StringValue("A123"),
			planVal:  types.StringUnknown(),
			wantPlan: types.StringValue("A123"),
		},
		{
			name:     "update carries null state value",
			stateRaw: nonNullRaw,
			planRaw:  nonNullRaw,
			stateVal: types.StringNull(),
			planVal:  types.StringUnknown(),
			wantPlan: types.StringNull(),
		},
		{
			name:     "known plan value untouched",
			stateRaw: nonNullRaw,
			planRaw:  nonNullRaw,
			stateVal: types.StringValue("A123"),
			planVal:  types.StringValue("A123"),
			wantPlan: types.StringValue("A123"),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			req := planmodifier.StringRequest{
				State:      tfsdk.State{Raw: tt.stateRaw},
				Plan:       tfsdk.Plan{Raw: tt.planRaw},
				StateValue: tt.stateVal,
				PlanValue:  tt.planVal,
			}
			resp := planmodifier.StringResponse{PlanValue: tt.planVal}
			createOnlyString().PlanModifyString(context.Background(), req, &resp)

			if !resp.PlanValue.Equal(tt.wantPlan) {
				t.Errorf("PlanValue = %v, want %v", resp.PlanValue, tt.wantPlan)
			}
		})
	}
}
