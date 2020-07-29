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

package spec

import (
	"fmt"
	"path/filepath"
	"reflect"

	"github.com/pingcap/errors"
	"github.com/pingcap/tiup/pkg/version"
)

var tidbSpec *SpecManager

// GetSpecManager return the spec manager of tidb cluster.
func GetSpecManager() *SpecManager {
	if !initialized {
		panic("must Initialize profile first")
	}
	return tidbSpec
}

// ClusterMeta is the specification of generic cluster metadata
type ClusterMeta struct {
	User    string `yaml:"user"`         // the user to run and manage cluster on remote
	Version string `yaml:"tidb_version"` // the version of TiDB cluster
	//EnableTLS      bool   `yaml:"enable_tls"`
	//EnableFirewall bool   `yaml:"firewall"`
	OpsVer string `yaml:"last_ops_ver,omitempty"` // the version of ourself that updated the meta last time

	Topology *Specification `yaml:"topology"`
}

var _ UpgradableMetadata = &ClusterMeta{}

// SetVersion implement UpgradableMetadata interface.
func (m *ClusterMeta) SetVersion(s string) {
	m.Version = s
}

// SetUser implement UpgradableMetadata interface.
func (m *ClusterMeta) SetUser(s string) {
	m.User = s
}

// GetTopology implement Metadata interface.
func (m *ClusterMeta) GetTopology() Topology {
	return m.Topology
}

// SetTopology implement Metadata interface.
func (m *ClusterMeta) SetTopology(topo Topology) {
	tidbTopo, ok := topo.(*Specification)
	if !ok {
		panic(fmt.Sprintln("wrong type: ", reflect.TypeOf(topo)))
	}

	m.Topology = tidbTopo
}

// GetBaseMeta implements Metadata interface.
func (m *ClusterMeta) GetBaseMeta() *BaseMeta {
	return &BaseMeta{
		Version: m.Version,
		User:    m.User,
		OpsVer:  &m.OpsVer,
	}
}

// AuditDir return the directory for saving audit log.
func AuditDir() string {
	return filepath.Join(profileDir, TiOpsAuditDir)
}

// SaveClusterMeta saves the cluster meta information to profile directory
func SaveClusterMeta(clusterName string, cmeta *ClusterMeta) error {
	// set the cmd version
	cmeta.OpsVer = version.NewTiUPVersion().String()
	return GetSpecManager().SaveMeta(clusterName, cmeta)
}

// ClusterMetadata tries to read the metadata of a cluster from file
func ClusterMetadata(clusterName string) (*ClusterMeta, error) {
	var cm ClusterMeta
	err := GetSpecManager().Metadata(clusterName, &cm)
	if err != nil {
		// Return the value of cm even on error, to make sure the caller can get the data
		// we read, if there's any.
		// This is necessary when, either by manual editing of meta.yaml file, by not fully
		// validated `edit-config`, or by some unexpected operations from a broken legacy
		// release, we could provide max possibility that operations like `display`, `scale`
		// and `destroy` are still (more or less) working, by ignoring certain errors.
		return &cm, errors.AddStack(err)
	}

	return &cm, nil
}
