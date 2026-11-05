package curl2

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"time"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore/policy"
	"github.com/Azure/azure-sdk-for-go/sdk/azidentity"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/path"
)

// Token fetching shared by the curl2_auth0_token and curl2_azuread_token
// data sources and their ephemeral resources. Credentials come from the
// provider's auth0 / azure_ad blocks, which Configure copies into these
// environment variables.

type tokenRequestBody struct {
	ClientID     string `json:"client_id"`
	ClientSecret string `json:"client_secret"`
	Audience     string `json:"audience"`
	GrantType    string `json:"grant_type"`
}

type tokenResponseBody struct {
	AccessToken string `json:"access_token"`
	TokenType   string `json:"token_type"`
	ExpiresIn   int64  `json:"expires_in"`
}

type auth0Token struct {
	AccessToken string
	ExpiresAt   time.Time // zero if Auth0 did not say
}

func requireEnv(diags *diag.Diagnostics, name, summary, block string) string {
	value := os.Getenv(name)
	if value == "" {
		diags.AddError(summary, fmt.Sprintf("Set it in the provider's %s block or the %s environment variable.", block, name))
	}
	return value
}

// fetchAuth0Token gets an access token with the client-credentials grant.
func fetchAuth0Token(ctx context.Context, client *HttpClient, audience string) (auth0Token, diag.Diagnostics) {
	var diags diag.Diagnostics
	clientID := requireEnv(&diags, "AUTH0_CLIENT_ID", "Missing Auth0 Client ID", "auth0")
	clientSecret := requireEnv(&diags, "AUTH0_CLIENT_SECRET", "Missing Auth0 Client Secret", "auth0")
	domain := requireEnv(&diags, "AUTH0_DOMAIN", "Missing Auth0 Domain", "auth0")
	if audience == "" {
		diags.AddAttributeError(path.Root("audience"), "Missing Auth0 Audience", "The token cannot be fetched as audience is missing.")
	}
	if diags.HasError() {
		return auth0Token{}, diags
	}

	payload, err := json.Marshal(tokenRequestBody{
		ClientID:     clientID,
		ClientSecret: clientSecret,
		Audience:     audience,
		GrantType:    "client_credentials",
	})
	if err != nil {
		diags.AddError("Error marshalling json to get auth0 token", err.Error())
		return auth0Token{}, diags
	}

	request, err := http.NewRequestWithContext(ctx, http.MethodPost, domain+"/oauth/token", bytes.NewReader(payload))
	if err != nil {
		diags.AddError("Error generating new request auth0 token", err.Error())
		return auth0Token{}, diags
	}
	request.Header.Set("content-type", "application/json")

	// Use the provider's HTTP client so disable_tls and timeout_ms apply.
	httpClient := http.DefaultClient
	if client != nil {
		httpClient = client.httpClient.HTTPClient
	}
	res, err := httpClient.Do(request)
	if err != nil {
		diags.AddError("Error sending HTTP request to get auth0 token", err.Error())
		return auth0Token{}, diags
	}
	defer res.Body.Close()

	body, err := io.ReadAll(res.Body)
	if err != nil {
		diags.AddError("Error reading body of response from auth0", err.Error())
		return auth0Token{}, diags
	}
	if res.StatusCode != http.StatusOK {
		diags.AddError("Auth0 did not return a token", fmt.Sprintf("%s returned HTTP %d: %s", domain+"/oauth/token", res.StatusCode, body))
		return auth0Token{}, diags
	}

	var response tokenResponseBody
	if err := json.Unmarshal(body, &response); err != nil {
		diags.AddError("Error unmarshalling body of response from auth0", err.Error())
		return auth0Token{}, diags
	}
	if response.AccessToken == "" {
		diags.AddError("Auth0 did not return a token", "The response had no access_token.")
		return auth0Token{}, diags
	}

	token := auth0Token{AccessToken: response.AccessToken}
	if response.ExpiresIn > 0 {
		token.ExpiresAt = time.Now().Add(time.Duration(response.ExpiresIn) * time.Second)
	}
	return token, diags
}

// fetchAzureADToken gets an access token for the given scopes.
func fetchAzureADToken(ctx context.Context, scopes []string) (string, time.Time, diag.Diagnostics) {
	var diags diag.Diagnostics
	requireEnv(&diags, "AZURE_CLIENT_ID", "Missing Client ID", "azure_ad")
	requireEnv(&diags, "AZURE_CLIENT_SECRET", "Missing Client Secret", "azure_ad")
	tenantID := requireEnv(&diags, "AZURE_TENANT_ID", "Missing Tenant ID", "azure_ad")
	if len(scopes) == 0 {
		diags.AddAttributeError(path.Root("scopes"), "Missing Scopes", "The token cannot be fetched as scopes are missing.")
	}
	if diags.HasError() {
		return "", time.Time{}, diags
	}

	cred, err := azidentity.NewDefaultAzureCredential(nil)
	if err != nil {
		diags.AddError("Error getting default azure credentials", err.Error())
		return "", time.Time{}, diags
	}

	token, err := cred.GetToken(ctx, policy.TokenRequestOptions{TenantID: tenantID, Scopes: scopes})
	if err != nil {
		diags.AddError("Error getting azure ad token", err.Error())
		return "", time.Time{}, diags
	}
	return token.Token, token.ExpiresOn, diags
}
