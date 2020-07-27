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

package operator

import (
	"fmt"

	"github.com/pingcap/tiup/pkg/cluster/executor"
	"github.com/pingcap/tiup/pkg/cluster/spec"
	"github.com/pingcap/tiup/pkg/set"
)

// Options represents the operation options
type Options struct {
	Roles             []string
	Nodes             []string
	Force             bool  // Option for upgrade subcommand
	SSHTimeout        int64 // timeout in seconds when connecting an SSH server
	OptTimeout        int64 // timeout in seconds for operations that support it, not to confuse with SSH timeout
	APITimeout        int64 // timeout in seconds for API operations that support it, like transfering store leader
	IgnoreConfigCheck bool  // should we ignore the config check result after init config
	NativeSSH         bool  // should use native ssh client or builtin easy ssh

	// Some data will be retained when destroying instances
	RetainDataRoles []string
	RetainDataNodes []string
}

// Operation represents the type of cluster operation
type Operation byte

// Operation represents the kind of cluster operation
const (
	StartOperation Operation = iota
	StopOperation
	RestartOperation
	DestroyOperation
	UpgradeOperation
	ScaleInOperation
	ScaleOutOperation
	DestroyTombstoneOperation
)

var opStringify = [...]string{
	"StartOperation",
	"StopOperation",
	"RestartOperation",
	"DestroyOperation",
	"UpgradeOperation",
	"ScaleInOperation",
	"ScaleOutOperation",
	"DestroyTombstoneOperation",
}

func (op Operation) String() string {
	if op <= DestroyTombstoneOperation {
		return opStringify[op]
	}
	return fmt.Sprintf("unknonw-op(%d)", op)
}

// FilterComponent filter components by set
func FilterComponent(comps []spec.Component, components set.StringSet) (res []spec.Component) {
	if len(components) == 0 {
		res = comps
		return
	}

	for _, c := range comps {
		if !components.Exist(c.Name()) {
			continue
		}

		res = append(res, c)
	}

	return
}

// FilterInstance filter instances by set
func FilterInstance(instances []spec.Instance, nodes set.StringSet) (res []spec.Instance) {
	if len(nodes) == 0 {
		res = instances
		return
	}

	for _, c := range instances {
		if !nodes.Exist(c.ID()) {
			continue
		}
		res = append(res, c)
	}

	return
}

// ExecutorGetter get the executor by host.
type ExecutorGetter interface {
	Get(host string) (e executor.Executor)
}
