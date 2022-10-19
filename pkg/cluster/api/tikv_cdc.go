// Copyright 2022 PingCAP, Inc.
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

package api

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/pingcap/errors"
	logprinter "github.com/pingcap/tiup/pkg/logger/printer"
	"github.com/pingcap/tiup/pkg/utils"
)

// TiKVCDCOpenAPIClient is client for access TiKVCDC Open API
type TiKVCDCOpenAPIClient struct {
	urls   []string
	client *utils.HTTPClient
	ctx    context.Context
}

// NewTiKVCDCOpenAPIClient return a `TiKVCDCOpenAPIClient`
func NewTiKVCDCOpenAPIClient(ctx context.Context, addresses []string, timeout time.Duration, tlsConfig *tls.Config) *TiKVCDCOpenAPIClient {
	httpPrefix := "http"
	if tlsConfig != nil {
		httpPrefix = "https"
	}
	urls := make([]string, 0, len(addresses))
	for _, addr := range addresses {
		urls = append(urls, fmt.Sprintf("%s://%s", httpPrefix, addr))
	}

	return &TiKVCDCOpenAPIClient{
		urls:   urls,
		client: utils.NewHTTPClient(timeout, tlsConfig),
		ctx:    ctx,
	}
}

func (c *TiKVCDCOpenAPIClient) getEndpoints(api string) (endpoints []string) {
	for _, url := range c.urls {
		endpoints = append(endpoints, fmt.Sprintf("%s/%s", url, api))
	}
	return endpoints
}

// ResignOwner resign the TiKV-CDC owner, and wait for a new owner be found
func (c *TiKVCDCOpenAPIClient) ResignOwner() error {
	api := "api/v1/owner/resign"
	endpoints := c.getEndpoints(api)
	_, err := tryURLs(endpoints, func(endpoint string) ([]byte, error) {
		body, statusCode, err := c.client.PostWithStatusCode(c.ctx, endpoint, nil)
		if err != nil {
			if statusCode == http.StatusNotFound {
				c.l().Debugf("resign owner does not found, ignore it, err: %+v", err)
				return body, nil
			}
			return body, err
		}
		return body, nil
	})

	if err != nil {
		return err
	}

	owner, err := c.GetOwner()
	if err != nil {
		return err
	}

	c.l().Debugf("tikv-cdc resign owner successfully, and new owner found, owner: %+v", owner)
	return nil
}

// GetOwner return the TiKV-CDC owner capture information
func (c *TiKVCDCOpenAPIClient) GetOwner() (*TiKVCDCCapture, error) {
	captures, err := c.GetAllCaptures()
	if err != nil {
		return nil, err
	}

	for _, capture := range captures {
		if capture.IsOwner {
			return capture, nil
		}
	}
	return nil, fmt.Errorf("cannot found the tikv-cdc owner, query urls: %+v", c.urls)
}

// GetCaptureByAddr return the capture information by the address
func (c *TiKVCDCOpenAPIClient) GetCaptureByAddr(addr string) (*TiKVCDCCapture, error) {
	captures, err := c.GetAllCaptures()
	if err != nil {
		return nil, err
	}

	for _, capture := range captures {
		if capture.AdvertiseAddr == addr {
			return capture, nil
		}
	}

	return nil, fmt.Errorf("capture not found, addr: %s", addr)
}

// GetAllCaptures return all captures instantaneously
func (c *TiKVCDCOpenAPIClient) GetAllCaptures() (result []*TiKVCDCCapture, err error) {
	err = utils.Retry(func() error {
		result, err = c.queryAllCaptures()
		if err != nil {
			return err
		}
		return nil
	}, utils.RetryOption{
		Timeout: 10 * time.Second,
	})
	return result, err
}

func (c *TiKVCDCOpenAPIClient) queryAllCaptures() ([]*TiKVCDCCapture, error) {
	api := "api/v1/captures"
	endpoints := c.getEndpoints(api)

	var response []*TiKVCDCCapture

	_, err := tryURLs(endpoints, func(endpoint string) ([]byte, error) {
		body, statusCode, err := c.client.GetWithStatusCode(c.ctx, endpoint)
		if err != nil {
			if statusCode == http.StatusNotFound {
				// Ignore error, and return nil to trigger hard restart
				c.l().Debugf("get all captures failed, ignore it, err: %+v", err)
				return body, nil
			}
			return body, err
		}

		return body, json.Unmarshal(body, &response)
	})

	return response, err
}

// IsCaptureAlive return error if the capture is not alive
func (c *TiKVCDCOpenAPIClient) IsCaptureAlive() error {
	status, err := c.GetStatus()
	if err != nil {
		return err
	}
	if status.Liveness != TiKVCDCCaptureAlive {
		return fmt.Errorf("capture is not alive, request url: %+v", c.urls[0])
	}
	return nil
}

// GetStatus return the status of the TiKVCDC server.
func (c *TiKVCDCOpenAPIClient) GetStatus() (result TiKVCDCServerStatus, err error) {
	api := "api/v1/status"
	// client should only have address to the target TiKV-CDC server, not all.
	endpoints := c.getEndpoints(api)

	err = utils.Retry(func() error {
		data, statusCode, err := c.client.GetWithStatusCode(c.ctx, endpoints[0])
		if err != nil {
			if statusCode == http.StatusNotFound {
				c.l().Debugf("capture server status failed, ignore it, err: %+v", err)
				return nil
			}
			err = json.Unmarshal(data, &result)
			if err != nil {
				return err
			}
			if result.Liveness == TiKVCDCCaptureAlive {
				return nil
			}
			return errors.New("capture status is not alive, retry it")
		}
		return nil
	}, utils.RetryOption{
		Timeout: 10 * time.Second,
	})

	return result, err
}

func (c *TiKVCDCOpenAPIClient) l() *logprinter.Logger {
	return c.ctx.Value(logprinter.ContextKeyLogger).(*logprinter.Logger)
}

// TiKVCDCLiveness is the liveness status of a capture.
type TiKVCDCLiveness int32

const (
	// TiKVCDCCaptureAlive means the capture is alive, and ready to serve.
	TiKVCDCCaptureAlive TiKVCDCLiveness = 0
	// TiKVCDCCaptureStopping means the capture is in the process of graceful shutdown.
	TiKVCDCCaptureStopping TiKVCDCLiveness = 1
)

// TiKVCDCServerStatus holds some common information of a TiCDC server
type TiKVCDCServerStatus struct {
	Version  string          `json:"version"`
	GitHash  string          `json:"git_hash"`
	ID       string          `json:"id"`
	Pid      int             `json:"pid"`
	IsOwner  bool            `json:"is_owner"`
	Liveness TiKVCDCLiveness `json:"liveness"`
}

// TiKVCDCCapture holds common information of a capture in cdc
type TiKVCDCCapture struct {
	ID            string `json:"id"`
	IsOwner       bool   `json:"is_owner"`
	AdvertiseAddr string `json:"address"`
}
