package utils

import (
	"crypto/tls"
	"net/http"
	"time"
)

// Client is a wrap of http.Client to fit our usage
type Client struct {
	URL     string
	client  *http.Client
	timeout time.Duration
}

// NewClient creates a new Client
func NewClient(url string, tlsConfig *tls.Config, timeout ...time.Duration) *Client {
	client := &Client{
		URL:     url,
		timeout: time.Second * 10, // default timeout is 10s
		client: &http.Client{
			Transport: &http.Transport{TLSClientConfig: tlsConfig},
		},
	}

	if len(timeout) > 0 {
		client.timeout = timeout[0]
	}

	return client
}

// Get is a wrap of http.Client.Get(), that send the GET request to server
func (c *Client) Get() (*http.Response, error) {
	return c.client.Get(c.URL)
}
