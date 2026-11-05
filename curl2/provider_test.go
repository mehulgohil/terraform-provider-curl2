package curl2

import (
	"context"
	"net/http"
	"testing"
	"time"

	"github.com/hashicorp/terraform-plugin-framework/provider"
	"github.com/hashicorp/terraform-plugin-framework/tfsdk"
	"github.com/hashicorp/terraform-plugin-go/tftypes"
)

// The provider block's disable_tls, timeout_ms and retry settings must reach
// the client the data sources use. Before the fix the retry block was read
// and then dropped.
func TestProviderConfigureAppliesSettings(t *testing.T) {
	ctx := context.Background()
	p := &curl2Provider{}

	var schemaResp provider.SchemaResponse
	p.Schema(ctx, provider.SchemaRequest{}, &schemaResp)
	objType := schemaResp.Schema.Type().TerraformType(ctx).(tftypes.Object)

	raw := map[string]tftypes.Value{}
	for name, attrType := range objType.AttributeTypes {
		raw[name] = tftypes.NewValue(attrType, nil)
	}
	raw["disable_tls"] = tftypes.NewValue(tftypes.Bool, true)
	raw["timeout_ms"] = tftypes.NewValue(tftypes.Number, 1500)
	retryType := objType.AttributeTypes["retry"].(tftypes.Object)
	raw["retry"] = tftypes.NewValue(retryType, map[string]tftypes.Value{
		"retry_attempts": tftypes.NewValue(tftypes.Number, 3),
		"min_delay_ms":   tftypes.NewValue(tftypes.Number, 10),
		"max_delay_ms":   tftypes.NewValue(tftypes.Number, 20),
	})

	req := provider.ConfigureRequest{Config: tfsdk.Config{Schema: schemaResp.Schema, Raw: tftypes.NewValue(objType, raw)}}
	var resp provider.ConfigureResponse
	p.Configure(ctx, req, &resp)
	if resp.Diagnostics.HasError() {
		t.Fatalf("Configure failed: %v", resp.Diagnostics)
	}

	c := resp.DataSourceData.(*HttpClient).httpClient
	if c.RetryMax != 3 || c.RetryWaitMin != 10*time.Millisecond || c.RetryWaitMax != 20*time.Millisecond {
		t.Errorf("retry = %d, %v, %v; want 3, 10ms, 20ms", c.RetryMax, c.RetryWaitMin, c.RetryWaitMax)
	}
	if c.HTTPClient.Timeout != 1500*time.Millisecond {
		t.Errorf("timeout = %v, want 1.5s", c.HTTPClient.Timeout)
	}
	tlsConfig := c.HTTPClient.Transport.(*http.Transport).TLSClientConfig
	if tlsConfig == nil || !tlsConfig.InsecureSkipVerify {
		t.Errorf("TLS config = %+v, want InsecureSkipVerify", tlsConfig)
	}
}
