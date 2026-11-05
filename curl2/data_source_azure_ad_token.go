package curl2

import (
	"context"

	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

var (
	_ datasource.DataSource              = &azureADTokenDataSource{}
	_ datasource.DataSourceWithConfigure = &azureADTokenDataSource{}
)

func NewAzureADTokenDataSource() datasource.DataSource {
	return &azureADTokenDataSource{}
}

type azureADTokenDataModelRequest struct {
	Scopes   types.List   `tfsdk:"scopes"`
	Response types.Object `tfsdk:"response"`
}

type azureADTokenDataSource struct{}

func (a *azureADTokenDataSource) Configure(ctx context.Context, req datasource.ConfigureRequest, response *datasource.ConfigureResponse) {
	if req.ProviderData == nil {
		return
	}
}

func (a *azureADTokenDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var config azureADTokenDataModelRequest
	resp.Diagnostics.Append(req.Config.Get(ctx, &config)...)
	if resp.Diagnostics.HasError() {
		return
	}

	var scopes []string
	resp.Diagnostics.Append(config.Scopes.ElementsAs(ctx, &scopes, false)...)
	if resp.Diagnostics.HasError() {
		return
	}

	token, expiresOn, diags := fetchAzureADToken(ctx, scopes)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	config.Response, diags = types.ObjectValue(
		map[string]attr.Type{"token": types.StringType, "expires_on": types.StringType},
		map[string]attr.Value{"token": types.StringValue(token), "expires_on": types.StringValue(expiresOn.String())},
	)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &config)...)
}

func (a *azureADTokenDataSource) Metadata(ctx context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_azuread_token"
}

func (a *azureADTokenDataSource) Schema(ctx context.Context, req datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "Fetches Azure AD Token",
		Attributes: map[string]schema.Attribute{
			"scopes": schema.ListAttribute{
				ElementType: types.StringType,
				Description: "The value passed for the scope parameter in this request should be the resource identifier (application ID URI) of the resource you want, affixed with the .default suffix. Example: [\"https://graph.microsoft.com/.default\"]",
				Required:    true,
			},
			"response": schema.ObjectAttribute{
				AttributeTypes: map[string]attr.Type{
					"token":      types.StringType,
					"expires_on": types.StringType,
				},
				Description: "Token response.",
				Computed:    true,
			},
		},
	}
}
