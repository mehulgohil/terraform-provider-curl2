package curl2

import (
	"context"
	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/provider"
	"github.com/hashicorp/terraform-plugin-framework/provider/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-framework/types/basetypes"
	"github.com/hashicorp/terraform-plugin-log/tflog"
	"os"
)

var (
	_ provider.Provider = &curl2Provider{}
)

func NewProvider() provider.Provider {
	return &curl2Provider{}
}

type curl2Provider struct{}

// curl2ProviderModel maps provider schema data to a Go type.
type curl2ProviderModel struct {
	DisableTLS types.Bool   `tfsdk:"disable_tls"`
	TimeoutMS  types.Int64  `tfsdk:"timeout_ms"`
	Retry      types.Object `tfsdk:"retry"`
	AzureAD    types.Object `tfsdk:"azure_ad"`
	Auth0      types.Object `tfsdk:"auth0"`
}

type retryModel struct {
	RetryAttempts types.Int64 `tfsdk:"retry_attempts"`
	MinDelay      types.Int64 `tfsdk:"min_delay_ms"`
	MaxDelay      types.Int64 `tfsdk:"max_delay_ms"`
}

type azureADModel struct {
	ClientID     types.String `tfsdk:"client_id"`
	ClientSecret types.String `tfsdk:"client_secret"`
	TenantID     types.String `tfsdk:"tenant_id"`
}

type auth0Model struct {
	ClientID     types.String `tfsdk:"client_id"`
	ClientSecret types.String `tfsdk:"client_secret"`
	Domain       types.String `tfsdk:"domain"`
}

func (c *curl2Provider) Metadata(_ context.Context, _ provider.MetadataRequest, resp *provider.MetadataResponse) {
	resp.TypeName = "curl2"
}

// Schema defines the provider-level schema for configuration data.
func (c *curl2Provider) Schema(_ context.Context, _ provider.SchemaRequest, resp *provider.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "Triggers HTTP(s) requests along with JSON body, authentication as well as custom headers. It also supports token generation from IDP like Azure AD, Auth0.",
		Attributes: map[string]schema.Attribute{
			"disable_tls": schema.BoolAttribute{
				Optional:    true,
				Description: "Use to disable the TLS verification. Defaults to false.",
			},
			"timeout_ms": schema.Int64Attribute{
				Optional:    true,
				Description: "Request Timeout in milliseconds. Defaults to 0, no timeout",
			},
		},
		Blocks: map[string]schema.Block{
			"retry": schema.SingleNestedBlock{
				Description: "Retry request configuration. By default there are no retries.",
				Attributes: map[string]schema.Attribute{
					"retry_attempts": schema.Int64Attribute{
						Description: "The number of times the request is to be retried. For example, if 2 is specified, the request will be tried a maximum of 3 times.",
						Optional:    true,
					},
					"min_delay_ms": schema.Int64Attribute{
						Description: "The minimum delay between retry requests in milliseconds.",
						Optional:    true,
					},
					"max_delay_ms": schema.Int64Attribute{
						Description: "The maximum delay between retry requests in milliseconds.",
						Optional:    true,
					},
				},
			},
			"azure_ad": schema.SingleNestedBlock{
				Description: "Azure AD Configuration which is required if you are using `curl2_azuread_token` data",
				Attributes: map[string]schema.Attribute{
					"client_id": schema.StringAttribute{
						Description: "Application ID of an Azure service principal. You can also set it as ENV variable `AZURE_CLIENT_ID`",
						Optional:    true,
					},
					"client_secret": schema.StringAttribute{
						Description: "Password of the Azure service principal. You can also set it as ENV variable `AZURE_CLIENT_SECRET`",
						Optional:    true,
					},
					"tenant_id": schema.StringAttribute{
						Description: "ID of the application's Azure AD tenant. You can also set it as ENV variable `AZURE_TENANT_ID`",
						Optional:    true,
					},
				},
			},
			"auth0": schema.SingleNestedBlock{
				Description: "Auth0 Configuration which is required if you are using `curl2_auth0_token` data",
				Attributes: map[string]schema.Attribute{
					"client_id": schema.StringAttribute{
						Description: "Application's Client ID. You can also set it as ENV variable `AUTH0_CLIENT_ID`",
						Optional:    true,
					},
					"client_secret": schema.StringAttribute{
						Description: "Application's Client Secret. You can also set it as ENV variable `AUTH0_CLIENT_SECRET`",
						Optional:    true,
					},
					"domain": schema.StringAttribute{
						Description: "Auth0 domain URL in the format `https://<your-tenant-name>.auth0.com`. You can also set it as ENV variable `AUTH0_DOMAIN`",
						Optional:    true,
					},
				},
			},
		},
	}
}

