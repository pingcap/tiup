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

package meta

import (
	"io/ioutil"
	"os"

	"github.com/joomcode/errorx"
	"github.com/pingcap/errors"
	"github.com/pingcap/tiup/pkg/cliutil"
	"github.com/pingcap/tiup/pkg/file"
	"github.com/pingcap/tiup/pkg/utils"
	"github.com/pingcap/tiup/pkg/version"
	"gopkg.in/yaml.v2"
)

const (
	// MetaFileName is the file name of the meta file.
	MetaFileName = "meta.yaml"
	// PatchDirName is the directory to store patch file eg. {PatchDirName}/tidb-hotfix.tar.gz
	PatchDirName = "patch"
	// BackupDirName is the directory to save backup files.
	BackupDirName = "backup"
)

var (
	errNSCluster = errNS.NewSubNamespace("cluster")
	// ErrClusterCreateDirFailed is ErrClusterCreateDirFailed
	ErrClusterCreateDirFailed = errNSCluster.NewType("create_dir_failed")
	// ErrClusterSaveMetaFailed is ErrClusterSaveMetaFailed
	ErrClusterSaveMetaFailed = errNSCluster.NewType("save_meta_failed")
)

// ClusterMeta is the specification of generic cluster metadata
type ClusterMeta struct {
	User    string `yaml:"user"`         // the user to run and manage cluster on remote
	Version string `yaml:"tidb_version"` // the version of TiDB cluster
	//EnableTLS      bool   `yaml:"enable_tls"`
	//EnableFirewall bool   `yaml:"firewall"`
	OpsVer string `yaml:"last_ops_ver,omitempty"` // the version of ourself that updated the meta last time

	Topology *ClusterSpecification `yaml:"topology"`
}

// EnsureClusterDir ensures that the cluster directory exists.
func EnsureClusterDir(clusterName string) error {
	if err := utils.CreateDir(ClusterPath(clusterName)); err != nil {
		return ErrClusterCreateDirFailed.
			Wrap(err, "Failed to create cluster metadata directory '%s'", ClusterPath(clusterName)).
			WithProperty(cliutil.SuggestionFromString("Please check file system permissions and try again."))
	}
	return nil
}

// SaveClusterMeta saves the cluster meta information to profile directory
func SaveClusterMeta(clusterName string, meta *ClusterMeta) error {
	wrapError := func(err error) *errorx.Error {
		return ErrClusterSaveMetaFailed.Wrap(err, "Failed to save cluster metadata")
	}

	metaFile := ClusterPath(clusterName, MetaFileName)
	backupDir := ClusterPath(clusterName, BackupDirName)

	// set the cmd version
	meta.OpsVer = version.NewTiUPVersion().String()

	if err := EnsureClusterDir(clusterName); err != nil {
		return wrapError(err)
	}

	if err := os.MkdirAll(backupDir, 0755); err != nil {
		return wrapError(err)
	}

	data, err := yaml.Marshal(meta)
	if err != nil {
		return wrapError(err)
	}

	err = file.SaveFileWithBackup(metaFile, data, backupDir)
	if err != nil {
		return wrapError(err)
	}

	return nil
}

// ClusterMetadata tries to read the metadata of a cluster from file
func ClusterMetadata(clusterName string) (*ClusterMeta, error) {
	var cm ClusterMeta
	topoFile := ClusterPath(clusterName, MetaFileName)

	yamlFile, err := ioutil.ReadFile(topoFile)
	if err != nil {
		return nil, errors.Trace(err)
	}

	if err = yaml.Unmarshal(yamlFile, &cm); err != nil {
		// return the meta no matter there is error or not
		return &cm, errors.Trace(err)
	}
	return &cm, nil
}
