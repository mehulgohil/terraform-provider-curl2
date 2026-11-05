package curl2

import (
	"bytes"
	"context"
	"encoding/json"
	"github.com/hashicorp/go-retryablehttp"
	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"io"
)

var (
	_ datasource.DataSource              = &curl2DataSource{}
	_ datasource.DataSourceWithConfigure = &curl2DataSource{}
)

func NewCurl2DataSource() datasource.DataSource {
	return &curl2DataSource{}
}

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

type curl2DataSource struct {
	client *HttpClient
}

func (c *curl2DataSource) Metadata(ctx context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName
}

func (c *curl2DataSource) Schema(ctx context.Context, req datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "Fetches the response for the api",
		Attributes: map[string]schema.Attribute{
			"uri": schema.StringAttribute{
				Description: "URI of resource you'd like to retrieve via HTTP(s).",
				Required:    true,
			},
			"http_method": schema.StringAttribute{
				Description: "HTTP method like GET, POST, PUT, DELETE, PATCH.",
				Required:    true,
			},
			"json": schema.StringAttribute{
				Description: "JSON object in string format if using POST, PUT or PATCH method.",
				Optional:    true,
			},
			"response": schema.ObjectAttribute{
				AttributeTypes: map[string]attr.Type{
					"uri":         types.StringType,
					"body":        types.StringType,
					"status_code": types.Int64Type,
				},
				Description: "Valued returned by the HTTP request.",
				Computed:    true,
			},
			"auth_type": schema.StringAttribute{
				Description: "Authentication Type, Bearer or Basic.",
				Optional:    true,
			},
			"bearer_token": schema.StringAttribute{
				Description: "Bearer Token to be used for Authentication.",
				Optional:    true,
				Sensitive:   true,
			},
			"basic_auth_username": schema.StringAttribute{
				Description: "Username to be used for Basic Authentication.",
				Optional:    true,
			},
			"basic_auth_password": schema.StringAttribute{
				Description: "Password to be used for Authentication.",
				Optional:    true,
				Sensitive:   true,
			},
			"headers": schema.MapAttribute{
				Description: "Headers to be added.",
				ElementType: types.StringType,
				Optional:    true,
			},
			"ignore_errors": schema.BoolAttribute{
				Description: "If true, a request that fails (connection refused, DNS failure, timeout, or a 429/5xx response that is still failing after the configured retries) does not fail the plan. Instead `error` holds the reason, `response.status_code` is 0 and `response.body` is empty. Defaults to false.",
				Optional:    true,
			},
			"error": schema.StringAttribute{
				Description: "Why the request failed when `ignore_errors` is true. Null when the request succeeded.",
				Computed:    true,
			},
		},
	}
}

func (c *curl2DataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var config curl2DataModelRequest

	diags := req.Config.Get(ctx, &config)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	var body io.Reader = nil

	if config.JSON.ValueString() != "" {
		var jsonData interface{}
		if err := json.Unmarshal([]byte(config.JSON.ValueString()), &jsonData); err != nil {
			resp.Diagnostics.AddError(
				"Failed to parse JSON parameter",
				err.Error(),
			)
			return
		}

		requestBody, err := json.Marshal(jsonData)
		if err != nil {
			resp.Diagnostics.AddError(
				"Failed to marshal JSON data",
				err.Error(),
			)
			return
		}
		body = bytes.NewBuffer(requestBody)
	}

	newReq, err := retryablehttp.NewRequest(config.HTTPMethod.ValueString(), config.URI.ValueString(), body)
	if err != nil {
		resp.Diagnostics.AddError(
			"Unable to create new http request",
			err.Error(),
		)
		return
	}

	resp.Diagnostics.Append(setHeaders(ctx, newReq, config.Headers)...)
	if resp.Diagnostics.HasError() {
		return
	}

	if config.AuthType.ValueString() != "" {
		if config.AuthType.ValueString() == "Bearer" {
			if config.BearerToken.ValueString() == "" {
				resp.Diagnostics.AddError(
					"Invalid Bearer Token",
					"Bearer Token Parameter must be provided",
				)
				return
			}

			newReq.Header.Set("Authorization", "Bearer "+config.BearerToken.ValueString())
		}

		if config.AuthType.ValueString() == "Basic" {
			if config.BasicAuthUsername.ValueString() == "" || config.BasicAuthPassword.ValueString() == "" {
				resp.Diagnostics.AddError(
					"Invalid Basic Auth Token",
					"Basic Username and Password Parameters must be provided",
				)
				return
			}

			newReq.SetBasicAuth(config.BasicAuthUsername.ValueString(), config.BasicAuthPassword.ValueString())
		}
	}

	statusCode, responseData, summary, err := c.send(newReq)
	config.Error = types.StringNull()
	if err != nil {
		if !config.IgnoreErrors.ValueBool() {
			resp.Diagnostics.AddError(summary, err.Error())
			return
		}
		config.Error = types.StringValue(summary + ": " + err.Error())
	}

	config.Response, diags = types.ObjectValue(
		map[string]attr.Type{
			"uri":         types.StringType,
			"body":        types.StringType,
			"status_code": types.Int64Type,
		},
		map[string]attr.Value{
			"uri":         config.URI,
			"body":        types.StringValue(string(responseData)),
			"status_code": types.Int64Value(int64(statusCode)),
		},
	)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	diags = resp.State.Set(ctx, &config)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}
}

// send runs the request and returns the status code and body. On failure it
// returns a short summary and the error, with status 0 and an empty body.
func (c *curl2DataSource) send(req *retryablehttp.Request) (int, []byte, string, error) {
	r, err := c.client.httpClient.Do(req)
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

func (c *curl2DataSource) Configure(ctx context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
	if req.ProviderData == nil {
		return
	}
	c.client = req.ProviderData.(*HttpClient)
}
