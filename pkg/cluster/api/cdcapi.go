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

	"github.com/pingcap/errors"
	logprinter "github.com/pingcap/tiup/pkg/logger/printer"
	"github.com/pingcap/tiup/pkg/utils"
)

// CDCOpenAPIClient is client for access TiCDC Open API
type CDCOpenAPIClient struct {
	urls   []string
	client *utils.HTTPClient
	ctx    context.Context
}

// NewCDCOpenAPIClient return a `CDCOpenAPIClient`
func NewCDCOpenAPIClient(ctx context.Context, addresses []string, timeout time.Duration, tlsConfig *tls.Config) *CDCOpenAPIClient {
	httpPrefix := "http"
	if tlsConfig != nil {
		httpPrefix = "https"
	}
	urls := make([]string, 0, len(addresses))
	for _, addr := range addresses {
		urls = append(urls, fmt.Sprintf("%s://%s", httpPrefix, addr))
	}

	return &CDCOpenAPIClient{
		urls:   urls,
		client: utils.NewHTTPClient(timeout, tlsConfig),
		ctx:    ctx,
	}
}

func (c *CDCOpenAPIClient) getEndpoints(api string) (endpoints []string) {
	for _, url := range c.urls {
		endpoints = append(endpoints, fmt.Sprintf("%s/%s", url, api))
	}
	return endpoints
}

func drainCapture(client *CDCOpenAPIClient, target string) (int, error) {
	api := "api/v1/captures/drain"
	endpoints := client.getEndpoints(api)

	request := DrainCaptureRequest{
		CaptureID: target,
	}
	body, err := json.Marshal(request)
	if err != nil {
		return 0, err
	}

	var resp DrainCaptureResp
	_, err = tryURLs(endpoints, func(endpoint string) ([]byte, error) {
		data, statusCode, err := client.client.Put(client.ctx, endpoint, bytes.NewReader(body))
		if err != nil {
			switch statusCode {
			case http.StatusNotFound:
				// old version cdc does not support `DrainCapture`, return nil to trigger hard restart.
				client.l().Debugf("cdc drain capture does not support, ignore it, target: %s, err: %+v", target, err)
				return data, nil
			case http.StatusServiceUnavailable:
				if bytes.Contains(data, []byte("CDC:ErrVersionIncompatible")) {
					client.l().Debugf("cdc drain capture meet version incompatible, ignore it, target: %s, err: %+v", target, err)
					return data, nil
				}
				// cdc is not ready to accept request, return error to trigger retry.
				client.l().Debugf("cdc drain capture meet service unavailable, retry it, target: %s, err: %+v", target, err)
				return data, err
			default:
			}
			// match https://github.com/pingcap/tiflow/blob/e3d0d9d23b77c7884b70016ddbd8030ffeb95dfd/pkg/errors/cdc_errors.go#L55-L57
			if bytes.Contains(data, []byte("CDC:ErrSchedulerRequestFailed")) {
				client.l().Debugf("cdc drain capture failed, data: %s, err: %+v", data, err)
				return data, nil
			}
			// match https://github.com/pingcap/tiflow/blob/e3d0d9d23b77c7884b70016ddbd8030ffeb95dfd/pkg/errors/cdc_errors.go#L51-L54
			if bytes.Contains(data, []byte("CDC:ErrCaptureNotExist")) {
				client.l().Debugf("cdc drain capture failed, data: %s, err: %+v", data, err)
				return data, nil
			}
			client.l().Debugf("cdc drain capture failed, data: %s, statusCode: %d, err: %+v", data, statusCode, err)
			return data, err
		}
		return data, json.Unmarshal(data, &resp)
	})
	return resp.CurrentTableCount, err
}

// DrainCapture request cdc owner move all tables on the target capture to other captures.
func (c *CDCOpenAPIClient) DrainCapture(addr, target string, apiTimeoutSeconds int) error {
	if _, err := c.getCaptureByID(target); err != nil {
		c.l().Debugf("cdc drain capture failed, cannot find the capture, address: %s, target: %s, err: %+v", addr, target, err)
		return err
	}
	c.l().Infof("\t Start drain the capture, address: %s, captureID: %s", addr, target)
	start := time.Now()
	err := utils.Retry(func() error {
		count, err := drainCapture(c, target)
		if err != nil {
			return err
		}
		if count == 0 {
			return nil
		}
		c.l().Infof("\t Still waiting for %d tables to transfer...", count)
		return fmt.Errorf("drain capture not finished yet, target: %s, count: %d", target, count)
	}, utils.RetryOption{
		Delay:   1 * time.Second,
		Timeout: time.Duration(apiTimeoutSeconds) * time.Second,
	})

	c.l().Debugf("cdc drain capture finished, target: %s, elapsed: %+v", target, time.Since(start))
	return err
}

