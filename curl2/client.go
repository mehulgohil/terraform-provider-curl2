package curl2

import (
	"crypto/tls"
	"net/http"
	"time"
)

type ApiClientOpts struct {
	insecure bool
	timeout  int64
}

type HttpClient struct {
	httpClient *http.Client
}

func NewClient(opts ApiClientOpts) (*HttpClient, error) {
	tlsConfig := &tls.Config{
		InsecureSkipVerify: opts.insecure,
	}

	tr := &http.Transport{
		TLSClientConfig: tlsConfig,
		Proxy:           http.ProxyFromEnvironment,
	}

	client := HttpClient{
		httpClient: &http.Client{
			Timeout:   time.Millisecond * time.Duration(opts.timeout),
			Transport: tr,
		},
	}

	return &client, nil
}
