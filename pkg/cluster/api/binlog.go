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

package api

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/pingcap/errors"
	"github.com/pingcap/tiup/pkg/utils"
	clientv3 "go.etcd.io/etcd/client/v3"
)

// BinlogClient is the client of binlog.
type BinlogClient struct {
	tls        *tls.Config
	httpClient *http.Client
	etcdClient *clientv3.Client
}

// NewBinlogClient create a BinlogClient.
func NewBinlogClient(pdEndpoints []string, timeout time.Duration, tlsConfig *tls.Config) (*BinlogClient, error) {
	if timeout < time.Second {
		timeout = time.Second * 5
	}

	etcdClient, err := clientv3.New(clientv3.Config{
		Endpoints:   pdEndpoints,
		DialTimeout: timeout,
		TLS:         tlsConfig,
	})
	if err != nil {
		return nil, errors.AddStack(err)
	}

	return &BinlogClient{
		tls:        tlsConfig,
		httpClient: utils.NewHTTPClient(timeout, tlsConfig).Client(),
		etcdClient: etcdClient,
	}, nil
}

func (c *BinlogClient) getURL(addr string) string {
	scheme := "http"
	if c.tls != nil {
		scheme = "https"
	}

	return fmt.Sprintf("%s://%s", scheme, addr)
}

func (c *BinlogClient) getOfflineURL(addr string, nodeID string) string {
	return fmt.Sprintf("%s/state/%s/close", c.getURL(addr), nodeID)
}

// StatusResp represents the response of status api.
type StatusResp struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

// NodeStatus represents the status saved in etcd.
type NodeStatus struct {
	NodeID      string `json:"nodeId"`
	Addr        string `json:"host"`
	State       string `json:"state"`
	MaxCommitTS int64  `json:"maxCommitTS"`
	UpdateTS    int64  `json:"updateTS"`
}

// IsPumpTombstone check if drainer is tombstone.
func (c *BinlogClient) IsPumpTombstone(ctx context.Context, addr string) (bool, error) {
	nodeID, err := c.nodeID(ctx, addr, "pumps")
	if err != nil {
		return false, err
	}
	return c.isTombstone(ctx, "pumps", nodeID)
}

// IsDrainerTombstone check if drainer is tombstone.
func (c *BinlogClient) IsDrainerTombstone(ctx context.Context, addr string) (bool, error) {
	nodeID, err := c.nodeID(ctx, addr, "drainers")
	if err != nil {
		return false, err
	}
	return c.isTombstone(ctx, "drainers", nodeID)
}

func (c *BinlogClient) isTombstone(ctx context.Context, ty string, nodeID string) (bool, error) {
	s, err := c.nodeStatus(ctx, ty, nodeID)
	if err != nil {
		return false, err
	}

	if s.State == "offline" {
		return true, nil
	}
	return false, nil
}

// nolint (unused)
func (c *BinlogClient) pumpNodeStatus(ctx context.Context) (status []*NodeStatus, err error) {
	return c.nodesStatus(ctx, "pumps")
}

// nolint (unused)
func (c *BinlogClient) drainerNodeStatus(ctx context.Context) (status []*NodeStatus, err error) {
	return c.nodesStatus(ctx, "drainers")
}

func (c *BinlogClient) nodeID(ctx context.Context, addr, ty string) (string, error) {
	// the number of nodes with the same ip:port
	targetNodes := []string{}

	nodes, err := c.nodesStatus(ctx, ty)
	if err != nil {
		return "", err
	}

	addrs := []string{}
	for _, node := range nodes {
		if addr == node.Addr {
			targetNodes = append(targetNodes, node.NodeID)
			continue
		}
		addrs = append(addrs, addr)
	}

	switch len(targetNodes) {
	case 0:
		return "", errors.Errorf("%s node id for address %s not found, found address: %s", ty, addr, addrs)
	case 1:
		return targetNodes[0], nil
	default:
		return "", errors.Errorf("found multiple %s nodes with the same host, found nodes: %s", ty, strings.Join(targetNodes, ","))
	}
}

// UpdateDrainerState update the specify state as the specified state.
func (c *BinlogClient) UpdateDrainerState(ctx context.Context, addr string, state string) error {
	nodeID, err := c.nodeID(ctx, addr, "drainers")
	if err != nil {
		return err
	}
	return c.updateStatus(ctx, "drainers", nodeID, state)
}

