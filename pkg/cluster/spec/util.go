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
	"path/filepath"

	"github.com/pingcap/errors"
	"github.com/pingcap/tiup/pkg/meta"
	"github.com/pingcap/tiup/pkg/version"
)

const (
	// PatchDirName is the directory to store patch file eg. {PatchDirName}/tidb-hotfix.tar.gz
	PatchDirName = "patch"
)

// ClusterMeta is the specification of generic cluster metadata
type ClusterMeta struct {
	User    string `yaml:"user"`         // the user to run and manage cluster on remote
	Version string `yaml:"tidb_version"` // the version of TiDB cluster
	//EnableTLS      bool   `yaml:"enable_tls"`
	//EnableFirewall bool   `yaml:"firewall"`
	OpsVer string `yaml:"last_ops_ver,omitempty"` // the version of ourself that updated the meta last time

	Topology *Specification `yaml:"topology"`
}

// NewTiDBSpec create a Spec for tidb cluster.
func NewTiDBSpec() *meta.Spec {
	clusterBaseDir := filepath.Join(profileDir, TiOpsClusterDir)
	clusterSpec := meta.NewSpec(clusterBaseDir)
	return clusterSpec
}

// SaveClusterMeta saves the cluster meta information to profile directory
func SaveClusterMeta(clusterName string, cmeta *ClusterMeta) error {
	// set the cmd version
	cmeta.OpsVer = version.NewTiUPVersion().String()
	return NewTiDBSpec().SaveClusterMeta(clusterName, cmeta)
}

// ClusterMetadata tries to read the metadata of a cluster from file
func ClusterMetadata(clusterName string) (*ClusterMeta, error) {
	var cm ClusterMeta
	err := NewTiDBSpec().ClusterMetadata(clusterName, &cm)
	if err != nil {
		return nil, errors.AddStack(err)
	}

	return &cm, nil
}
