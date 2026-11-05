package curl2

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/hashicorp/terraform-plugin-framework/ephemeral"
	"github.com/hashicorp/terraform-plugin-framework/tfsdk"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-go/tftypes"
)

// openEphemeral runs an ephemeral resource's Open with the given config
// values, the same way Terraform calls it.
func openEphemeral(t *testing.T, e ephemeral.EphemeralResourceWithConfigure, values map[string]tftypes.Value) *ephemeral.OpenResponse {
	t.Helper()
	ctx := context.Background()
	e.Configure(ctx, ephemeral.ConfigureRequest{ProviderData: NewClient(ApiClientOpts{})}, &ephemeral.ConfigureResponse{})

	var schemaResp ephemeral.SchemaResponse
	e.Schema(ctx, ephemeral.SchemaRequest{}, &schemaResp)
	objType := schemaResp.Schema.Type().TerraformType(ctx).(tftypes.Object)

	raw := map[string]tftypes.Value{}
	for name, attrType := range objType.AttributeTypes {
		raw[name] = tftypes.NewValue(attrType, nil)
	}
	for name, v := range values {
		raw[name] = v
	}

	req := ephemeral.OpenRequest{Config: tfsdk.Config{Schema: schemaResp.Schema, Raw: tftypes.NewValue(objType, raw)}}
	resp := &ephemeral.OpenResponse{Result: tfsdk.EphemeralResultData{Schema: schemaResp.Schema, Raw: tftypes.NewValue(objType, nil)}}
	e.Open(ctx, req, resp)
	return resp
}

// fakeAuth0 serves /oauth/token like Auth0's client-credentials endpoint.
func fakeAuth0(t *testing.T, status int) *httptest.Server {
	t.Helper()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var body tokenRequestBody
		json.NewDecoder(r.Body).Decode(&body)
		if r.URL.Path != "/oauth/token" || body.GrantType != "client_credentials" || body.ClientID != "id" || body.Audience != "https://api.example.com" {
			t.Errorf("unexpected token request: %s %+v", r.URL.Path, body)
		}
		w.WriteHeader(status)
		if status == http.StatusOK {
			json.NewEncoder(w).Encode(map[string]any{"access_token": "secret-token", "token_type": "Bearer", "expires_in": 3600})
		} else {
			w.Write([]byte(`{"error":"access_denied"}`))
		}
	}))
	t.Cleanup(server.Close)
	t.Setenv("AUTH0_CLIENT_ID", "id")
	t.Setenv("AUTH0_CLIENT_SECRET", "secret")
	t.Setenv("AUTH0_DOMAIN", server.URL)
	return server
}

func TestEphemeralAuth0Token(t *testing.T) {
	fakeAuth0(t, http.StatusOK)

	resp := openEphemeral(t, &auth0TokenEphemeral{}, map[string]tftypes.Value{"audience": str("https://api.example.com")})
	if resp.Diagnostics.HasError() {
		t.Fatalf("Open failed: %v", resp.Diagnostics)
	}
	var m auth0TokenEphemeralModel
	resp.Result.Get(context.Background(), &m)
	if m.Token.ValueString() != "secret-token" {
		t.Errorf("token = %q", m.Token.ValueString())
	}
	expires, err := time.Parse(time.RFC3339, m.ExpiresAt.ValueString())
	if err != nil || time.Until(expires) < 59*time.Minute || time.Until(expires) > 61*time.Minute {
		t.Errorf("expires_at = %q, want about an hour from now", m.ExpiresAt.ValueString())
	}
}

func TestEphemeralAuth0TokenRejected(t *testing.T) {
	fakeAuth0(t, http.StatusUnauthorized)

	resp := openEphemeral(t, &auth0TokenEphemeral{}, map[string]tftypes.Value{"audience": str("https://api.example.com")})
	if !resp.Diagnostics.HasError() {
		t.Error("a 401 from Auth0 did not fail; it used to return an empty token")
	}
}

func TestEphemeralAzureADTokenNeedsCredentials(t *testing.T) {
	t.Setenv("AZURE_CLIENT_ID", "")
	t.Setenv("AZURE_CLIENT_SECRET", "")
	t.Setenv("AZURE_TENANT_ID", "")

	resp := openEphemeral(t, &azureADTokenEphemeral{}, map[string]tftypes.Value{
		"scopes": tftypes.NewValue(tftypes.List{ElementType: tftypes.String}, []tftypes.Value{str("https://graph.microsoft.com/.default")}),
	})
	if resp.Diagnostics.ErrorsCount() != 3 {
		t.Errorf("want 3 missing-credential errors, got %v", resp.Diagnostics)
	}
}

// An ephemeral token feeds an ephemeral curl2 request.
func TestEphemeralCurl2WithBearerToken(t *testing.T) {
	var auth string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		auth = r.Header.Get("Authorization")
		w.Write([]byte(`{"ok":true}`))
	}))
	defer server.Close()

	resp := openEphemeral(t, &curl2Ephemeral{}, map[string]tftypes.Value{
		"uri":          str(server.URL),
		"http_method":  str("GET"),
		"auth_type":    str("Bearer"),
		"bearer_token": str("secret-token"),
	})
	if resp.Diagnostics.HasError() {
		t.Fatalf("Open failed: %v", resp.Diagnostics)
	}
	if auth != "Bearer secret-token" {
		t.Errorf("Authorization = %q", auth)
	}
	var m curl2DataModelRequest
	resp.Result.Get(context.Background(), &m)
	attrs := m.Response.Attributes()
	if attrs["status_code"].(types.Int64).ValueInt64() != 200 || attrs["body"].(types.String).ValueString() != `{"ok":true}` {
		t.Errorf("response = %v", attrs)
	}
}

func TestProviderRegistersEphemeralResources(t *testing.T) {
	ctx := context.Background()
	var names []string
	for _, newResource := range (&curl2Provider{}).EphemeralResources(ctx) {
		var resp ephemeral.MetadataResponse
		newResource().Metadata(ctx, ephemeral.MetadataRequest{ProviderTypeName: "curl2"}, &resp)
		names = append(names, resp.TypeName)
	}
	want := []string{"curl2", "curl2_azuread_token", "curl2_auth0_token"}
	if len(names) != len(want) {
		t.Fatalf("ephemeral resources = %v, want %v", names, want)
	}
	for i := range want {
		if names[i] != want[i] {
			t.Errorf("ephemeral resources = %v, want %v", names, want)
		}
	}
}
