package curl2

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/providerserver"
	"github.com/hashicorp/terraform-plugin-go/tfprotov6"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/plancheck"
	"github.com/hashicorp/terraform-plugin-testing/terraform"
)

func TestExtractID(t *testing.T) {
	body := []byte(`{"id": 42, "name": "x", "data": {"uuid": "ab-1", "items": [{"id": "first"}]}, "ok": true}`)
	cases := map[string]string{"id": "42", "data.uuid": "ab-1", "data.items.0.id": "first", "ok": "true"}
	for path, want := range cases {
		got, err := extractID(body, path)
		if err != nil || got != want {
			t.Errorf("extractID(%q) = %q, %v; want %q", path, got, err, want)
		}
	}
	for _, path := range []string{"missing", "data.items.5.id", "data", "name.x"} {
		if _, err := extractID(body, path); err == nil {
			t.Errorf("extractID(%q) did not fail", path)
		}
	}
	if _, err := extractID([]byte("not json"), "id"); err == nil {
		t.Error("non-JSON body did not fail")
	}
}

// fakeAPI is an in-memory REST API: POST /hooks, GET/PUT/DELETE /hooks/{id}.
type fakeAPI struct {
	mu      sync.Mutex
	nextID  int
	objects map[string]string // id -> JSON body
	calls   []string
	server  *httptest.Server
}

func newFakeAPI(t *testing.T) *fakeAPI {
	api := &fakeAPI{objects: map[string]string{}}
	api.server = httptest.NewServer(http.HandlerFunc(api.handle))
	t.Cleanup(api.server.Close)
	return api
}

func (a *fakeAPI) handle(w http.ResponseWriter, r *http.Request) {
	a.mu.Lock()
	defer a.mu.Unlock()
	body, _ := io.ReadAll(r.Body)
	a.calls = append(a.calls, r.Method+" "+r.URL.Path)
	if r.Header.Get("Authorization") != "Bearer test-token" {
		w.WriteHeader(http.StatusUnauthorized)
		return
	}

	id := strings.TrimPrefix(r.URL.Path, "/hooks/")
	switch {
	case r.Method == http.MethodPost && r.URL.Path == "/hooks":
		a.nextID++
		id = strconv.Itoa(a.nextID)
		a.objects[id] = string(body)
		w.WriteHeader(http.StatusCreated)
		fmt.Fprintf(w, `{"data":{"id":%s},"stored":%s}`, id, body)
	case a.objects[id] == "":
		w.WriteHeader(http.StatusNotFound)
	case r.Method == http.MethodGet:
		fmt.Fprint(w, a.objects[id])
	case r.Method == http.MethodPut:
		a.objects[id] = string(body)
		fmt.Fprint(w, string(body))
	case r.Method == http.MethodDelete:
		delete(a.objects, id)
		w.WriteHeader(http.StatusNoContent)
	default:
		w.WriteHeader(http.StatusMethodNotAllowed)
	}
}

func (a *fakeAPI) object(id string) map[string]any {
	a.mu.Lock()
	defer a.mu.Unlock()
	var v map[string]any
	json.Unmarshal([]byte(a.objects[id]), &v)
	return v
}

func (a *fakeAPI) count() int {
	a.mu.Lock()
	defer a.mu.Unlock()
	return len(a.objects)
}

var testProviders = map[string]func() (tfprotov6.ProviderServer, error){
	"curl2": providerserver.NewProtocol6WithError(NewProvider()),
}

func hookConfig(baseURL, name string, withUpdate bool) string {
	update := ""
	if withUpdate {
		update = `update = { uri = "` + baseURL + `/hooks/{id}", json = jsonencode({ name = "` + name + `", id = "{id}" }) }`
	}
	return `
resource "curl2" "hook" {
  id_path      = "data.id"
  auth_type    = "Bearer"
  bearer_token = "test-token"
  create  = { uri = "` + baseURL + `/hooks", json = jsonencode({ name = "` + name + `" }) }
  read    = { uri = "` + baseURL + `/hooks/{id}" }
  ` + update + `
  destroy = { uri = "` + baseURL + `/hooks/{id}" }
}
`
}

