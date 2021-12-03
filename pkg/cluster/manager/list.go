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

package manager

import (
	"encoding/json"
	"errors"
	"fmt"

	perrs "github.com/pingcap/errors"
	"github.com/pingcap/tiup/pkg/cluster/spec"
	logprinter "github.com/pingcap/tiup/pkg/logger/log"
	"github.com/pingcap/tiup/pkg/meta"
	"github.com/pingcap/tiup/pkg/tui"
)

// Cluster represents a clsuter
type Cluster struct {
	Name       string `json:"name"`
	User       string `json:"user"`
	Version    string `json:"version"`
	Path       string `json:"path"`
	PrivateKey string `json:"private_key"`
}

// ListCluster list the clusters.
func (m *Manager) ListCluster() error {
	clusters, err := m.GetClusterList()
	if err != nil {
		return err
	}

	switch m.logger.GetDisplayMode() {
	case logprinter.DisplayModeJSON:
		clusterObj := struct {
			Clusters []Cluster `json:"clusters"`
		}{
			Clusters: clusters,
		}
		data, err := json.Marshal(clusterObj)
		if err != nil {
			return err
		}
		fmt.Println(string(data))
	default:
		clusterTable := [][]string{
			// Header
			{"Name", "User", "Version", "Path", "PrivateKey"},
		}
		for _, v := range clusters {
			clusterTable = append(clusterTable, []string{
				v.Name,
				v.User,
				v.Version,
				v.Path,
				v.PrivateKey,
			})
		}
		tui.PrintTable(clusterTable, true)
	}
	return nil
}

// GetClusterList get the clusters list.
func (m *Manager) GetClusterList() ([]Cluster, error) {
	names, err := m.specManager.List()
	if err != nil {
		return nil, err
	}

	var clusters = []Cluster{}

	for _, name := range names {
		metadata, err := m.meta(name)
		if err != nil && !errors.Is(perrs.Cause(err), meta.ErrValidate) &&
			!errors.Is(perrs.Cause(err), spec.ErrNoTiSparkMaster) {
			return nil, perrs.Trace(err)
		}

		base := metadata.GetBaseMeta()

		clusters = append(clusters, Cluster{
			Name:       name,
			User:       base.User,
			Version:    base.Version,
			Path:       m.specManager.Path(name),
			PrivateKey: m.specManager.Path(name, "ssh", "id_rsa"),
		})
	}

	return clusters, nil
}
