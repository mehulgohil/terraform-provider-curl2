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
}

type retryModel struct {
	RetryAttempts types.Int64 `tfsdk:"retry_attempts"`
	MinDelay      types.Int64 `tfsdk:"min_delay_ms"`
	MaxDelay      types.Int64 `tfsdk:"max_delay_ms"`
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
	}
}

func (c *curl2Provider) Resources(ctx context.Context) []func() resource.Resource {
	return []func() resource.Resource{}
}
