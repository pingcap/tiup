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
	"io/ioutil"
	"net/http"
	"time"

	"github.com/pingcap/errors"
	"go.etcd.io/etcd/clientv3"
)

// BinlogClient is the client of binlog.
type BinlogClient struct {
	tls        *tls.Config
	httpClient *http.Client
	etcdClient *clientv3.Client
}

// NewBinlogClient create a BinlogClient.
func NewBinlogClient(pdEndpoint []string, tlsConfig *tls.Config) (*BinlogClient, error) {
	etcdClient, err := clientv3.New(clientv3.Config{
		Endpoints:   pdEndpoint,
		DialTimeout: time.Second * 5,
		TLS:         tlsConfig,
	})
	if err != nil {
		return nil, errors.AddStack(err)
	}

	return &BinlogClient{
		tls: tlsConfig,
		httpClient: &http.Client{
			Transport: &http.Transport{
				TLSClientConfig: tlsConfig,
			},
		},
		etcdClient: etcdClient,
	}, nil
}

func (c *BinlogClient) getURL(addr string) string {
	schema := "http"
	if c.tls != nil {
		schema = "https"
	}

	return fmt.Sprintf("%s://%s", schema, addr)
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
func (c *BinlogClient) IsPumpTombstone(addr string) (bool, error) {
	nodeID, err := c.nodeID(addr, "pumps")
	if err != nil {
		return false, err
	}
	return c.isTombstone("pumps", nodeID)
}

// IsDrainerTombstone check if drainer is tombstone.
func (c *BinlogClient) IsDrainerTombstone(addr string) (bool, error) {
	nodeID, err := c.nodeID(addr, "drainers")
	if err != nil {
		return false, err
	}
	return c.isTombstone("drainer", nodeID)
}

func (c *BinlogClient) isTombstone(ty string, nodeID string) (bool, error) {
	status, err := c.nodeStatus(ty)
	if err != nil {
		return false, err
	}

	for _, s := range status {
		if s.NodeID == nodeID {
			if s.State == "offline" {
				return true, nil
			}
			return false, nil
		}
	}

	return false, errors.Errorf("node not exist: %s", nodeID)
}

// nolint (unused)
func (c *BinlogClient) pumpNodeStatus() (status []*NodeStatus, err error) {
	return c.nodeStatus("pumps")
}

// nolint (unused)
func (c *BinlogClient) drainerNodeStatus() (status []*NodeStatus, err error) {
	return c.nodeStatus("drainers")
}

func (c *BinlogClient) nodeID(addr, ty string) (string, error) {
	nodes, err := c.nodeStatus(ty)
	if err != nil {
		return "", err
	}

	addrs := []string{}
	for _, node := range nodes {
		if addr == node.Addr {
			return node.NodeID, nil
		}
		addrs = append(addrs, addr)
	}

	return "", errors.Errorf("%s node id for address %s not found, found address: %s", ty, addr, addrs)
}

// UpdateDrainerState update the specify state as the specified state.
func (c *BinlogClient) UpdateDrainerState(addr string, state string) error {
	nodeID, err := c.nodeID(addr, "drainers")
	if err != nil {
		return err
	}
	return c.updateStatus("drainers", nodeID, state)
}

// UpdatePumpState update the specify state as the specified state.
func (c *BinlogClient) UpdatePumpState(addr string, state string) error {
	nodeID, err := c.nodeID(addr, "pumps")
	if err != nil {
		return err
	}
	return c.updateStatus("pumps", nodeID, state)
}

// updateStatus update the specify state as the specified state.
func (c *BinlogClient) updateStatus(ty string, nodeID string, state string) error {
	key := fmt.Sprintf("/tidb-binlog/v1/%s/%s", ty, nodeID)

	ctx := context.Background()
	resp, err := c.etcdClient.KV.Get(ctx, key)
	if err != nil {
		return errors.AddStack(err)
	}

	if len(resp.Kvs) == 0 {
		return errors.Errorf("no pump with node id: %v", nodeID)
	}

	var nodeStatus NodeStatus
	err = json.Unmarshal(resp.Kvs[0].Value, &nodeStatus)
	if err != nil {
		return errors.AddStack(err)
	}

	if nodeStatus.State == state {
		return nil
	}

	nodeStatus.State = state

	data, err := json.Marshal(&nodeStatus)
	if err != nil {
		return errors.AddStack(err)
	}

	_, err = c.etcdClient.Put(ctx, key, string(data))
	if err != nil {
		return errors.AddStack(err)
	}

	return nil
}

func (c *BinlogClient) nodeStatus(ty string) (status []*NodeStatus, err error) {
	key := fmt.Sprintf("/tidb-binlog/v1/%s", ty)

	resp, err := c.etcdClient.KV.Get(context.Background(), key, clientv3.WithPrefix())
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
	data, err := ioutil.ReadAll(resp.Body)
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
func (c *BinlogClient) OfflinePump(addr string) error {
	nodeID, err := c.nodeID(addr, "pumps")
	if err != nil {
		return err
	}
	return c.offline(addr, nodeID)
}

// OfflineDrainer offline a drainer.
func (c *BinlogClient) OfflineDrainer(addr string) error {
	nodeID, err := c.nodeID(addr, "drainers")
	if err != nil {
		return err
	}
	return c.offline(addr, nodeID)
}
