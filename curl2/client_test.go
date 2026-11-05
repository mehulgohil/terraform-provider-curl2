package curl2

import (
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	"github.com/hashicorp/go-retryablehttp"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

func get(t *testing.T, c *HttpClient, url string) (*http.Response, error) {
	t.Helper()
	req, err := retryablehttp.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		t.Fatal(err)
	}
	resp, err := c.httpClient.Do(req)
	if err == nil {
		resp.Body.Close()
	}
	return resp, err
}

func TestClientDisableTLS(t *testing.T) {
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))
	defer server.Close()

	if _, err := get(t, NewClient(ApiClientOpts{}), server.URL); err == nil {
		t.Error("self-signed certificate accepted with disable_tls unset")
	}
	if _, err := get(t, NewClient(ApiClientOpts{insecure: true}), server.URL); err != nil {
		t.Errorf("disable_tls = true still verified the certificate: %v", err)
	}
}

func TestClientTimeout(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(300 * time.Millisecond)
	}))
	defer server.Close()

	if _, err := get(t, NewClient(ApiClientOpts{timeout: 50}), server.URL); err == nil {
		t.Error("timeout_ms = 50 did not stop a 300ms request")
	}
	if _, err := get(t, NewClient(ApiClientOpts{}), server.URL); err != nil {
		t.Errorf("no timeout set, request failed: %v", err)
	}
}

func TestClientRetries(t *testing.T) {
	var calls atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if calls.Add(1) <= 2 {
			w.WriteHeader(http.StatusServiceUnavailable)
		}
	}))
	defer server.Close()

	client := NewClient(ApiClientOpts{
		maxRetries: 2,
		minDelay:   types.Int64Value(1),
		maxDelay:   types.Int64Value(5),
	})
	resp, err := get(t, client, server.URL)
	if err != nil {
		t.Fatalf("request failed after retries: %v", err)
	}
	if resp.StatusCode != http.StatusOK || calls.Load() != 3 {
		t.Errorf("status %d after %d calls, want 200 after 3", resp.StatusCode, calls.Load())
	}
}