// UpdatePumpState update the specify state as the specified state.
func (c *BinlogClient) UpdatePumpState(ctx context.Context, addr string, state string) error {
	nodeID, err := c.nodeID(ctx, addr, "pumps")
	if err != nil {
		return err
	}
	return c.updateStatus(ctx, "pumps", nodeID, state)
}

// updateStatus update the specify state as the specified state.
func (c *BinlogClient) updateStatus(ctx context.Context, ty string, nodeID string, state string) error {
	ctx, f := context.WithTimeout(ctx, c.httpClient.Timeout)
	defer f()
	s, err := c.nodeStatus(ctx, ty, nodeID)
	if err != nil {
		return errors.AddStack(err)
	}

	if s.State == state {
		return nil
	}

	s.State = state

	data, err := json.Marshal(&s)
	if err != nil {
		return errors.AddStack(err)
	}

	key := fmt.Sprintf("/tidb-binlog/v1/%s/%s", ty, nodeID)
	_, err = c.etcdClient.Put(ctx, key, string(data))
	if err != nil {
		return errors.AddStack(err)
	}

	return nil
}

func (c *BinlogClient) nodesStatus(ctx context.Context, ty string) (status []*NodeStatus, err error) {
	key := fmt.Sprintf("/tidb-binlog/v1/%s", ty)

	// set timeout, otherwise it will keep retrying
	ctx, f := context.WithTimeout(ctx, c.httpClient.Timeout)
	defer f()
	resp, err := c.etcdClient.KV.Get(ctx, key, clientv3.WithPrefix())
	if err != nil {
		return nil, errors.AddStack(err)
	}

	for _, kv := range resp.Kvs {
		var s NodeStatus
		err = json.Unmarshal(kv.Value, &s)
		if err != nil {
			return nil, errors.Annotatef(err, "key: %s,data: %s", string(kv.Key), string(kv.Value))
		}

		status = append(status, &s)
	}

	return
}

// nodeStatus get nodeStatus with nodeID
func (c *BinlogClient) nodeStatus(ctx context.Context, ty string, nodeID string) (node *NodeStatus, err error) {
	key := fmt.Sprintf("/tidb-binlog/v1/%s/%s", ty, nodeID)

	resp, err := c.etcdClient.KV.Get(ctx, key)
	if err != nil {
		return nil, errors.AddStack(err)
	}

	if len(resp.Kvs) > 0 {
		err = json.Unmarshal(resp.Kvs[0].Value, &node)
		if err != nil {
			return nil, errors.Annotatef(err, "key: %s,data: %s", string(resp.Kvs[0].Key), string(resp.Kvs[0].Value))
		}
		return
	}
	return nil, errors.Errorf("%s node-id: %s not found, found address: %s", ty, nodeID, key)
}

func (c *BinlogClient) offline(addr string, nodeID string) error {
	url := c.getOfflineURL(addr, nodeID)
	req, err := http.NewRequest("PUT", url, nil)
	if err != nil {
		return errors.AddStack(err)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return errors.AddStack(err)
	}

	if resp.StatusCode < 200 || resp.StatusCode >= 400 {
		return errors.Errorf("error requesting %s, code: %d",
			resp.Request.URL, resp.StatusCode)
	}

	defer resp.Body.Close()
	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return errors.AddStack(err)
	}

	var status StatusResp
	err = json.Unmarshal(data, &status)
	if err != nil {
		return errors.Annotatef(err, "data: %s", string(data))
	}

	if status.Code != 200 {
		return errors.Errorf("server error: %s", status.Message)
	}

	return nil
}

// OfflinePump offline a pump.
func (c *BinlogClient) OfflinePump(ctx context.Context, addr string) error {
	nodeID, err := c.nodeID(ctx, addr, "pumps")
	if err != nil {
		return err
	}

	s, err := c.nodeStatus(ctx, "pumps", nodeID)
	if err != nil {
		return err
	}

	if s.State == "offline" {
		return nil
	}

	return c.offline(addr, nodeID)
}

// OfflineDrainer offline a drainer.
func (c *BinlogClient) OfflineDrainer(ctx context.Context, addr string) error {
	nodeID, err := c.nodeID(ctx, addr, "drainers")
	if err != nil {
		return err
	}

	s, err := c.nodeStatus(ctx, "drainers", nodeID)
	if err != nil {
		return err
	}

	if s.State == "offline" {
		return nil
	}

	return c.offline(addr, nodeID)
}
