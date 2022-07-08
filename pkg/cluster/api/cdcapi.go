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
	"bytes"
	"context"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	logprinter "github.com/pingcap/tiup/pkg/logger/printer"
	"github.com/pingcap/tiup/pkg/utils"
)

// CDCOpenAPIClient is client for access TiCDC Open API
type CDCOpenAPIClient struct {
	addrs      []string
	tlsEnabled bool
	client     *utils.HTTPClient
	ctx        context.Context
}

// NewCDCOpenAPIClient return a `CDCOpenAPIClient`
func NewCDCOpenAPIClient(ctx context.Context, addrs []string, timeout time.Duration, tlsConfig *tls.Config) *CDCOpenAPIClient {
	enableTLS := false
	if tlsConfig != nil {
		enableTLS = true
	}

	return &CDCOpenAPIClient{
		addrs:      addrs,
		tlsEnabled: enableTLS,
		client:     utils.NewHTTPClient(timeout, tlsConfig),
		ctx:        ctx,
	}
}

// DrainCapture request cdc owner move all tables on the target capture to other captures.
func (c *CDCOpenAPIClient) DrainCapture(target string) (result int, err error) {
	api := "api/v1/captures/drain"
	endpoints := c.getEndpoints(api)

	request := DrainCaptureRequest{
		CaptureID: target,
	}
	body, err := json.Marshal(request)
	if err != nil {
		return 0, err
	}

	var data []byte
	_, err = tryURLs(endpoints, func(endpoint string) ([]byte, error) {
		data, statusCode, err := c.client.Put(c.ctx, endpoint, bytes.NewReader(body))
		if err != nil {
			if statusCode == http.StatusNotFound {
				// old version cdc does not support `DrainCapture`, return nil to trigger hard restart.
				c.l().Debugf("cdc drain capture does not support, ignore it, target: %s, err: %s", target, err)
				return data, nil
			}

			if bytes.Contains(body, []byte("scheduler request failed")) {
				c.l().Debugf("cdc drain capture failed: %s", body)
				return data, nil
			}
			if bytes.Contains(body, []byte("capture not exists")) {
				c.l().Debugf("cdc drain capture failed: %s", body)
				return data, nil
			}
			return data, err
		}
		return data, nil
	})
	if err != nil {
		return 0, err
	}

	var resp DrainCaptureResp
	err = json.Unmarshal(data, &resp)
	if err != nil {
		return resp.CurrentTableCount, err
	}

	c.l().Infof("cdc drain capture finished, target=%+v, current_table_count=%+v, err=%+v", target, result, err)

	return resp.CurrentTableCount, nil
}

// ResignOwner resign the cdc owner, to make owner switch
func (c *CDCOpenAPIClient) ResignOwner() error {
	api := "api/v1/owner/resign"
	endpoints := c.getEndpoints(api)
	_, err := tryURLs(endpoints, func(endpoint string) ([]byte, error) {
		body, statusCode, err := c.client.PostWithStatusCode(c.ctx, endpoint, nil)
		if err != nil {
			if statusCode == http.StatusNotFound {
				c.l().Debugf("resign owner does not found, ignore: %s, err: %s", body, err)
				return body, nil
			}

			c.l().Warnf("cdc resign owner failed: %v", err)
			return body, err
		}
		c.l().Infof("cdc resign owner successfully")
		return body, nil
	})

	return err
}

func (c *CDCOpenAPIClient) getURL(addr string) string {
	httpPrefix := "http"
	if c.tlsEnabled {
		httpPrefix = "https"
	}
	return fmt.Sprintf("%s://%s", httpPrefix, addr)
}

func (c *CDCOpenAPIClient) getEndpoints(cmd string) (endpoints []string) {
	for _, addr := range c.addrs {
		endpoint := fmt.Sprintf("%s/%s", c.getURL(addr), cmd)
		endpoints = append(endpoints, endpoint)
	}

	return endpoints
}

// GetAllCaptures return all captures instantaneously
func (c *CDCOpenAPIClient) GetAllCaptures() (result []*Capture, err error) {
	err = utils.Retry(func() error {
		result, err = getAllCaptures(c)
		if err != nil {
			return err
		}
		return nil
	}, utils.RetryOption{
		Delay:   500 * time.Millisecond,
		Timeout: 20 * time.Second,
	})

	return result, err
}

// GetStatus return the status of the TiCDC server.
func (c *CDCOpenAPIClient) GetStatus() (result Liveness, err error) {
	err = utils.Retry(func() error {
		result, err = getCDCServerStatus(c)
		if err != nil {
			return err
		}
		return nil
	}, utils.RetryOption{
		Delay:   100 * time.Millisecond,
		Timeout: 20 * time.Second,
	})

	if err != nil {
		c.l().Warnf("cdc get capture status failed, %s", err)
	}

	return result, err
}

func getCDCServerStatus(client *CDCOpenAPIClient) (Liveness, error) {
	api := "api/v1/status"
	endpoints := client.getEndpoints(api)

	var response ServerStatus
	data, err := client.client.Get(client.ctx, endpoints[0])
	if err != nil {
		return response.Liveness, err
	}

	err = json.Unmarshal(data, &response)
	if err != nil {
		return response.Liveness, err
	}

	return response.Liveness, nil
}

func getAllCaptures(client *CDCOpenAPIClient) ([]*Capture, error) {
	api := "api/v1/captures"
	endpoints := client.getEndpoints(api)

	var response []*Capture

	_, err := tryURLs(endpoints, func(endpoint string) ([]byte, error) {
		body, statusCode, err := client.client.GetWithStatusCode(client.ctx, endpoint)
		if err != nil {
			if statusCode == http.StatusNotFound {
				// old version cdc does not support open api, also the stopped cdc instance
				// return nil to trigger hard restart
				client.l().Warnf("get all captures not exist, ignore: %s, statusCode: %+v, err: %s", body, statusCode, err)
				return body, nil
			}
			return body, err
		}

		return body, json.Unmarshal(body, &response)
	})

	if err != nil {
		return nil, err
	}
	return response, nil
}

func (c *CDCOpenAPIClient) l() *logprinter.Logger {
	return c.ctx.Value(logprinter.ContextKeyLogger).(*logprinter.Logger)
}

// Liveness is the liveness status of a capture.
type Liveness int32

const (
	// LivenessCaptureAlive means the capture is alive, and ready to serve.
	LivenessCaptureAlive Liveness = 0
	// LivenessCaptureStopping means the capture is in the process of graceful shutdown.
	LivenessCaptureStopping Liveness = 1
)

// ServerStatus holds some common information of a TiCDC server
type ServerStatus struct {
	Version  string   `json:"version"`
	GitHash  string   `json:"git_hash"`
	ID       string   `json:"id"`
	Pid      int      `json:"pid"`
	IsOwner  bool     `json:"is_owner"`
	Liveness Liveness `json:"liveness"`
}

// Capture holds common information of a capture in cdc
type Capture struct {
	ID            string `json:"id"`
	IsOwner       bool   `json:"is_owner"`
	AdvertiseAddr string `json:"address"`
}

// DrainCaptureRequest is request for manual `DrainCapture`
type DrainCaptureRequest struct {
	CaptureID string `json:"capture_id"`
}

// DrainCaptureResp is response for manual `DrainCapture`
type DrainCaptureResp struct {
	CurrentTableCount int `json:"current_table_count"`
}
