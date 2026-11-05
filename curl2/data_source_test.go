package curl2

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/tfsdk"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-go/tftypes"
)

// readCurl2 runs the curl2 data source's Read with the given config values,
// the same way Terraform calls it.
func readCurl2(t *testing.T, values map[string]tftypes.Value) (curl2DataModelRequest, *datasource.ReadResponse) {
	t.Helper()
	ctx := context.Background()

	ds := &curl2DataSource{client: NewClient(ApiClientOpts{})}
	var schemaResp datasource.SchemaResponse
	ds.Schema(ctx, datasource.SchemaRequest{}, &schemaResp)
	objType := schemaResp.Schema.Type().TerraformType(ctx).(tftypes.Object)

	raw := map[string]tftypes.Value{}
	for name, attrType := range objType.AttributeTypes {
		raw[name] = tftypes.NewValue(attrType, nil)
	}
	for name, v := range values {
		raw[name] = v
	}

	req := datasource.ReadRequest{Config: tfsdk.Config{Schema: schemaResp.Schema, Raw: tftypes.NewValue(objType, raw)}}
	resp := &datasource.ReadResponse{State: tfsdk.State{Schema: schemaResp.Schema, Raw: tftypes.NewValue(objType, nil)}}
	ds.Read(ctx, req, resp)

	var state curl2DataModelRequest
	if !resp.Diagnostics.HasError() {
		if diags := resp.State.Get(ctx, &state); diags.HasError() {
			t.Fatalf("reading state: %v", diags)
		}
	}
	return state, resp
}

func str(s string) tftypes.Value { return tftypes.NewValue(tftypes.String, s) }

func closedServerURL() string {
	server := httptest.NewServer(http.NotFoundHandler())
	server.Close()
	return server.URL
}

func TestReadSendsHeadersAndReturnsResponse(t *testing.T) {
	var version string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		version = r.Header.Get("X-GitHub-Api-Version")
		w.WriteHeader(http.StatusTeapot)
		w.Write([]byte("hello"))
	}))
	defer server.Close()

	state, resp := readCurl2(t, map[string]tftypes.Value{
		"uri":         str(server.URL),
		"http_method": str("GET"),
		"headers": tftypes.NewValue(tftypes.Map{ElementType: tftypes.String}, map[string]tftypes.Value{
			"X-GitHub-Api-Version": str("2022-11-28"),
		}),
	})
	if resp.Diagnostics.HasError() {
		t.Fatalf("Read failed: %v", resp.Diagnostics)
	}
	if version != "2022-11-28" {
		t.Errorf("server got header %q, want 2022-11-28", version)
	}
	attrs := state.Response.Attributes()
	if attrs["status_code"].(types.Int64).ValueInt64() != http.StatusTeapot || attrs["body"].(types.String).ValueString() != "hello" {
		t.Errorf("response = %v", attrs)
	}
	if !state.Error.IsNull() {
		t.Errorf("error = %q, want null", state.Error.ValueString())
	}
}

func TestReadFailsByDefault(t *testing.T) {
	_, resp := readCurl2(t, map[string]tftypes.Value{
		"uri":         str(closedServerURL()),
		"http_method": str("GET"),
	})
	if !resp.Diagnostics.HasError() {
		t.Error("unreachable server did not fail the read")
	}
}

// Issue #15: ignore_errors keeps an unreachable endpoint from failing the plan.
func TestReadIgnoreErrors(t *testing.T) {
	state, resp := readCurl2(t, map[string]tftypes.Value{
		"uri":           str(closedServerURL()),
		"http_method":   str("GET"),
		"ignore_errors": tftypes.NewValue(tftypes.Bool, true),
	})
	if resp.Diagnostics.HasError() {
		t.Fatalf("ignore_errors = true still failed: %v", resp.Diagnostics)
	}
	if state.Error.IsNull() || state.Error.ValueString() == "" {
		t.Error("error is empty, want the reason")
	}
	attrs := state.Response.Attributes()
	if attrs["status_code"].(types.Int64).ValueInt64() != 0 || attrs["body"].(types.String).ValueString() != "" {
		t.Errorf("response = %v, want status 0 and empty body", attrs)
	}
}
