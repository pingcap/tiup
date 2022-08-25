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

// environment variable names that used to interrupt operations
const (
	EnvNameSkipScaleInTopoCheck = "SKIP_SCALEIN_TOPO_CHECK"
	EnvNamePDEndpointOverwrite  = "FORCE_PD_ENDPOINTS"
)

// Options represents the operation options
type Options struct {
	Roles               []string
	Nodes               []string
	Force               bool             // Option for upgrade/tls subcommand
	SSHTimeout          uint64           // timeout in seconds when connecting an SSH server
	OptTimeout          uint64           // timeout in seconds for operations that support it, not to confuse with SSH timeout
	APITimeout          uint64           // timeout in seconds for API operations that support it, like transferring store leader
	IgnoreConfigCheck   bool             // should we ignore the config check result after init config
	NativeSSH           bool             // should use native ssh client or builtin easy ssh (deprecated, shoule use SSHType)
	SSHType             executor.SSHType // the ssh type: 'builtin', 'system', 'none'
	Concurrency         int              // max number of parallel tasks to run
	SSHProxyHost        string           // the ssh proxy host
	SSHProxyPort        int              // the ssh proxy port
	SSHProxyUser        string           // the ssh proxy user
	SSHProxyIdentity    string           // the ssh proxy identity file
	SSHProxyUsePassword bool             // use password instead of identity file for ssh proxy connection
	SSHProxyTimeout     uint64           // timeout in seconds when connecting the proxy host

	// What type of things should we cleanup in clean command
	CleanupData     bool // should we cleanup data
	CleanupLog      bool // should we clenaup log
	CleanupAuditLog bool // should we clenaup tidb server auit log

	// Some data will be retained when destroying instances
	RetainDataRoles []string
	RetainDataNodes []string

	// Show uptime or not
	ShowUptime bool

	DisplayMode string // the output format
	Operation   Operation
}

// Operation represents the type of cluster operation
type Operation byte

// Operation represents the kind of cluster operation
const (
	// StartOperation Operation = iota
	// StopOperation
	RestartOperation Operation = iota
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
		var role string
		switch c.Name() {
		case spec.ComponentTiSpark:
			role = c.Role()
		default:
			role = c.Name()
		}
		if !components.Exist(role) {
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