func (c *curl2Provider) Configure(ctx context.Context, req provider.ConfigureRequest, resp *provider.ConfigureResponse) {
	tflog.Info(ctx, "Configuring curl2 client", map[string]any{"success": false})
	// Retrieve provider data from configuration
	var config curl2ProviderModel
	diags := req.Config.Get(ctx, &config)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	var azureAD azureADModel
	if !config.AzureAD.IsNull() && !config.AzureAD.IsUnknown() {
		diags = config.AzureAD.As(ctx, &azureAD, basetypes.ObjectAsOptions{})
		resp.Diagnostics.Append(diags...)
		if resp.Diagnostics.HasError() {
			return
		}

		err := os.Setenv("AZURE_CLIENT_ID", azureAD.ClientID.ValueString())
		if err != nil {
			resp.Diagnostics.AddError(
				"Unable to set AZURE_CLIENT_ID Env Variables",
				err.Error(),
			)
			return
		}
		err = os.Setenv("AZURE_CLIENT_SECRET", azureAD.ClientSecret.ValueString())
		if err != nil {
			resp.Diagnostics.AddError(
				"Unable to set AZURE_CLIENT_SECRET Env Variables",
				err.Error(),
			)
			return
		}
		err = os.Setenv("AZURE_TENANT_ID", azureAD.TenantID.ValueString())
		if err != nil {
			resp.Diagnostics.AddError(
				"Unable to set AZURE_TENANT_ID Env Variables",
				err.Error(),
			)
			return
		}
	}

	var auth0Config auth0Model
	if !config.Auth0.IsNull() && !config.Auth0.IsUnknown() {
		diags = config.Auth0.As(ctx, &auth0Config, basetypes.ObjectAsOptions{})
		resp.Diagnostics.Append(diags...)
		if resp.Diagnostics.HasError() {
			return
		}

		err := os.Setenv("AUTH0_CLIENT_ID", auth0Config.ClientID.ValueString())
		if err != nil {
			resp.Diagnostics.AddError(
				"Unable to set AUTH0_CLIENT_ID Env Variables",
				err.Error(),
			)
			return
		}
		err = os.Setenv("AUTH0_CLIENT_SECRET", auth0Config.ClientSecret.ValueString())
		if err != nil {
			resp.Diagnostics.AddError(
				"Unable to set AUTH0_CLIENT_SECRET Env Variables",
				err.Error(),
			)
			return
		}
		err = os.Setenv("AUTH0_DOMAIN", auth0Config.Domain.ValueString())
		if err != nil {
			resp.Diagnostics.AddError(
				"Unable to set AUTH0_DOMAIN Env Variables",
				err.Error(),
			)
			return
		}
	}

	var retry retryModel
	if !config.Retry.IsNull() && !config.Retry.IsUnknown() {
		diags = config.Retry.As(ctx, &retry, basetypes.ObjectAsOptions{})
		resp.Diagnostics.Append(diags...)
		if resp.Diagnostics.HasError() {
			return
		}
	}

	opts := ApiClientOpts{
		insecure: config.DisableTLS.ValueBool(),
		timeout:  config.TimeoutMS.ValueInt64(),
	}
	client := NewClient(opts)

	resp.DataSourceData = client
	resp.ResourceData = client

	tflog.Info(ctx, "Configured curl2 client", map[string]any{"success": true})
}

func (c *curl2Provider) DataSources(ctx context.Context) []func() datasource.DataSource {
	return []func() datasource.DataSource{
		NewCurl2DataSource,
		NewAzureADTokenDataSource,
		NewAuth0TokenDataSource,
	}
}

func (c *curl2Provider) Resources(ctx context.Context) []func() resource.Resource {
	return []func() resource.Resource{}
}