// ResignOwner resign the cdc owner, and wait for a new owner be found
// address is the current owner's address
func (c *CDCOpenAPIClient) ResignOwner(address string) error {
	err := utils.Retry(func() error {
		return resignOwner(c, address)
	}, utils.RetryOption{
		Delay:   2 * time.Second,
		Timeout: 10 * time.Second,
	})
	return err
}

func resignOwner(c *CDCOpenAPIClient, addr string) error {
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

	if owner.AdvertiseAddr == addr {
		return fmt.Errorf("old owner in power again, resign again, owner: %+v", owner)
	}

	c.l().Debugf("cdc resign owner successfully, and new owner found, owner: %+v", owner)
	return nil
}

// GetOwner return the cdc owner capture information
func (c *CDCOpenAPIClient) GetOwner() (result *Capture, err error) {
	err = utils.Retry(func() error {
		captures, err := c.GetAllCaptures()
		if err != nil {
			return err
		}
		for _, capture := range captures {
			if capture.IsOwner {
				result = capture
				return nil
			}
		}
		return fmt.Errorf("no owner found")
	}, utils.RetryOption{
		Delay:   time.Second,
		Timeout: 10 * time.Second,
	})

	return result, err
}

func (c *CDCOpenAPIClient) getCaptureByID(id string) (*Capture, error) {
	var result *Capture
	err := utils.Retry(func() error {
		captures, err := c.GetAllCaptures()
		if err != nil {
			return err
		}
		for _, capture := range captures {
			if capture.ID == id {
				result = capture
				return nil
			}
		}
		return fmt.Errorf("target capture not found")
	}, utils.RetryOption{
		Delay:   time.Second,
		Timeout: 10 * time.Second,
	})
	return result, err
}

// GetCaptureByAddr return the capture information by the address
func (c *CDCOpenAPIClient) GetCaptureByAddr(addr string) (result *Capture, err error) {
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
func (c *CDCOpenAPIClient) GetAllCaptures() ([]*Capture, error) {
	api := "api/v1/captures"
	endpoints := c.getEndpoints(api)

	var result []*Capture
	_, err := tryURLs(endpoints, func(endpoint string) ([]byte, error) {
		body, statusCode, err := c.client.GetWithStatusCode(c.ctx, endpoint)
		if err != nil {
			if statusCode == http.StatusNotFound {
				// old version cdc does not support open api, also the stopped cdc instance
				// return nil to trigger hard restart
				c.l().Debugf("get all captures not support, ignore it, err: %+v", err)
				return body, nil
			}
			return body, err
		}
		return body, json.Unmarshal(body, &result)
	})

	return result, err
}

// IsCaptureAlive return error if the capture is not alive
func (c *CDCOpenAPIClient) IsCaptureAlive() error {
	status, err := c.GetStatus()
	if err != nil {
		return err
	}
	if status.Liveness != LivenessCaptureAlive {
		return fmt.Errorf("capture is not alive, request url: %+v", c.urls[0])
	}
	return nil
}

// GetStatus return the status of the TiCDC server.
func (c *CDCOpenAPIClient) GetStatus() (result ServerStatus, err error) {
	api := "api/v1/status"
	// client should only have address to the target cdc server, not all cdc servers.
	endpoints := c.getEndpoints(api)

	err = utils.Retry(func() error {
		data, statusCode, err := c.client.GetWithStatusCode(c.ctx, endpoints[0])
		if err != nil {
			if statusCode == http.StatusNotFound {
				c.l().Debugf("capture server status api not support, ignore it, err: %+v", err)
				return nil
			}
			err = json.Unmarshal(data, &result)
			if err != nil {
				return err
			}
			if result.Liveness == LivenessCaptureAlive {
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

// Healthy return true if the TiCDC cluster is healthy
func (c *CDCOpenAPIClient) Healthy() error {
	err := utils.Retry(func() error {
		return isHealthy(c)
	}, utils.RetryOption{
		Timeout: 10 * time.Second,
	})
	return err
}

func isHealthy(client *CDCOpenAPIClient) error {
	api := "api/v1/health"
	endpoints := client.getEndpoints(api)

	_, err := tryURLs(endpoints, func(endpoint string) ([]byte, error) {
		_, statusCode, err := client.client.GetWithStatusCode(client.ctx, endpoint)
		if err != nil {
			switch statusCode {
			// It's likely the TiCDC does not support the API, return error to trigger hard restart.
			case http.StatusNotFound:
				client.l().Debugf("cdc check healthy does not support, ignore it")
				return nil, nil
			case http.StatusInternalServerError:
				client.l().Debugf("cdc check healthy: internal server error, retry it, err: %+v", err)
			}
			return nil, err
		}
		return nil, nil
	})
	return err
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
