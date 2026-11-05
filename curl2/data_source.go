package curl2

import (
	"context"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

var (
	_ datasource.DataSource              = &curl2DataSource{}
	_ datasource.DataSourceWithConfigure = &curl2DataSource{}
)

func NewCurl2DataSource() datasource.DataSource {
	return &curl2DataSource{}
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
			"uri":                 schema.StringAttribute{Description: descURI, Required: true},
			"http_method":         schema.StringAttribute{Description: descHTTPMethod, Required: true},
			"json":                schema.StringAttribute{Description: descJSON, Optional: true},
			"response":            schema.ObjectAttribute{Description: descResponse, AttributeTypes: responseAttrTypes, Computed: true},
			"auth_type":           schema.StringAttribute{Description: descAuthType, Optional: true},
			"bearer_token":        schema.StringAttribute{Description: descBearerToken, Optional: true, Sensitive: true},
			"basic_auth_username": schema.StringAttribute{Description: descBasicAuthUsername, Optional: true},
			"basic_auth_password": schema.StringAttribute{Description: descBasicAuthPassword, Optional: true, Sensitive: true},
			"headers":             schema.MapAttribute{Description: descHeaders, ElementType: types.StringType, Optional: true},
			"ignore_errors":       schema.BoolAttribute{Description: descIgnoreErrors, Optional: true},
			"error":               schema.StringAttribute{Description: descError, Computed: true},
		},
	}
}

func (c *curl2DataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var config curl2DataModelRequest
	resp.Diagnostics.Append(req.Config.Get(ctx, &config)...)
	if resp.Diagnostics.HasError() {
		return
	}

	resp.Diagnostics.Append(runCurl2Request(ctx, c.client, &config)...)
	if resp.Diagnostics.HasError() {
		return
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &config)...)
}

func (c *curl2DataSource) Configure(ctx context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
	if req.ProviderData == nil {
		return
	}
	c.client = req.ProviderData.(*HttpClient)
}
