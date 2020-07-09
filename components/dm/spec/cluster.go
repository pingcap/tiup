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

	cspec "github.com/pingcap/tiup/pkg/cluster/spec"
)

var specManager *cspec.SpecManager

// DMMeta is the specification of generic cluster metadata
type DMMeta struct {
	User    string `yaml:"user"`       // the user to run and manage cluster on remote
	Version string `yaml:"dm_version"` // the version of TiDB cluster
	//EnableTLS      bool   `yaml:"enable_tls"`
	//EnableFirewall bool   `yaml:"firewall"`

	Topology *DMTopologySpecification `yaml:"topology"`
}

var _ cspec.UpgradableMetadata = &DMMeta{}

// SetVersion implement UpgradableMetadata interface.
func (m *DMMeta) SetVersion(s string) {
	m.Version = s
}

// GetTopology implements Metadata interface.
func (m *DMMeta) GetTopology() cspec.Topology {
	return m.Topology
}

// SetTopology implements Metadata interface.
func (m *DMMeta) SetTopology(topo cspec.Topology) {
	dmTopo, ok := topo.(*DMTopologySpecification)
	if !ok {
		panic(fmt.Sprintln("wrong type: ", reflect.TypeOf(topo)))
	}

	m.Topology = dmTopo
}

// GetBaseMeta implements Metadata interface.
func (m *DMMeta) GetBaseMeta() *cspec.BaseMeta {
	return &cspec.BaseMeta{
		Version: m.Version,
		User:    m.User,
	}
}

// GetSpecManager return the spec manager of dm cluster.
func GetSpecManager() *cspec.SpecManager {
	if specManager == nil {
		specManager = cspec.NewSpec(filepath.Join(cspec.ProfileDir(), cspec.TiOpsClusterDir))
	}
	return specManager
}
