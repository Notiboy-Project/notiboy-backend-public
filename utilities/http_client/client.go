package http_client

import (
	"crypto/tls"
	"net/http"
	"time"
)

var httpClient *http.Client

func GetClient() *http.Client {
	if httpClient != nil {
		return httpClient
	}

	httpClient = &http.Client{
		Transport: &http.Transport{
			MaxIdleConnsPerHost: 10,
			MaxIdleConns:        10,
			IdleConnTimeout:     30 * time.Second,
			DisableKeepAlives:   false,
			TLSClientConfig: &tls.Config{
				InsecureSkipVerify: false,
			},
		},
		Timeout: time.Second * 30,
	}

	return httpClient
}
