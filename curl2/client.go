package curl2

import (
	"crypto/tls"
	"net/http"
	"time"

	"github.com/hashicorp/go-retryablehttp"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

type ApiClientOpts struct {
	insecure   bool
	timeout    int64
	maxRetries int
	minDelay   types.Int64
	maxDelay   types.Int64
}

type HttpClient struct {
	httpClient *retryablehttp.Client
}

// NewClient builds the HTTP client every data source uses.
//
// The TLS setting and the timeout go on retryClient.HTTPClient, the client
// that sends each attempt. They used to be set on a copy returned by
// StandardClient() that nothing used, so disable_tls and timeout_ms had no
// effect.
func NewClient(opts ApiClientOpts) *HttpClient {
	retryClient := retryablehttp.NewClient()
	retryClient.RetryMax = opts.maxRetries

	if !opts.minDelay.IsNull() && !opts.minDelay.IsUnknown() && opts.minDelay.ValueInt64() >= 0 {
		retryClient.RetryWaitMin = time.Duration(opts.minDelay.ValueInt64()) * time.Millisecond
	}

	if !opts.maxDelay.IsNull() && !opts.maxDelay.IsUnknown() && opts.maxDelay.ValueInt64() >= 0 {
		retryClient.RetryWaitMax = time.Duration(opts.maxDelay.ValueInt64()) * time.Millisecond
	}

	transport := http.DefaultTransport.(*http.Transport).Clone() // keeps ProxyFromEnvironment
	transport.TLSClientConfig = &tls.Config{InsecureSkipVerify: opts.insecure}

	retryClient.HTTPClient = &http.Client{
		Transport: transport,
		Timeout:   time.Duration(opts.timeout) * time.Millisecond, // 0 means no timeout
	}

	return &HttpClient{httpClient: retryClient}
}
