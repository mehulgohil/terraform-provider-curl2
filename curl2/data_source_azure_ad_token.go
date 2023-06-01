package curl2

import (
	"context"
	"github.com/Azure/azure-sdk-for-go/sdk/azcore/policy"
	"github.com/Azure/azure-sdk-for-go/sdk/azidentity"
	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"os"
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

	diags := req.Config.Get(ctx, &config)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	clientID := os.Getenv("AZURE_CLIENT_ID")
	clientSecret := os.Getenv("AZURE_CLIENT_SECRET")
	tenantID := os.Getenv("AZURE_TENANT_ID")

	if clientID == "" {
		resp.Diagnostics.AddAttributeError(
			path.Root("client_id"),
			"Missing Client ID",
			"The data source cannot get the token as client id is missing in provider azure_ad block",
		)
	}

	if clientSecret == "" {
		resp.Diagnostics.AddAttributeError(
			path.Root("client_secret"),
			"Missing Client Secret",
			"The data source cannot get the token as client secret is missing in provider azure_ad block",
		)
	}

	if tenantID == "" {
		resp.Diagnostics.AddAttributeError(
			path.Root("tenant_id"),
			"Missing Tenant ID",
			"The data source cannot get the token as tenant id is missing in provider azure_ad block",
		)
	}

	if len(config.Scopes.Elements()) == 0 {
		resp.Diagnostics.AddAttributeError(
			path.Root("scopes"),
			"Missing Scopes",
			"The data source cannot get the token as scopes are missing",
		)
	}

	if resp.Diagnostics.HasError() {
		return
	}

	cred, err := azidentity.NewDefaultAzureCredential(nil)
	if err != nil {
		resp.Diagnostics.AddError(
			"Error getting default azure credentials",
			err.Error(),
		)
		return
	}
	var scopeArr []string
	config.Scopes.ElementsAs(ctx, &scopeArr, false)

	token, err := cred.GetToken(context.Background(), policy.TokenRequestOptions{
		TenantID: tenantID,
		Scopes:   scopeArr,
	})
	if err != nil {
		resp.Diagnostics.AddError(
			"Error getting azure ad token",
			err.Error(),
		)
		return
	}

	config.Response, diags = types.ObjectValue(
		map[string]attr.Type{
			"token":      types.StringType,
			"expires_on": types.StringType,
		},
		map[string]attr.Value{
			"token":      types.StringValue(token.Token),
			"expires_on": types.StringValue(token.ExpiresOn.String()),
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
