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
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/pingcap/tiup/pkg/cluster/ctxt"
)

// scope can be either "system", "user" or "global"
const (
	SystemdScopeSystem = "system"
	SystemdScopeUser   = "user"
	SystemdScopeGlobal = "global"
)

// SystemdModuleConfig is the configurations used to initialize a SystemdModule
type SystemdModuleConfig struct {
	Unit         string        // the name of systemd unit(s)
	Action       string        // the action to perform with the unit
	ReloadDaemon bool          // run daemon-reload before other actions
	CheckActive  bool          // run is-active before action
	Scope        string        // user, system or global
	Force        bool          // add the `--force` arg to systemctl command
	Signal       string        // specify the signal to send to process
	Timeout      time.Duration // timeout to execute the command
}

// SystemdModule is the module used to control systemd units
type SystemdModule struct {
	cmd     string        // the built command
	sudo    bool          // does the command need to be run as root
	timeout time.Duration // timeout to execute the command
}

// NewSystemdModule builds and returns a SystemdModule object base on
// given config.
func NewSystemdModule(config SystemdModuleConfig) *SystemdModule {
	systemctl := "systemctl"
	sudo := true

	if config.Force {
		systemctl = fmt.Sprintf("%s --force", systemctl)
	}

	if config.Signal != "" {
		systemctl = fmt.Sprintf("%s --signal %s", systemctl, config.Signal)
	}

	switch config.Scope {
	case SystemdScopeUser:
		sudo = false // `--user` scope does not need root privilege
		fallthrough
	case SystemdScopeGlobal:
		systemctl = fmt.Sprintf("%s --%s", systemctl, config.Scope)
	}

	cmd := fmt.Sprintf("%s %s %s",
		systemctl, strings.ToLower(config.Action), config.Unit)

	if config.CheckActive {
		cmd = fmt.Sprintf("if [[ $(%s is-active %s) == \"active\" ]]; then %s; fi",
			systemctl, config.Unit, cmd)
	}

	if config.ReloadDaemon {
		cmd = fmt.Sprintf("%s daemon-reload && %s",
			systemctl, cmd)
	}
	mod := &SystemdModule{
		cmd:     cmd,
		sudo:    sudo,
		timeout: config.Timeout,
	}

	// the default TimeoutStopSec of systemd is 90s, after which it sends a SIGKILL
	// to remaining processes, set the default value slightly larger than it
	if config.Timeout == 0 {
		mod.timeout = time.Second * 100
	}

	return mod
}

// Execute passes the command to executor and returns its results, the executor
// should be already initialized.
func (mod *SystemdModule) Execute(ctx context.Context, exec ctxt.Executor) ([]byte, []byte, error) {
	return exec.Execute(ctx, mod.cmd, mod.sudo, mod.timeout)
}
