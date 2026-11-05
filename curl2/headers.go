package curl2

import (
	"context"

	"github.com/hashicorp/go-retryablehttp"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

// setHeaders copies the configured headers onto the request.
//
// It reads the raw string values. attr.Value.String() must not be used here:
// it returns the Terraform debug form, which wraps strings in quotes
// (e.g. `"2022-11-28"`), and those quotes were sent to the server (issue #13).
func setHeaders(ctx context.Context, req *retryablehttp.Request, headers types.Map) diag.Diagnostics {
	if headers.IsNull() || headers.IsUnknown() {
		return nil
	}

	var values map[string]string
	diags := headers.ElementsAs(ctx, &values, false)
	if diags.HasError() {
		return diags
	}

	for key, value := range values {
		req.Header.Set(key, value)
	}
	return diags
}
