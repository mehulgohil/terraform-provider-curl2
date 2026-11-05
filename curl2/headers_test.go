package curl2

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/hashicorp/go-retryablehttp"
	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

// Regression test for issue #13: header values were sent wrapped in quotes.
func TestSetHeadersSendsRawValues(t *testing.T) {
	var got http.Header
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		got = r.Header.Clone()
	}))
	defer server.Close()

	headers := types.MapValueMust(types.StringType, map[string]attr.Value{
		"Accept":               types.StringValue("application/vnd.github+json"),
		"X-GitHub-Api-Version": types.StringValue("2022-11-28"),
	})

	req, err := retryablehttp.NewRequest(http.MethodGet, server.URL, nil)
	if err != nil {
		t.Fatal(err)
	}
	if diags := setHeaders(context.Background(), req, headers); diags.HasError() {
		t.Fatalf("setHeaders: %v", diags)
	}

	resp, err := retryablehttp.NewClient().Do(req)
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()

	want := map[string]string{
		"Accept":               "application/vnd.github+json",
		"X-Github-Api-Version": "2022-11-28",
	}
	for key, value := range want {
		if got.Get(key) != value {
			t.Errorf("header %s = %q, want %q", key, got.Get(key), value)
		}
	}
}

func TestSetHeadersNullMap(t *testing.T) {
	req, err := retryablehttp.NewRequest(http.MethodGet, "http://example.invalid", nil)
	if err != nil {
		t.Fatal(err)
	}
	if diags := setHeaders(context.Background(), req, types.MapNull(types.StringType)); diags.HasError() {
		t.Fatalf("setHeaders: %v", diags)
	}
	if len(req.Header) != 0 {
		t.Errorf("expected no headers, got %v", req.Header)
	}
}
