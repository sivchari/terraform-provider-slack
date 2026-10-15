package internal

import (
	"context"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

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
