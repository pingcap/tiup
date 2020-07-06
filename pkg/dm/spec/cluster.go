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

	cspec "github.com/pingcap/tiup/pkg/cluster/spec"
	"github.com/pingcap/tiup/pkg/meta"
)

const dmDir = "dm"

// DMMeta is the specification of generic cluster metadata
type DMMeta struct {
	User    string `yaml:"user"`       // the user to run and manage cluster on remote
	Version string `yaml:"dm_version"` // the version of TiDB cluster
	//EnableTLS      bool   `yaml:"enable_tls"`
	//EnableFirewall bool   `yaml:"firewall"`

	Topology *DMTopologySpecification `yaml:"topology"`
}

// NewDMSpec create dm Spec.
func NewDMSpec() *meta.SpecManager {
	return meta.NewSpec(filepath.Join(cspec.ProfileDir(), dmDir))
}
