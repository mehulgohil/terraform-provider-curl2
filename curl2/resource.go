package curl2

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"strings"

	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/objectplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

// The curl2 resource manages an object behind a REST API: a request on
// create, an optional request on read (refresh), update and destroy.
// "{id}" in a read, update or destroy uri or json is replaced with the ID
// taken from the create response.

var (
	_ resource.Resource              = &curl2Resource{}
	_ resource.ResourceWithConfigure = &curl2Resource{}
)

func NewCurl2Resource() resource.Resource { return &curl2Resource{} }

type curl2Resource struct{ client *HttpClient }

type requestSpec struct {
	HTTPMethod types.String `tfsdk:"http_method"`
	URI        types.String `tfsdk:"uri"`
	JSON       types.String `tfsdk:"json"`
}

type curl2ResourceModel struct {
	ID                types.String `tfsdk:"id"`
	IDPath            types.String `tfsdk:"id_path"`
	Create            *requestSpec `tfsdk:"create"`
	Read              *requestSpec `tfsdk:"read"`
	Update            *requestSpec `tfsdk:"update"`
	Destroy           *requestSpec `tfsdk:"destroy"`
	Headers           types.Map    `tfsdk:"headers"`
	AuthType          types.String `tfsdk:"auth_type"`
	BearerToken       types.String `tfsdk:"bearer_token"`
	BasicAuthUsername types.String `tfsdk:"basic_auth_username"`
	BasicAuthPassword types.String `tfsdk:"basic_auth_password"`
	Response          types.Object `tfsdk:"response"`
}

var resourceResponseAttrTypes = map[string]attr.Type{
	"status_code": types.Int64Type,
	"body":        types.StringType,
}

func requestSpecAttributes(defaultMethod string) map[string]schema.Attribute {
	return map[string]schema.Attribute{
		"http_method": schema.StringAttribute{Description: fmt.Sprintf("HTTP method. Defaults to %s.", defaultMethod), Optional: true},
		"uri":         schema.StringAttribute{Description: "URI to send the request to. `{id}` is replaced with the resource ID (not in `create`).", Required: true},
		"json":        schema.StringAttribute{Description: "JSON body in string format, e.g. from `jsonencode()`. `{id}` is replaced with the resource ID (not in `create`).", Optional: true},
	}
}

func (r *curl2Resource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName
}

func (r *curl2Resource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "Manages an object behind a REST API with a request on create, and optional requests on read, update and destroy. " +
			"Any response outside 200-299 fails the operation. `{id}` in the read, update and destroy `uri` or `json` is replaced with the ID taken from the create response.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Description:   "The object's ID: the value at `id_path` in the create response, or a hash of the create request if `id_path` is not set.",
				Computed:      true,
				PlanModifiers: []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
			},
			"id_path": schema.StringAttribute{
				Description:   "Where the ID is in the create response JSON, as a dot path such as `id`, `data.id` or `items.0.id`.",
				Optional:      true,
				PlanModifiers: []planmodifier.String{stringplanmodifier.RequiresReplace()},
			},
			"create": schema.SingleNestedAttribute{
				Description: "Request sent on create. If `update` is not set, any change to `create` replaces the object (destroy, then create).",
				Required:    true,
				Attributes:  requestSpecAttributes("POST"),
				PlanModifiers: []planmodifier.Object{objectplanmodifier.RequiresReplaceIf(
					replaceIfNoUpdate,
					"Replaces the object when `update` is not set.",
					"Replaces the object when `update` is not set.",
				)},
			},
			"read": schema.SingleNestedAttribute{
				Description: "Request sent on refresh. A 404 or 410 response means the object was deleted outside Terraform, and it is created again. If not set, refresh sends no request.",
				Optional:    true,
				Attributes:  requestSpecAttributes("GET"),
			},
			"update": schema.SingleNestedAttribute{
				Description: "Request sent when `create` or `update` changes. If not set, those changes replace the object.",
				Optional:    true,
				Attributes:  requestSpecAttributes("PUT"),
			},
			"destroy": schema.SingleNestedAttribute{
				Description: "Request sent on destroy. A 404 or 410 response counts as success. If not set, destroy only removes the object from state.",
				Optional:    true,
				Attributes:  requestSpecAttributes("DELETE"),
			},
			"headers":             schema.MapAttribute{Description: "Headers sent with every request.", ElementType: types.StringType, Optional: true},
			"auth_type":           schema.StringAttribute{Description: descAuthType, Optional: true},
			"bearer_token":        schema.StringAttribute{Description: descBearerToken, Optional: true, Sensitive: true},
			"basic_auth_username": schema.StringAttribute{Description: descBasicAuthUsername, Optional: true},
			"basic_auth_password": schema.StringAttribute{Description: descBasicAuthPassword, Optional: true, Sensitive: true},
			"response": schema.SingleNestedAttribute{
				Description: "Response of the last create, update or read request.",
				Computed:    true,
				Attributes: map[string]schema.Attribute{
					"status_code": schema.Int64Attribute{Computed: true},
					"body":        schema.StringAttribute{Computed: true},
				},
			},
		},
	}
}

