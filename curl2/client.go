package curl2

import (
	"crypto/tls"
	"net/http"
	"time"
)

type ApiClientOpts struct {
	insecure bool
	timeout  int
}

type HttpClient struct {
	httpClient *http.Client
	timeout    int
}

func NewClient() (*HttpClient, error) {
	opts := ApiClientOpts{
		insecure: false,
		timeout:  0,
	}

	tlsConfig := &tls.Config{
		InsecureSkipVerify: opts.insecure,
	}

	tr := &http.Transport{
		TLSClientConfig: tlsConfig,
		Proxy:           http.ProxyFromEnvironment,
	}

	client := HttpClient{
		httpClient: &http.Client{
			Timeout:   time.Second * time.Duration(opts.timeout),
			Transport: tr,
		},
	}

	return &client, nil
}