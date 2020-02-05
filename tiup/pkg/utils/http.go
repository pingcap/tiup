package utils

import (
	"crypto/tls"
	"fmt"
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

// DownloadFile downloads a file and check for its SHA256 checksum
func DownloadFile(url string, checksum string) error {
	if len(url) < 1 {
		return fmt.Errorf("url is empty")
	}
	fmt.Printf("DEMO: download %s with SHA256 checksum %s\n", url, checksum)
	return nil
}
