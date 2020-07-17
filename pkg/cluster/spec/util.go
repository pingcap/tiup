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
	"github.com/pingcap/errors"
	"github.com/pingcap/tiup/pkg/meta"
	"github.com/pingcap/tiup/pkg/version"
)

const (
	// PatchDirName is the directory to store patch file eg. {PatchDirName}/tidb-hotfix.tar.gz
	PatchDirName = "patch"
)

var tidbSpec *meta.SpecManager

// GetSpecManager return the spec manager of tidb cluster.
func GetSpecManager() *meta.SpecManager {
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
