package utils

import (
	"crypto/tls"
	"fmt"
	"net/http"
	"os"
	"time"

	"code.cloudfoundry.org/bytefmt"
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

// DownloadFileWithProgress downloads a file and check its checksum with a progress output
// returns download file
// TODO: checksum
func DownloadFileWithProgress(url string, to string, checksum string) (string, error) {
	client := grab.NewClient()

	req, err := grab.NewRequest(to, url)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Download failed: %v\n", err)
		return "", err
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
			fmt.Printf("\033[1AProgress %s / %s bytes (%.2f%%)\033[K\n",
				bytefmt.ByteSize(uint64(resp.BytesComplete())),
				bytefmt.ByteSize(uint64(resp.Size)),
				100*resp.Progress())

		case <-resp.Done:
			break L
		}
	}

	// check for errors
	if err := resp.Err(); err != nil {
		fmt.Fprintf(os.Stderr, "Download failed: %v\n", err)
		return "", err
	}
	// TODO: checksum

	fmt.Printf("Download saved to %v \n", resp.Filename)

	return resp.Filename, nil
}
