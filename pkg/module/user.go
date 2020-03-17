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

	"github.com/pingcap-incubator/tiops/pkg/executor"
)

const (
	defaultShell = "/bin/bash"

	UserActionAdd = "add"
	UserActionDel = "del"
	//UserActionModify = "modify"

	// TODO: in RHEL/CentOS, the commands are in /usr/sbin, but in some
	// other distros they may be in other location such as /usr/bin, we'll
	// need to check and find the proper path of commands in the future.
	useraddCmd = "/usr/sbin/useradd"
	userdelCmd = "/usr/sbin/userdel"
	//usermodCmd = "/usr/sbin/usermod"
)

// UserModuleConfig is the configurations used to initialize a UserModule
type UserModuleConfig struct {
	Action string // add, del or modify user
	Name   string // username
	Home   string // home directory of user
	Shell  string // login shell of the user
	Sudoer bool   // when true, the user will be added to sudoers list
}

// UserModule is the module used to control systemd units
type UserModule struct {
	cmd string // the built command
}

// NewUserModule builds and returns a UserModule object base on given config.
func NewUserModule(config UserModuleConfig) *UserModule {
	cmd := ""

	switch config.Action {
	case UserActionAdd:
		cmd = useraddCmd
		// set user's home
		if config.Home != "" {
			cmd = fmt.Sprintf("/usr/bin/mkdir -p %s && %s -d %s",
				config.Home, cmd, config.Home)
		}
		// set user's login shell
		if config.Shell != "" {
			cmd = fmt.Sprintf("%s -s %s", config.Shell)
		} else {
			cmd = fmt.Sprintf("%s -s %s", defaultShell)
		}

		cmd = fmt.Sprintf("%s %s", cmd, config.Name)

		// prevent errors when username already in use
		cmd = fmt.Sprintf("%s || [ $? -eq 9]", cmd)

		// add user to sudoers list
		if config.Sudoer {
			sudoLine := fmt.Sprintf("%s ALL=(ALL) NOPASSWD:ALL",
				config.Name)
			cmd = fmt.Sprintf("%s && %s",
				buildBashInsertLine(sudoLine, "/etc/suduers"))
		}
	case UserActionDel:
		cmd = fmt.Sprintf("%s -r %s", userdelCmd, config.Name)
		// prevent errors when user does not exist
		cmd = fmt.Sprintf("%s || [ $? -eq 6 ]", cmd)

		//	case UserActionModify:
		//		cmd = usermodCmd
	}

	return &UserModule{
		cmd: cmd,
	}
}

// Execute passes the command to executor and returns its results, the executor
// should be already initialized.
func (mod *UserModule) Execute(exec executor.TiOpsExecutor) ([]byte, []byte, error) {
	return exec.Execute(mod.cmd, true)
}

func buildBashInsertLine(line string, file string) string {
	if line == "" || file == "" {
		return ""
	}
	return fmt.Sprintf("grep -qxF '%s' %s || echo '%s' >> %s",
		line, file, line, file)
}
