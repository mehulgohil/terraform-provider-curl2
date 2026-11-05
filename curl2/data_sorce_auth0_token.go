package curl2

import (
	"context"

	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

var (
	_ datasource.DataSource              = &auth0TokenDataSource{}
	_ datasource.DataSourceWithConfigure = &auth0TokenDataSource{}
)

func NewAuth0TokenDataSource() datasource.DataSource {
	return &auth0TokenDataSource{}
}

type auth0TokenDataModelRequest struct {
	Audience types.String `tfsdk:"audience"`
	Response types.Object `tfsdk:"response"`
}

type auth0TokenDataSource struct {
	client *HttpClient
}

func (a *auth0TokenDataSource) Configure(ctx context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
	if req.ProviderData == nil {
		return
	}
	a.client = req.ProviderData.(*HttpClient)
}

func (a *auth0TokenDataSource) Metadata(ctx context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_auth0_token"
}

func (a *auth0TokenDataSource) Schema(ctx context.Context, req datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "Fetches Auth0 Token",
		Attributes: map[string]schema.Attribute{
			"audience": schema.StringAttribute{
				Description: "The audience for the token, which is your API. Example: \"https://xyz.com\"",
				Required:    true,
			},
			"response": schema.ObjectAttribute{
				AttributeTypes: map[string]attr.Type{
					"token": types.StringType,
				},
				Description: "Token response.",
				Computed:    true,
			},
		},
	}
}

func (a *auth0TokenDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var config auth0TokenDataModelRequest
	resp.Diagnostics.Append(req.Config.Get(ctx, &config)...)
	if resp.Diagnostics.HasError() {
		return
	}

	token, diags := fetchAuth0Token(ctx, a.client, config.Audience.ValueString())
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	config.Response, diags = types.ObjectValue(
		map[string]attr.Type{"token": types.StringType},
		map[string]attr.Value{"token": types.StringValue(token.AccessToken)},
	)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &config)...)
}
