package curl2

import (
	"bytes"
	"context"
	"encoding/json"
	"io"

	"github.com/hashicorp/go-retryablehttp"
	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

// curl2DataModelRequest is the model shared by the curl2 data source and the
// curl2 ephemeral resource, which have the same attributes.
type curl2DataModelRequest struct {
	URI               types.String `tfsdk:"uri"`
	HTTPMethod        types.String `tfsdk:"http_method"`
	JSON              types.String `tfsdk:"json"`
	Response          types.Object `tfsdk:"response"`
	AuthType          types.String `tfsdk:"auth_type"`
	BearerToken       types.String `tfsdk:"bearer_token"`
	BasicAuthUsername types.String `tfsdk:"basic_auth_username"`
	BasicAuthPassword types.String `tfsdk:"basic_auth_password"`
	Headers           types.Map    `tfsdk:"headers"`
	IgnoreErrors      types.Bool   `tfsdk:"ignore_errors"`
	Error             types.String `tfsdk:"error"`
}

// Attribute descriptions shared by the data source and ephemeral schemas.
const (
	descURI               = "URI of resource you'd like to retrieve via HTTP(s)."
	descHTTPMethod        = "HTTP method like GET, POST, PUT, DELETE, PATCH."
	descJSON              = "JSON object in string format if using POST, PUT or PATCH method."
	descResponse          = "Valued returned by the HTTP request."
	descAuthType          = "Authentication Type, Bearer or Basic."
	descBearerToken       = "Bearer Token to be used for Authentication."
	descBasicAuthUsername = "Username to be used for Basic Authentication."
	descBasicAuthPassword = "Password to be used for Authentication."
	descHeaders           = "Headers to be added."
	descIgnoreErrors      = "If true, a request that fails (connection refused, DNS failure, timeout, or a 429/5xx response that is still failing after the configured retries) does not fail the plan. Instead `error` holds the reason, `response.status_code` is 0 and `response.body` is empty. Defaults to false."
	descError             = "Why the request failed when `ignore_errors` is true. Null when the request succeeded."
)

var responseAttrTypes = map[string]attr.Type{
	"uri":         types.StringType,
	"body":        types.StringType,
	"status_code": types.Int64Type,
}

// runCurl2Request sends the request the model describes and fills in
// m.Response and m.Error.
func runCurl2Request(ctx context.Context, client *HttpClient, m *curl2DataModelRequest) diag.Diagnostics {
	var diags diag.Diagnostics

	var body io.Reader
	if m.JSON.ValueString() != "" {
		var jsonData any
		if err := json.Unmarshal([]byte(m.JSON.ValueString()), &jsonData); err != nil {
			diags.AddError("Failed to parse JSON parameter", err.Error())
			return diags
		}
		requestBody, err := json.Marshal(jsonData)
		if err != nil {
			diags.AddError("Failed to marshal JSON data", err.Error())
			return diags
		}
		body = bytes.NewBuffer(requestBody)
	}

	req, err := retryablehttp.NewRequestWithContext(ctx, m.HTTPMethod.ValueString(), m.URI.ValueString(), body)
	if err != nil {
		diags.AddError("Unable to create new http request", err.Error())
		return diags
	}

	diags.Append(setHeaders(ctx, req, m.Headers)...)
	if diags.HasError() {
		return diags
	}

	switch m.AuthType.ValueString() {
	case "Bearer":
		if m.BearerToken.ValueString() == "" {
			diags.AddError("Invalid Bearer Token", "Bearer Token Parameter must be provided")
			return diags
		}
		req.Header.Set("Authorization", "Bearer "+m.BearerToken.ValueString())
	case "Basic":
		if m.BasicAuthUsername.ValueString() == "" || m.BasicAuthPassword.ValueString() == "" {
			diags.AddError("Invalid Basic Auth Token", "Basic Username and Password Parameters must be provided")
			return diags
		}
		req.SetBasicAuth(m.BasicAuthUsername.ValueString(), m.BasicAuthPassword.ValueString())
	}

	statusCode, responseData, summary, err := send(client, req)
	m.Error = types.StringNull()
	if err != nil {
		if !m.IgnoreErrors.ValueBool() {
			diags.AddError(summary, err.Error())
			return diags
		}
		m.Error = types.StringValue(summary + ": " + err.Error())
	}

	var d diag.Diagnostics
	m.Response, d = types.ObjectValue(responseAttrTypes, map[string]attr.Value{
		"uri":         m.URI,
		"body":        types.StringValue(string(responseData)),
		"status_code": types.Int64Value(int64(statusCode)),
	})
	diags.Append(d...)
	return diags
}

// send runs the request and returns the status code and body. On failure it
// returns a short summary and the error, with status 0 and an empty body.
func send(client *HttpClient, req *retryablehttp.Request) (int, []byte, string, error) {
	r, err := client.httpClient.Do(req)
	if err != nil {
		return 0, nil, "Error calling api", err
	}
	defer r.Body.Close()

	responseData, err := io.ReadAll(r.Body)
	if err != nil {
		return 0, nil, "Error reading response body", err
	}
	return r.StatusCode, responseData, "", nil
}