func checkAPI(api *fakeAPI, id, wantName string) resource.TestCheckFunc {
	return func(*terraform.State) error {
		if got := api.object(id)["name"]; got != wantName {
			return fmt.Errorf("API object %s has name %v, want %q", id, got, wantName)
		}
		return nil
	}
}

// Create, in-place update, refresh after an outside delete, and destroy,
// run by real Terraform. Needs TF_ACC=1 and a terraform binary.
func TestAccCurl2ResourceLifecycle(t *testing.T) {
	api := newFakeAPI(t)
	url := api.server.URL

	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testProviders,
		CheckDestroy: func(*terraform.State) error {
			if n := api.count(); n != 0 {
				return fmt.Errorf("%d objects left in the API after destroy", n)
			}
			return nil
		},
		Steps: []resource.TestStep{
			{
				Config: hookConfig(url, "first", true),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("curl2.hook", "id", "1"),
					resource.TestCheckResourceAttr("curl2.hook", "response.status_code", "201"), // from the create response
					checkAPI(api, "1", "first"),
				),
			},
			{
				// update is set, so a change updates in place with PUT /hooks/1.
				Config: hookConfig(url, "second", true),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{plancheck.ExpectResourceAction("curl2.hook", plancheck.ResourceActionUpdate)},
				},
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("curl2.hook", "id", "1"),
					checkAPI(api, "1", "second"),
					func(*terraform.State) error {
						if got := api.object("1")["id"]; got != "1" {
							return fmt.Errorf("{id} in update json became %v, want \"1\"", got)
						}
						return nil
					},
				),
			},
			{
				// Deleted outside Terraform: read gets 404, so it is created again.
				PreConfig: func() {
					api.mu.Lock()
					delete(api.objects, "1")
					api.mu.Unlock()
				},
				Config: hookConfig(url, "second", true),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{plancheck.ExpectResourceAction("curl2.hook", plancheck.ResourceActionCreate)},
				},
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("curl2.hook", "id", "2"),
					checkAPI(api, "2", "second"),
				),
			},
			{
				// Only headers change: saved in state, no PUT sent.
				Config: strings.Replace(hookConfig(url, "second", true), `auth_type    = "Bearer"`, `auth_type    = "Bearer"
  headers      = { X-Trace = "on" }`, 1),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{plancheck.ExpectResourceAction("curl2.hook", plancheck.ResourceActionUpdate)},
				},
				Check: func(*terraform.State) error {
					api.mu.Lock()
					defer api.mu.Unlock()
					puts := 0
					for _, c := range api.calls {
						if strings.HasPrefix(c, "PUT ") {
							puts++
						}
					}
					if puts != 1 {
						return fmt.Errorf("%d PUT requests, want 1 (a header change must not send one): %v", puts, api.calls)
					}
					return nil
				},
			},
		},
	})
}

// Without update, a change to create destroys the old object and creates a new one.
func TestAccCurl2ResourceReplaceWithoutUpdate(t *testing.T) {
	api := newFakeAPI(t)
	url := api.server.URL

	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testProviders,
		Steps: []resource.TestStep{
			{
				Config: hookConfig(url, "first", false),
				Check:  resource.TestCheckResourceAttr("curl2.hook", "id", "1"),
			},
			{
				Config: hookConfig(url, "second", false),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{plancheck.ExpectResourceAction("curl2.hook", plancheck.ResourceActionDestroyBeforeCreate)},
				},
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("curl2.hook", "id", "2"),
					checkAPI(api, "2", "second"),
					func(*terraform.State) error {
						if api.count() != 1 {
							return fmt.Errorf("%d objects in the API, want 1 (the old one destroyed)", api.count())
						}
						return nil
					},
				),
			},
		},
	})
}

// A non-2xx create response fails the apply and stores nothing.
func TestAccCurl2ResourceCreateFails(t *testing.T) {
	api := newFakeAPI(t)
	config := strings.Replace(hookConfig(api.server.URL, "x", true), `bearer_token = "test-token"`, `bearer_token = "wrong"`, 1)

	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testProviders,
		Steps: []resource.TestStep{{
			Config:      config,
			ExpectError: regexpMust(`Create request failed[\s\S]*HTTP 401`),
		}},
	})
}

func regexpMust(s string) *regexp.Regexp { return regexp.MustCompile(s) }
