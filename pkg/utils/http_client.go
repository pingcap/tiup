// Copyright 2020 PingCAP, Inc.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// See the License for the specific language governing permissions and
// limitations under the License.

package utils

import (
	"crypto/tls"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"time"
)

// HTTPClient is a wrap of http.Client
type HTTPClient struct {
	client *http.Client
}

// NewHTTPClient returns a new HTTP client with timeout and HTTPS support
func NewHTTPClient(timeout time.Duration, tlsConfig *tls.Config) *HTTPClient {
	if timeout < time.Second {
		timeout = 10 * time.Second // default timeout is 10s
	}
	return &HTTPClient{
		client: &http.Client{
			Timeout: timeout,
			Transport: &http.Transport{
				TLSClientConfig: tlsConfig,
			},
		},
	}
}

// Get fetch an URL with GET method and returns the response
func (c *HTTPClient) Get(url string) ([]byte, error) {
	res, err := c.client.Get(url)
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()

	return checkHTTPResponse(res)
}

// Post send a POST request to the url and returns the response
func (c *HTTPClient) Post(url string, body io.Reader) ([]byte, error) {
	res, err := c.client.Post(url, "application/json", body)
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()

	return checkHTTPResponse(res)
}

// Delete send a DELETE request to the url and returns the response and status code.
func (c *HTTPClient) Delete(url string, body io.Reader) ([]byte, int, error) {
	var statusCode int
	req, err := http.NewRequest("DELETE", url, body)
	if err != nil {
		return nil, statusCode, err
	}

	res, err := c.client.Do(req)
	if err != nil {
		return nil, statusCode, err
	}
	defer res.Body.Close()
	b, err := checkHTTPResponse(res)
	statusCode = res.StatusCode
	return b, statusCode, err
}

// checkHTTPResponse checks if an HTTP response is with normal status codes
func checkHTTPResponse(res *http.Response) ([]byte, error) {
	body, err := ioutil.ReadAll(res.Body)
	if err != nil {
		return nil, err
	}
	if res.StatusCode < 200 || res.StatusCode >= 400 {
		return body, fmt.Errorf("error requesting %s, response: %s, code %d",
			res.Request.URL, string(body[:]), res.StatusCode)
	}
	return body, nil
}
