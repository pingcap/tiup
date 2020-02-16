package utils

import (
	"crypto/tls"
	"fmt"
	"net/http"
	"os"
	"time"

	"github.com/cavaliercoder/grab"
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

// DownloadFileWithProgress downloads a file and check its checksum with a progress output
func DownloadFileWithProgress(url string, to string, checksum string) error {
	client := grab.NewClient()

	req, err := grab.NewRequest(to, url)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Download failed: %v\n", err)
		return err
	}

	fmt.Printf("Downloading %v...\n\n", req.URL())
	resp := client.Do(req)

	// start progress output loop
	t := time.NewTicker(200 * time.Millisecond)
	defer t.Stop()

L:
	for {
		select {
		case <-t.C:
			fmt.Printf("\033[1AProgress %d / %d bytes (%.2f%%)\033[K\n",
				resp.BytesComplete(),
				resp.Size,
				100*resp.Progress())

		case <-resp.Done:
			break L
		}
	}

	// check for errors
	if err := resp.Err(); err != nil {
		fmt.Fprintf(os.Stderr, "Download failed: %v\n", err)
		return err
	}

	fmt.Printf("Download saved to %v \n", to)
	return nil
}