func replaceIfNoUpdate(ctx context.Context, req planmodifier.ObjectRequest, resp *objectplanmodifier.RequiresReplaceIfFuncResponse) {
	var update types.Object
	resp.Diagnostics.Append(req.Plan.GetAttribute(ctx, path.Root("update"), &update)...)
	resp.RequiresReplace = update.IsNull()
}

func (r *curl2Resource) Configure(_ context.Context, req resource.ConfigureRequest, _ *resource.ConfigureResponse) {
	if req.ProviderData != nil {
		r.client = req.ProviderData.(*HttpClient)
	}
}

func (m *curl2ResourceModel) auth() authConfig {
	return authConfig{AuthType: m.AuthType, BearerToken: m.BearerToken, BasicAuthUsername: m.BasicAuthUsername, BasicAuthPassword: m.BasicAuthPassword}
}

// call sends one lifecycle request and returns the status code and body.
func (r *curl2Resource) call(ctx context.Context, m *curl2ResourceModel, spec *requestSpec, defaultMethod, id string) (int, []byte, diag.Diagnostics) {
	method := spec.HTTPMethod.ValueString()
	if method == "" {
		method = defaultMethod
	}
	uri := strings.ReplaceAll(spec.URI.ValueString(), "{id}", url.PathEscape(id))
	body := strings.ReplaceAll(spec.JSON.ValueString(), "{id}", id)

	req, diags := newRequest(ctx, method, uri, body, m.Headers, m.auth())
	if diags.HasError() {
		return 0, nil, diags
	}
	status, responseBody, summary, err := send(r.client, req)
	if err != nil {
		diags.AddError(summary, fmt.Sprintf("%s %s: %s", method, uri, err))
	}
	return status, responseBody, diags
}

func is2xx(status int) bool { return status >= 200 && status <= 299 }

func isGone(status int) bool { return status == http.StatusNotFound || status == http.StatusGone }

func statusError(diags *diag.Diagnostics, op string, status int, body []byte) {
	diags.AddError(fmt.Sprintf("%s request failed", op), fmt.Sprintf("The API returned HTTP %d: %s", status, truncate(body, 1000)))
}

func truncate(b []byte, n int) string {
	if len(b) <= n {
		return string(b)
	}
	return string(b[:n]) + "..."
}

func setResponse(m *curl2ResourceModel, status int, body []byte) diag.Diagnostics {
	var diags diag.Diagnostics
	m.Response, diags = types.ObjectValue(resourceResponseAttrTypes, map[string]attr.Value{
		"status_code": types.Int64Value(int64(status)),
		"body":        types.StringValue(string(body)),
	})
	return diags
}

