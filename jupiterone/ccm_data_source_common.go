package jupiterone

import (
	"context"

	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

// stringSlice converts an optional list attribute into a Go slice, returning
// nil when the attribute is unset so that the corresponding filter is omitted
// from the request rather than sent as an empty list.
func stringSlice(ctx context.Context, list types.List) ([]string, diag.Diagnostics) {
	var diags diag.Diagnostics

	if list.IsNull() || list.IsUnknown() {
		return nil, diags
	}

	var values []string
	diags.Append(list.ElementsAs(ctx, &values, false)...)

	return values, diags
}

// optionalBool converts an unset bool attribute into a nil pointer. A plain
// false is a meaningful filter value, so it must be distinguishable from
// "not configured".
func optionalBool(v types.Bool) *bool {
	if v.IsNull() || v.IsUnknown() {
		return nil
	}

	b := v.ValueBool()
	return &b
}
