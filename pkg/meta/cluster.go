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

	"github.com/pingcap-incubator/tiops/pkg/utils"
	"github.com/pingcap/errors"
	"gopkg.in/yaml.v2"
)

const (
	// MetaFileName is the file name of the meta file.
	MetaFileName = "meta.yaml"
)

// ClusterMeta is the specification of generic cluster metadata
type ClusterMeta struct {
	User    string `yaml:"user"`         // the user to run and manage cluster on remote
	Version string `yaml:"tidb_version"` // the version of TiDB cluster
	//EnableTLS      bool   `yaml:"enable_tls"`
	//EnableFirewall bool   `yaml:"firewall"`

	Topology *TopologySpecification `yaml:"topology"`
}

func SaveClusterMeta(clusterName string, meta *ClusterMeta) error {
	metaFile := ClusterPath(clusterName, MetaFileName)

	// Make sure the cluster path exists
	if err := utils.CreateDir(ClusterPath(clusterName)); err != nil {
		return errors.Trace(err)
	}

	f, err := os.OpenFile(metaFile, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, os.ModePerm)
	if err != nil {
		return errors.Trace(err)
	}
	defer f.Close()

	if err := yaml.NewEncoder(f).Encode(meta); err != nil {
		return errors.Trace(err)
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
		return nil, errors.Trace(err)
	}
	return &cm, nil
}

// ClusterTopology tries to read the topology of a cluster from file
func ClusterTopology(clusterName string) (*TopologySpecification, error) {
	meta, err := ClusterMetadata(clusterName)
	if err != nil {
		return nil, err
	}
	return meta.Topology, nil
}
