package curl2

import (
	"net/http"
	"net/http/httptest"
	"regexp"
	"sync/atomic"
	"testing"
	"time"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
)

// The 1.6.x fixes, run by real Terraform: header values (#13), the retry
// block, timeout_ms and ignore_errors (#15). Needs TF_ACC=1.
func TestAccCurl2DataSource(t *testing.T) {
	var calls atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/flaky": // fails twice per plan or apply, then works
			if calls.Add(1)%3 != 0 {
				w.WriteHeader(http.StatusServiceUnavailable)
				return
			}
			w.Write([]byte("recovered"))
		case "/slow":
			time.Sleep(500 * time.Millisecond)
		default:
			w.Write([]byte(r.Header.Get("X-GitHub-Api-Version")))
		}
	}))
	defer server.Close()
	closed := closedServerURL()

	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testProviders,
		Steps: []resource.TestStep{
			{
				Config: `
provider "curl2" {
  retry {
    retry_attempts = 2
    min_delay_ms   = 1
    max_delay_ms   = 5
  }
}

data "curl2" "headers" {
  http_method = "GET"
  uri         = "` + server.URL + `/echo"
  headers     = { X-GitHub-Api-Version = "2022-11-28" }
}

data "curl2" "flaky" {
  http_method = "GET"
  uri         = "` + server.URL + `/flaky"
}

data "curl2" "unreachable" {
  http_method   = "GET"
  uri           = "` + closed + `"
  ignore_errors = true
}
`,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("data.curl2.headers", "response.body", "2022-11-28"),
					resource.TestCheckResourceAttr("data.curl2.flaky", "response.body", "recovered"),
					resource.TestCheckResourceAttr("data.curl2.unreachable", "response.status_code", "0"),
					resource.TestMatchResourceAttr("data.curl2.unreachable", "error", regexp.MustCompile("Error calling api")),
				),
			},
			{
				Config: `
provider "curl2" {
  timeout_ms = 100
}

data "curl2" "slow" {
  http_method = "GET"
  uri         = "` + server.URL + `/slow"
}
`,
				ExpectError: regexp.MustCompile(`Error calling api`),
			},
		},
	})
}
