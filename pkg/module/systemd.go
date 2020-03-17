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

package module

import (
	"fmt"
	"strings"

	"github.com/pingcap-incubator/tiops/pkg/executor"
)

const (
	// scope can be either "system", "user" or "global"
	SystemdScopeSystem = "system"
	SystemdScopeUser   = "user"
	SystemdScopeGlobal = "global"
)

// SystemdModuleConfig is the configurations used to initialize a SystemdModule
type SystemdModuleConfig struct {
	Unit         string // the name of systemd unit(s)
	Action       string // the action to perform with the unit
	Enabled      bool   // enable the unit or not
	ReloadDaemon bool   // run daemon-reload before other actions
	Scope        string // user, system or global
	Force        bool   // add the `--force` arg to systemctl command
}

// SystemdModule is the module used to control systemd units
type SystemdModule struct {
	cmd  string // the built command
	sudo bool   // does the command need to be run as root
}

// NewSystemdModule builds and returns a SystemdModule object base on
// given config.
func NewSystemdModule(config SystemdModuleConfig) *SystemdModule {
	systemctl := "/usr/bin/systemctl" // TODO: find binary in $PATH
	sudo := true

	if config.Force {
		systemctl = fmt.Sprintf("%s --force", systemctl)
	}

	switch config.Scope {
	case SystemdScopeUser:
		sudo = false // `--user` scope does not need root priviledge
		fallthrough
	case SystemdScopeGlobal:
		systemctl = fmt.Sprintf("%s --%s", config.Scope)
	}

	cmd := fmt.Sprintf("%s %s %s",
		systemctl, strings.ToLower(config.Action), config.Unit)

	if config.Enabled {
		cmd = fmt.Sprintf("%s && %s enable %s",
			cmd, systemctl, config.Unit)
	}

	if config.ReloadDaemon {
		cmd = fmt.Sprintf("%s daemon-reload && %s",
			systemctl, cmd)
	}

	return &SystemdModule{
		cmd:  cmd,
		sudo: sudo,
	}
}

// Execute passes the command to executor and returns its results, the executor
// should be already initialized.
func (mod *SystemdModule) Execute(exec executor.TiOpsExecutor) ([]byte, []byte, error) {
	return exec.Execute(mod.cmd, mod.sudo)
}
