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
	"fmt"
	"time"

	"github.com/pingcap/tiup/pkg/utils"
)

// TiDBClient is client for access TiKVCDC Open API
type TiDBClient struct {
	urls   []string
	client *utils.HTTPClient
	ctx    context.Context
}

// NewTiDBClient return a `TiDBClient`
func NewTiDBClient(ctx context.Context, addresses []string, timeout time.Duration, tlsConfig *tls.Config) *TiDBClient {
	httpPrefix := "http"
	if tlsConfig != nil {
		httpPrefix = "https"
	}
	urls := make([]string, 0, len(addresses))
	for _, addr := range addresses {
		urls = append(urls, fmt.Sprintf("%s://%s", httpPrefix, addr))
	}

	return &TiDBClient{
		urls:   urls,
		client: utils.NewHTTPClient(timeout, tlsConfig),
		ctx:    ctx,
	}
}

func (c *TiDBClient) getEndpoints(api string) (endpoints []string) {
	for _, url := range c.urls {
		endpoints = append(endpoints, fmt.Sprintf("%s%s", url, api))
	}
	return endpoints
}

// StartUpgrade sends the start upgrade message to the TiDB server
func (c *TiDBClient) StartUpgrade() error {
	api := "/upgrade/start"
	endpoints := c.getEndpoints(api)
	_, err := tryURLs(endpoints, func(endpoint string) ([]byte, error) {
		return c.client.Post(c.ctx, endpoint, nil)
	})

	return err
}

// FinishUpgrade sends the finish upgrade message to the TiDB server
func (c *TiDBClient) FinishUpgrade() error {
	api := "/upgrade/finish"
	endpoints := c.getEndpoints(api)
	_, err := tryURLs(endpoints, func(endpoint string) ([]byte, error) {
		return c.client.Post(c.ctx, endpoint, nil)
	})

	return err
}
