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
	"io/ioutil"
	"os"

	meta2 "github.com/pingcap/tiup/pkg/cluster/spec"

	"github.com/joomcode/errorx"
	"github.com/pingcap/errors"
	"gopkg.in/yaml.v2"
)

// DMMeta is the specification of generic cluster metadata
type DMMeta struct {
	User    string `yaml:"user"`       // the user to run and manage cluster on remote
	Version string `yaml:"dm_version"` // the version of TiDB cluster
	//EnableTLS      bool   `yaml:"enable_tls"`
	//EnableFirewall bool   `yaml:"firewall"`

	Topology *Specification `yaml:"topology"`
}

// SaveDMMeta saves the cluster meta information to profile directory
func SaveDMMeta(clusterName string, meta *DMMeta) error {
	wrapError := func(err error) *errorx.Error {
		return meta2.ErrClusterSaveMetaFailed.Wrap(err, "Failed to save dm metadata")
	}

	metaFile := meta2.ClusterPath(clusterName, meta2.MetaFileName)

	if err := meta2.EnsureClusterDir(clusterName); err != nil {
		return wrapError(err)
	}

	f, err := os.OpenFile(metaFile, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, os.ModePerm)
	if err != nil {
		return wrapError(err)
	}
	defer f.Close()

	if err := yaml.NewEncoder(f).Encode(meta); err != nil {
		return wrapError(err)
	}
	return nil
}

// DMMetadata tries to read the metadata of a cluster from file
func DMMetadata(clusterName string) (*DMMeta, error) {
	var cm DMMeta
	topoFile := meta2.ClusterPath(clusterName, meta2.MetaFileName)

	yamlFile, err := ioutil.ReadFile(topoFile)
	if err != nil {
		return nil, errors.Trace(err)
	}

	if err = yaml.Unmarshal(yamlFile, &cm); err != nil {
		return nil, errors.Trace(err)
	}
	return &cm, nil
}
