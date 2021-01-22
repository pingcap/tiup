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

	"github.com/pingcap/tiup/pkg/cluster/ctxt"
)

// ShellModuleConfig is the configurations used to initialize a TiUPModuleSystemd
type ShellModuleConfig struct {
	Command  string // the command to run
	Sudo     bool   // whether use root privilege to run the command
	Chdir    string // change working directory before running the command
	UseShell bool   // whether use shell to invoke the command
}

// ShellModule is the module used to control systemd units
type ShellModule struct {
	cmd  string // the built command
	sudo bool
}

// NewShellModule builds and returns a ShellModule object base on given config.
func NewShellModule(config ShellModuleConfig) *ShellModule {
	cmd := config.Command

	if config.Chdir != "" {
		cmd = fmt.Sprintf("cd %s && %s",
			config.Chdir, cmd)
	}

	if config.UseShell {
		cmd = fmt.Sprintf("%s -c '%s'",
			defaultShell, cmd)
	}

	return &ShellModule{
		cmd:  cmd,
		sudo: config.Sudo,
	}
}

// Execute passes the command to executor and returns its results, the executor
// should be already initialized.
func (mod *ShellModule) Execute(ctx context.Context, exec ctxt.Executor) ([]byte, []byte, error) {
	return exec.Execute(ctx, mod.cmd, mod.sudo)
}
