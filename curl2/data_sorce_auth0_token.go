package curl2

import (
	"bytes"
	"context"
	"encoding/json"
	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"io"
	"net/http"
	"os"
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

type tokenRequestBody struct {
	ClientID     string `json:"client_id"`
	ClientSecret string `json:"client_secret"`
	Audience     string `json:"audience"`
	GrantType    string `json:"grant_type"`
}

type tokenResponseBody struct {
	AccessToken string `json:"access_token"`
	TokenType   string `json:"token_type"`
}
type auth0TokenDataSource struct{}

func (a *auth0TokenDataSource) Configure(ctx context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
	if req.ProviderData == nil {
		return
	}
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

	diags := req.Config.Get(ctx, &config)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	clientID := os.Getenv("AUTH0_CLIENT_ID")
	clientSecret := os.Getenv("AUTH0_CLIENT_SECRET")
	domain := os.Getenv("AUTH0_DOMAIN")

	if clientID == "" {
		resp.Diagnostics.AddAttributeError(
			path.Root("client_id"),
			"Missing Auth0 Client ID",
			"The data source cannot get the token as client id is missing in provider auth0 block",
		)
	}

	if clientSecret == "" {
		resp.Diagnostics.AddAttributeError(
			path.Root("client_secret"),
			"Missing Auth0 Client Secret",
			"The data source cannot get the token as client secret is missing in provider auth0 block",
		)
	}

	if domain == "" {
		resp.Diagnostics.AddAttributeError(
			path.Root("domain"),
			"Missing Auth0 Domain",
			"The data source cannot get the token as domain is missing in provider auth0 block",
		)
	}

	if config.Audience.ValueString() == "" {
		resp.Diagnostics.AddAttributeError(
			path.Root("audience"),
			"Missing Auth0 Audience",
			"The data source cannot get the token as audience is missing",
		)
	}

	if resp.Diagnostics.HasError() {
		return
	}

	url := domain + "/oauth/token"

	payload, err := json.Marshal(tokenRequestBody{
		ClientID:     clientID,
		ClientSecret: clientSecret,
		Audience:     config.Audience.ValueString(),
		GrantType:    "client_credentials",
	})
	if err != nil {
		resp.Diagnostics.AddError(
			"Error marshalling json to get auth0 token",
			err.Error(),
		)
		return
	}

	request, err := http.NewRequest("POST", url, bytes.NewReader(payload))
	if err != nil {
		resp.Diagnostics.AddError(
			"Error generating new request auth0 token",
			err.Error(),
		)
		return
	}
	request.Header.Add("content-type", "application/json")

	res, err := http.DefaultClient.Do(request)
	if err != nil {
		resp.Diagnostics.AddError(
			"Error sending HTTP request to get auth0 token",
			err.Error(),
		)
		return
	}

	defer res.Body.Close()
	body, err := io.ReadAll(res.Body)
	if err != nil {
		resp.Diagnostics.AddError(
			"Error reading body of response from auth0",
			err.Error(),
		)
		return
	}

	var response tokenResponseBody
	err = json.Unmarshal(body, &response)
	if err != nil {
		resp.Diagnostics.AddError(
			"Error unmarshalling body of response from auth0",
			err.Error(),
		)
		return
	}

	config.Response, diags = types.ObjectValue(
		map[string]attr.Type{
			"token": types.StringType,
		},
		map[string]attr.Value{
			"token": types.StringValue(response.AccessToken),
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
