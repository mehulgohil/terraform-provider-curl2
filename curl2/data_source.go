package curl2

import (
	"bytes"
	"context"
	"encoding/json"
	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"io"
	"net/http"
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
				ElementType: types.StringType,
				Optional:    true,
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

	uri := config.URI
	httpMethod := config.HTTPMethod
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

	newReq, err := http.NewRequest(httpMethod.ValueString(), uri.ValueString(), body)
	if err != nil {
		resp.Diagnostics.AddError(
			"Unable to create new http request",
			err.Error(),
		)
		return
	}

	for eachHeaderKey, eachHeaderValue := range config.Headers.Elements() {
		newReq.Header.Set(eachHeaderKey, eachHeaderValue.String())
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

	r, err := (*c.client).httpClient.Do(newReq)
	if err != nil {
		resp.Diagnostics.AddError(
			"Error calling api",
			err.Error(),
		)
		return
	}
	defer r.Body.Close()

	responseData, err := io.ReadAll(r.Body)
	if err != nil {
		resp.Diagnostics.AddError(
			"Error reading response body",
			err.Error(),
		)
		return
	}
	config.Response, diags = types.ObjectValue(
		map[string]attr.Type{
			"body":        types.StringType,
			"status_code": types.Int64Type,
		},
		map[string]attr.Value{
			"body":        types.StringValue(string(responseData)),
			"status_code": types.Int64Value(int64(r.StatusCode)),
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

func (c *curl2DataSource) Configure(ctx context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
	if req.ProviderData == nil {
		return
	}
	c.client = req.ProviderData.(*HttpClient)
}
