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

package task

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"path/filepath"
	"time"

	dmspec "github.com/pingcap/tiup/components/dm/spec"
	"github.com/pingcap/tiup/pkg/cluster/spec"
	"github.com/pingcap/tiup/pkg/proxy"
	"github.com/pingcap/tiup/pkg/utils"
)

const clusterInfoURL = "/api/v1/cluster/info"

// UpdateDMTopology is used to maintain the cluster meta information
type UpdateDMTopology struct {
	cluster  string
	metadata *dmspec.Metadata
	tcpProxy *proxy.TCPProxy
}

type dmcomponent struct {
	Host string `json:"host"`
	Port int    `json:"port"`
	Name string `json:"name,omitempty"`
}

type clusterInfoRequest struct {
	/*
		DMMaster     []dmcomponent `json:"master_topology_list"`
		DMWorker     []dmcomponent `json:"worker_topology_list"`
	*/
	Grafana      *dmcomponent `json:"grafana_topology,omitempty"`
	Prometheus   *dmcomponent `json:"prometheus_topology,omitempty"`
	AlertManager *dmcomponent `json:"alert_manager_topology,omitempty"`
}

// NewUpdateDMTopology creates task whick send topo info to dm openapi
func NewUpdateDMTopology(cluster string, metadata *dmspec.Metadata) *UpdateDMTopology {
	return &UpdateDMTopology{
		metadata: metadata,
		cluster:  cluster,
		tcpProxy: proxy.GetTCPProxy(),
	}
}

// String implements the fmt.Stringer interface
func (u *UpdateDMTopology) String() string {
	return fmt.Sprintf("UpdateDMTopology: cluster=%s", u.cluster)
}

// Execute implements the Task interface
func (u *UpdateDMTopology) Execute(ctx context.Context) error {
	tlsCfg, err := u.metadata.Topology.TLSConfig(
		filepath.Join(dmspec.GetSpecManager().Path(u.cluster), spec.TLSCertKeyDir),
	)
	if err != nil {
		return err
	}

	var clusterInfo clusterInfoRequest
	topo := u.metadata.Topology

	/*
		for _, inst := range topo.Masters {
			clusterInfo.DMMaster = append(clusterInfo.DMMaster, dmcomponent{
				Host: inst.Host,
				Port: inst.Port,
				Name: inst.Name,
			})
		}
		for _, inst := range topo.Masters {
			clusterInfo.DMWorker = append(clusterInfo.DMWorker, dmcomponent{
				Host: inst.Host,
				Port: inst.Port,
				Name: inst.Name,
			})
		}
	*/
	if len(topo.Grafanas) > 0 {
		inst := topo.Grafanas[0]
		clusterInfo.Grafana = &dmcomponent{
			Host: inst.Host,
			Port: inst.Port,
		}
	}
	if len(topo.Monitors) > 0 {
		inst := topo.Monitors[0]
		clusterInfo.Prometheus = &dmcomponent{
			Host: inst.Host,
			Port: inst.Port,
		}
	}
	if len(topo.Alertmanagers) > 0 {
		inst := topo.Alertmanagers[0]
		clusterInfo.AlertManager = &dmcomponent{
			Host: inst.Host,
			Port: inst.WebPort,
		}
	}

	body, err := json.Marshal(clusterInfo)
	if err != nil {
		return err
	}

	c := utils.NewHTTPClient(5*time.Second, tlsCfg)
	for _, inst := range topo.Masters {
		url := utils.Ternary(tlsCfg == nil, "http://", "https://").(string) + fmt.Sprintf("%s:%d", inst.Host, inst.Port) + clusterInfoURL
		_, err = c.Put(ctx, url, bytes.NewReader(body))
		if err == nil {
			break
		}
	}

	return err
}

// Rollback implements the Task interface
func (u *UpdateDMTopology) Rollback(ctx context.Context) error {
	return nil
}
