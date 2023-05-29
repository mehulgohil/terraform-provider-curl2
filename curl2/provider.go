package curl2

import (
	"context"
	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/provider"
	"github.com/hashicorp/terraform-plugin-framework/provider/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-log/tflog"
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
	DisableTLS types.Bool  `tfsdk:"disable_tls"`
	TimeoutMS  types.Int64 `tfsdk:"timeout_ms"`
}

func (c *curl2Provider) Metadata(_ context.Context, _ provider.MetadataRequest, resp *provider.MetadataResponse) {
	resp.TypeName = "curl2"
}

// Schema defines the provider-level schema for configuration data.
func (c *curl2Provider) Schema(_ context.Context, _ provider.SchemaRequest, resp *provider.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "Triggers HTTP(s) requests along with JSON body, authentication as well as custom headers",
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

	opts := ApiClientOpts{
		insecure: config.DisableTLS.ValueBool(),
		timeout:  config.TimeoutMS.ValueInt64(),
	}
	client, err := NewClient(opts)
	if err != nil {
		resp.Diagnostics.AddError(
			"Unable to Create curl2 client",
			"Client Error: "+err.Error(),
		)
		return
	}

	resp.DataSourceData = client
	resp.ResourceData = client

	tflog.Info(ctx, "Configured curl2 client", map[string]any{"success": true})
}

func (c *curl2Provider) DataSources(ctx context.Context) []func() datasource.DataSource {
	return []func() datasource.DataSource{
		NewCurl2DataSource,
	}
}

func (c *curl2Provider) Resources(ctx context.Context) []func() resource.Resource {
	return []func() resource.Resource{}
}
