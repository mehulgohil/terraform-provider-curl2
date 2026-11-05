package curl2

import (
	"context"
	"time"

	"github.com/hashicorp/terraform-plugin-framework/ephemeral"
	"github.com/hashicorp/terraform-plugin-framework/ephemeral/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

// Ephemeral resources (Terraform 1.10+). Their values are never written to
// the plan or the state file, so tokens fetched with them stay out of state.

var (
	_ ephemeral.EphemeralResourceWithConfigure = &auth0TokenEphemeral{}
	_ ephemeral.EphemeralResourceWithConfigure = &azureADTokenEphemeral{}
	_ ephemeral.EphemeralResourceWithConfigure = &curl2Ephemeral{}
)

func NewAuth0TokenEphemeral() ephemeral.EphemeralResource   { return &auth0TokenEphemeral{} }
func NewAzureADTokenEphemeral() ephemeral.EphemeralResource { return &azureADTokenEphemeral{} }
func NewCurl2Ephemeral() ephemeral.EphemeralResource        { return &curl2Ephemeral{} }

func clientFrom(req ephemeral.ConfigureRequest) *HttpClient {
	if req.ProviderData == nil {
		return nil
	}
	return req.ProviderData.(*HttpClient)
}

// timeValue is the RFC 3339 form of t, or null when t is zero.
func timeValue(t time.Time) types.String {
	if t.IsZero() {
		return types.StringNull()
	}
	return types.StringValue(t.UTC().Format(time.RFC3339))
}

// ---- curl2_auth0_token ----

type auth0TokenEphemeral struct{ client *HttpClient }

type auth0TokenEphemeralModel struct {
	Audience  types.String `tfsdk:"audience"`
	Token     types.String `tfsdk:"token"`
	ExpiresAt types.String `tfsdk:"expires_at"`
}

func (e *auth0TokenEphemeral) Metadata(_ context.Context, req ephemeral.MetadataRequest, resp *ephemeral.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_auth0_token"
}

func (e *auth0TokenEphemeral) Schema(_ context.Context, _ ephemeral.SchemaRequest, resp *ephemeral.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "Fetches an Auth0 access token with the client-credentials grant, without storing it in the plan or state. Uses the provider's `auth0` block. Requires Terraform 1.10 or later.",
		Attributes: map[string]schema.Attribute{
			"audience":   schema.StringAttribute{Description: "Unique identifier of the API the token is for.", Required: true},
			"token":      schema.StringAttribute{Description: "The access token.", Computed: true, Sensitive: true},
			"expires_at": schema.StringAttribute{Description: "When the token expires, in RFC 3339 format. Null if Auth0 did not say.", Computed: true},
		},
	}
}

func (e *auth0TokenEphemeral) Configure(_ context.Context, req ephemeral.ConfigureRequest, _ *ephemeral.ConfigureResponse) {
	e.client = clientFrom(req)
}

func (e *auth0TokenEphemeral) Open(ctx context.Context, req ephemeral.OpenRequest, resp *ephemeral.OpenResponse) {
	var m auth0TokenEphemeralModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &m)...)
	if resp.Diagnostics.HasError() {
		return
	}

	token, diags := fetchAuth0Token(ctx, e.client, m.Audience.ValueString())
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	m.Token = types.StringValue(token.AccessToken)
	m.ExpiresAt = timeValue(token.ExpiresAt)
	resp.Diagnostics.Append(resp.Result.Set(ctx, &m)...)
}

// ---- curl2_azuread_token ----

type azureADTokenEphemeral struct{}

type azureADTokenEphemeralModel struct {
	Scopes    types.List   `tfsdk:"scopes"`
	Token     types.String `tfsdk:"token"`
	ExpiresOn types.String `tfsdk:"expires_on"`
}

func (e *azureADTokenEphemeral) Metadata(_ context.Context, req ephemeral.MetadataRequest, resp *ephemeral.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_azuread_token"
}

func (e *azureADTokenEphemeral) Schema(_ context.Context, _ ephemeral.SchemaRequest, resp *ephemeral.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "Fetches an Azure AD access token without storing it in the plan or state. Uses the provider's `azure_ad` block. Requires Terraform 1.10 or later.",
		Attributes: map[string]schema.Attribute{
			"scopes": schema.ListAttribute{
				ElementType: types.StringType,
				Description: "Resource identifiers (application ID URIs) with the .default suffix. Example: [\"https://graph.microsoft.com/.default\"]",
				Required:    true,
			},
			"token":      schema.StringAttribute{Description: "The access token.", Computed: true, Sensitive: true},
			"expires_on": schema.StringAttribute{Description: "When the token expires, in RFC 3339 format.", Computed: true},
		},
	}
}

func (e *azureADTokenEphemeral) Configure(context.Context, ephemeral.ConfigureRequest, *ephemeral.ConfigureResponse) {
}

func (e *azureADTokenEphemeral) Open(ctx context.Context, req ephemeral.OpenRequest, resp *ephemeral.OpenResponse) {
	var m azureADTokenEphemeralModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &m)...)
	if resp.Diagnostics.HasError() {
		return
	}

	var scopes []string
	resp.Diagnostics.Append(m.Scopes.ElementsAs(ctx, &scopes, false)...)
	if resp.Diagnostics.HasError() {
		return
	}

	token, expiresOn, diags := fetchAzureADToken(ctx, scopes)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	m.Token = types.StringValue(token)
	m.ExpiresOn = timeValue(expiresOn)
	resp.Diagnostics.Append(resp.Result.Set(ctx, &m)...)
}

// ---- curl2 ----

// curl2Ephemeral is the curl2 request as an ephemeral resource, so it can
// take an ephemeral token (for example from ephemeral.curl2_auth0_token)
// and its response is not stored either.
type curl2Ephemeral struct{ client *HttpClient }

func (e *curl2Ephemeral) Metadata(_ context.Context, req ephemeral.MetadataRequest, resp *ephemeral.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName
}

func (e *curl2Ephemeral) Schema(_ context.Context, _ ephemeral.SchemaRequest, resp *ephemeral.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "Sends an HTTP(s) request without storing the request or the response in the plan or state. Use it with an ephemeral token. Requires Terraform 1.10 or later.",
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

func (e *curl2Ephemeral) Configure(_ context.Context, req ephemeral.ConfigureRequest, _ *ephemeral.ConfigureResponse) {
	e.client = clientFrom(req)
}

func (e *curl2Ephemeral) Open(ctx context.Context, req ephemeral.OpenRequest, resp *ephemeral.OpenResponse) {
	var m curl2DataModelRequest
	resp.Diagnostics.Append(req.Config.Get(ctx, &m)...)
	if resp.Diagnostics.HasError() {
		return
	}

	resp.Diagnostics.Append(runCurl2Request(ctx, e.client, &m)...)
	if resp.Diagnostics.HasError() {
		return
	}

	resp.Diagnostics.Append(resp.Result.Set(ctx, &m)...)
}
