package internal

import (
	"context"
	"strings"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/datasource"
	dsschema "github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/tfsdk"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-go/tftypes"
)

func TestUniqueStrings_ValidateList(t *testing.T) {
	t.Parallel()

	list := func(values ...attr.Value) types.List {
		return types.ListValueMust(types.StringType, values)
	}
	str := func(s string) attr.Value { return types.StringValue(s) }

	tests := []struct {
		name      string
		value     types.List
		wantErr   bool
		wantInMsg string
	}{
		{name: "distinct elements", value: list(str("U1"), str("U2"))},
		{name: "empty list", value: list()},
		{name: "null list is skipped", value: types.ListNull(types.StringType)},
		{name: "unknown list is skipped", value: types.ListUnknown(types.StringType)},
		{name: "unknown elements are skipped", value: list(types.StringUnknown(), types.StringUnknown())},
		{name: "null elements are skipped", value: list(types.StringNull(), types.StringNull())},
		{name: "repeated element", value: list(str("U1"), str("U2"), str("U1")), wantErr: true, wantInMsg: `"U1"`},
		{name: "adjacent repeated element", value: list(str("U3"), str("U3")), wantErr: true, wantInMsg: `"U3"`},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			req := validator.ListRequest{Path: path.Root("members"), ConfigValue: tt.value}
			var resp validator.ListResponse
			uniqueStrings().ValidateList(context.Background(), req, &resp)

			if got := resp.Diagnostics.HasError(); got != tt.wantErr {
				t.Errorf("HasError() = %v, want %v (diagnostics: %v)", got, tt.wantErr, resp.Diagnostics)
				return
			}
			if tt.wantInMsg != "" && !strings.Contains(resp.Diagnostics.Errors()[0].Detail(), tt.wantInMsg) {
				t.Errorf("detail = %q, want it to contain %q", resp.Diagnostics.Errors()[0].Detail(), tt.wantInMsg)
			}
		})
	}
}

func TestOneOfString_ValidateString(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		value   types.String
		wantErr bool
	}{
		{name: "allowed value", value: types.StringValue("archive")},
		{name: "another allowed value", value: types.StringValue("none")},
		{name: "unknown value is skipped", value: types.StringUnknown()},
		{name: "null value is skipped", value: types.StringNull()},
		{name: "other value is rejected", value: types.StringValue("delete"), wantErr: true},
		{name: "empty value is rejected", value: types.StringValue(""), wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			req := validator.StringRequest{Path: path.Root("action_on_destroy"), ConfigValue: tt.value}
			var resp validator.StringResponse
			oneOfString("archive", "none").ValidateString(context.Background(), req, &resp)

			if got := resp.Diagnostics.HasError(); got != tt.wantErr {
				t.Errorf("HasError() = %v, want %v (diagnostics: %v)", got, tt.wantErr, resp.Diagnostics)
			}
		})
	}
}

func TestExactlyOneOf_ValidateDataSource(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		config  map[string]any
		wantErr bool
	}{
		{name: "only the first is set", config: map[string]any{"id": "U1"}},
		{name: "only the last is set", config: map[string]any{"handle": "admins"}},
		{name: "none is set", config: map[string]any{}, wantErr: true},
		{name: "two are set", config: map[string]any{"id": "U1", "name": "ops"}, wantErr: true},
		{name: "all are set", config: map[string]any{"id": "U1", "name": "ops", "handle": "admins"}, wantErr: true},
		{name: "empty string counts as set", config: map[string]any{"id": "", "name": "ops"}, wantErr: true},
		{name: "unknown value is skipped", config: map[string]any{"id": tftypes.UnknownValue}},
		{name: "unknown value next to a set one is skipped", config: map[string]any{"id": tftypes.UnknownValue, "name": "ops"}},
	}

	attrs := map[string]dsschema.Attribute{}
	attrTypes := map[string]tftypes.Type{}
	for _, name := range []string{"id", "name", "handle"} {
		attrs[name] = dsschema.StringAttribute{Optional: true, Computed: true}
		attrTypes[name] = tftypes.String
	}
	objType := tftypes.Object{AttributeTypes: attrTypes}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			values := make(map[string]tftypes.Value, len(attrTypes))
			for name := range attrTypes {
				values[name] = tftypes.NewValue(tftypes.String, tt.config[name])
			}
			req := datasource.ValidateConfigRequest{
				Config: tfsdk.Config{Raw: tftypes.NewValue(objType, values), Schema: dsschema.Schema{Attributes: attrs}},
			}
			var resp datasource.ValidateConfigResponse
			exactlyOneOf("id", "name", "handle").ValidateDataSource(context.Background(), req, &resp)

			if got := resp.Diagnostics.HasError(); got != tt.wantErr {
				t.Errorf("HasError() = %v, want %v (diagnostics: %v)", got, tt.wantErr, resp.Diagnostics)
			}
		})
	}
}
