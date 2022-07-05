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
	"time"

	"github.com/pingcap/tiflow/cdc/model"
	"github.com/pingcap/tiup/pkg/utils"
)

type CDCOpenAPIClient struct {
	addrs      []string
	tlsEnabled bool
	client     *utils.HTTPClient
	ctx        context.Context
}

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

func drainCapture(client *CDCOpenAPIClient, target string) (int, error) {
	api := "/api/v1/captures/drain"
	endpoints := client.getEndpoints(api)

	request := model.DrainCaptureRequest{
		CaptureID: target,
	}
	body, err := json.Marshal(request)
	if err != nil {
		return 0, err
	}

	var data []byte
	_, err = tryURLs(endpoints, func(endpoint string) ([]byte, error) {
		// todo: also handle each kind of errors returned from the cdc
		data, err = client.client.PUT(client.ctx, api, bytes.NewReader(body))
		if err != nil {
			return data, err
		}
		return data, nil
	})
	if err != nil {
		return 0, err
	}

	var resp model.DrainCaptureResp
	err = json.Unmarshal(data, &resp)
	if err != nil {
		return resp.CurrentTableCount, err
	}

	return resp.CurrentTableCount, nil
}

// DrainCapture request cdc owner move all tables on the target capture to other captures.
func (c *CDCOpenAPIClient) DrainCapture(target string) (result int, err error) {
	err = utils.Retry(func() error {
		result, err = drainCapture(c, target)
		if err != nil {
			return err
		}
		return nil
	}, utils.RetryOption{
		Delay:   500 * time.Millisecond,
		Timeout: 10 * time.Second,
	})

	return result, err
}

// ResignOwner resign the cdc owner, to make owner switch
func (c *CDCOpenAPIClient) ResignOwner() error {
	api := "api/v1/owner/resign"
	endpoints := c.getEndpoints(api)
	_, err := tryURLs(endpoints, func(endpoint string) ([]byte, error) {
		body, err := c.client.Post(c.ctx, endpoint, nil)
		if err != nil {
			return body, err
		}
		return body, nil
	})

	return err
}

// GetURL builds the client URL of DMClient
func (c *CDCOpenAPIClient) GetURL(addr string) string {
	httpPrefix := "http"
	if c.tlsEnabled {
		httpPrefix = "https"
	}
	return fmt.Sprintf("%s://%s", httpPrefix, addr)
}

func (c *CDCOpenAPIClient) getEndpoints(cmd string) (endpoints []string) {
	for _, addr := range c.addrs {
		endpoint := fmt.Sprintf("%s/%s", c.GetURL(addr), cmd)
		endpoints = append(endpoints, endpoint)
	}

	return endpoints
}

func (c *CDCOpenAPIClient) GetAllCaptures() (result []*model.Capture, err error) {
	err = utils.Retry(func() error {
		result, err = getAllCaptures(c)
		if err != nil {
			return err
		}
		return nil
	}, utils.RetryOption{
		Delay:   500 * time.Millisecond,
		Timeout: 10 * time.Second,
	})

	return result, err
}

func (c *CDCOpenAPIClient) GetStatus() error {
	api := "/api/v1/status"
	endpoints := c.getEndpoints(api)

	var response model.ServerStatus
	_, err := tryURLs(endpoints, func(endpoint string) ([]byte, error) {
		data, err := c.client.Get(c.ctx, endpoint)
		if err != nil {
			return data, err
		}

		err = json.Unmarshal(data, &response)
		if err != nil {
			return data, err
		}

		if response.Liveness != model.LivenessCaptureAlive {
			return data, fmt.Errorf("cdc is not alive")
		}
		return data, nil
	})
	if err != nil {
		return err
	}

	return nil
}

func getAllCaptures(client *CDCOpenAPIClient) ([]*model.Capture, error) {
	api := "/api/v1/captures"
	endpoints := client.getEndpoints(api)

	var response []*model.Capture

	_, err := tryURLs(endpoints, func(endpoint string) ([]byte, error) {
		data, err := client.client.Get(client.ctx, endpoint)
		if err != nil {
			return data, err
		}

		return data, json.Unmarshal(data, response)
	})

	if err != nil {
		return nil, err
	}
	return response, nil
}
