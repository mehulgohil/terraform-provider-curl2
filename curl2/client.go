package curl2

import (
	"crypto/tls"
	"github.com/hashicorp/go-retryablehttp"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"net/http"
	"time"
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

func NewClient(opts ApiClientOpts) *HttpClient {
	retryClient := retryablehttp.NewClient()
	retryClient.RetryMax = opts.maxRetries

	if !opts.minDelay.IsNull() && !opts.minDelay.IsUnknown() && opts.minDelay.ValueInt64() >= 0 {
		retryClient.RetryWaitMin = time.Duration(opts.minDelay.ValueInt64()) * time.Millisecond
	}

	if !opts.maxDelay.IsNull() && !opts.maxDelay.IsUnknown() && opts.maxDelay.ValueInt64() >= 0 {
		retryClient.RetryWaitMax = time.Duration(opts.maxDelay.ValueInt64()) * time.Millisecond
	}

	standardClient := retryClient.StandardClient()

	tlsConfig := &tls.Config{
		InsecureSkipVerify: opts.insecure,
	}
	standardClient.Transport = &http.Transport{
		TLSClientConfig: tlsConfig,
		Proxy:           http.ProxyFromEnvironment,
	}

	tr := &http.Transport{
		TLSClientConfig: tlsConfig,
		Proxy:           http.ProxyFromEnvironment,
	}
	standardClient.Transport = tr

	client := HttpClient{
		httpClient: retryClient,
	}

	return &client
}