// extractID returns the value at a dot path such as "data.items.0.id".
func extractID(body []byte, idPath string) (string, error) {
	decoder := json.NewDecoder(bytes.NewReader(body))
	decoder.UseNumber()
	var value any
	if err := decoder.Decode(&value); err != nil {
		return "", fmt.Errorf("the create response is not JSON: %w", err)
	}
	for _, key := range strings.Split(idPath, ".") {
		switch node := value.(type) {
		case map[string]any:
			next, ok := node[key]
			if !ok {
				return "", fmt.Errorf("the create response has no %q", idPath)
			}
			value = next
		case []any:
			i, err := strconv.Atoi(key)
			if err != nil || i < 0 || i >= len(node) {
				return "", fmt.Errorf("the create response has no %q", idPath)
			}
			value = node[i]
		default:
			return "", fmt.Errorf("the create response has no %q", idPath)
		}
	}
	switch v := value.(type) {
	case string:
		return v, nil
	case json.Number:
		return v.String(), nil
	case bool:
		return strconv.FormatBool(v), nil
	default:
		return "", fmt.Errorf("the value at %q is not a string, number or boolean", idPath)
	}
}

func (r *curl2Resource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var m curl2ResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &m)...)
	if resp.Diagnostics.HasError() {
		return
	}

	status, body, diags := r.call(ctx, &m, m.Create, http.MethodPost, "")
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}
	if !is2xx(status) {
		statusError(&resp.Diagnostics, "Create", status, body)
		return
	}

	if m.IDPath.ValueString() != "" {
		id, err := extractID(body, m.IDPath.ValueString())
		if err != nil {
			resp.Diagnostics.AddError("Cannot read the ID from the create response",
				err.Error()+". The object may have been created; check the API before applying again.")
			return
		}
		m.ID = types.StringValue(id)
	} else {
		sum := sha256.Sum256([]byte(m.Create.HTTPMethod.ValueString() + " " + m.Create.URI.ValueString() + " " + m.Create.JSON.ValueString()))
		m.ID = types.StringValue(hex.EncodeToString(sum[:8]))
	}

	resp.Diagnostics.Append(setResponse(&m, status, body)...)
	resp.Diagnostics.Append(resp.State.Set(ctx, &m)...)
}

func (r *curl2Resource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var m curl2ResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &m)...)
	if resp.Diagnostics.HasError() || m.Read == nil {
		return
	}

	status, body, diags := r.call(ctx, &m, m.Read, http.MethodGet, m.ID.ValueString())
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}
	if isGone(status) {
		resp.State.RemoveResource(ctx)
		return
	}
	if !is2xx(status) {
		statusError(&resp.Diagnostics, "Read", status, body)
		return
	}

	resp.Diagnostics.Append(setResponse(&m, status, body)...)
	resp.Diagnostics.Append(resp.State.Set(ctx, &m)...)
}

func (r *curl2Resource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan, state curl2ResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	plan.ID = state.ID

	// Only a change to what the object should be sends the update request.
	// A change to headers, auth, read or destroy is just saved.
	if plan.Update != nil && (!specEqual(plan.Create, state.Create) || !specEqual(plan.Update, state.Update)) {
		status, body, diags := r.call(ctx, &plan, plan.Update, http.MethodPut, plan.ID.ValueString())
		resp.Diagnostics.Append(diags...)
		if resp.Diagnostics.HasError() {
			return
		}
		if !is2xx(status) {
			statusError(&resp.Diagnostics, "Update", status, body)
			return
		}
		resp.Diagnostics.Append(setResponse(&plan, status, body)...)
	} else {
		plan.Response = state.Response
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func specEqual(a, b *requestSpec) bool {
	if a == nil || b == nil {
		return a == b
	}
	return a.HTTPMethod.Equal(b.HTTPMethod) && a.URI.Equal(b.URI) && a.JSON.Equal(b.JSON)
}

func (r *curl2Resource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var m curl2ResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &m)...)
	if resp.Diagnostics.HasError() || m.Destroy == nil {
		return
	}

	status, body, diags := r.call(ctx, &m, m.Destroy, http.MethodDelete, m.ID.ValueString())
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}
	if !is2xx(status) && !isGone(status) {
		statusError(&resp.Diagnostics, "Destroy", status, body)
	}
}
