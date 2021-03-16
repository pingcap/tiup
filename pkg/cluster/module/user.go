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

const (
	defaultShell = "/bin/bash"

	// UserActionAdd add user.
	UserActionAdd = "add"
	// UserActionDel delete user.
	UserActionDel = "del"
	// UserActionModify = "modify"

	// TODO: in RHEL/CentOS, the commands are in /usr/sbin, but in some
	// other distros they may be in other location such as /usr/bin, we'll
	// need to check and find the proper path of commands in the future.
	useraddCmd  = "/usr/sbin/useradd"
	userdelCmd  = "/usr/sbin/userdel"
	groupaddCmd = "/usr/sbin/groupadd"
	// usermodCmd = "/usr/sbin/usermod"
)

var (
	errNSUser = errNS.NewSubNamespace("user")
	// ErrUserAddFailed is ErrUserAddFailed
	ErrUserAddFailed = errNSUser.NewType("user_add_failed")
	// ErrUserDeleteFailed is ErrUserDeleteFailed
	ErrUserDeleteFailed = errNSUser.NewType("user_delete_failed")
)

// UserModuleConfig is the configurations used to initialize a UserModule
type UserModuleConfig struct {
	Action string // add, del or modify user
	Name   string // username
	Group  string // group name
	Home   string // home directory of user
	Shell  string // login shell of the user
	Sudoer bool   // when true, the user will be added to sudoers list
}

// UserModule is the module used to control systemd units
type UserModule struct {
	config UserModuleConfig
	cmd    string // the built command
}

// NewUserModule builds and returns a UserModule object base on given config.
func NewUserModule(config UserModuleConfig) *UserModule {
	cmd := ""

	switch config.Action {
	case UserActionAdd:
		cmd = useraddCmd
		// You have to use -m, otherwise no home directory will be created. If you want to specify the path of the home directory, use -d and specify the path
		// useradd -m -d /PATH/TO/FOLDER
		cmd += " -m"
		if config.Home != "" {
			cmd += " -d" + config.Home
		}

		// set user's login shell
		if config.Shell != "" {
			cmd = fmt.Sprintf("%s -s %s", cmd, config.Shell)
		} else {
			cmd = fmt.Sprintf("%s -s %s", cmd, defaultShell)
		}

		// set user's group
		if config.Group == "" {
			config.Group = config.Name
		}

		// groupadd -f <group-name>
		groupAdd := fmt.Sprintf("%s -f %s", groupaddCmd, config.Group)

		// useradd -g <group-name> <user-name>
		cmd = fmt.Sprintf("%s -g %s %s", cmd, config.Group, config.Name)

		// prevent errors when username already in use
		cmd = fmt.Sprintf("id -u %s > /dev/null 2>&1 || (%s && %s)", config.Name, groupAdd, cmd)

		// add user to sudoers list
		if config.Sudoer {
			sudoLine := fmt.Sprintf("%s ALL=(ALL) NOPASSWD:ALL",
				config.Name)
			cmd = fmt.Sprintf("%s && %s",
				cmd,
				fmt.Sprintf("echo '%s' > /etc/sudoers.d/%s", sudoLine, config.Name))
		}

	case UserActionDel:
		cmd = fmt.Sprintf("%s -r %s", userdelCmd, config.Name)
		// prevent errors when user does not exist
		cmd = fmt.Sprintf("%s || [ $? -eq 6 ]", cmd)

		//	case UserActionModify:
		//		cmd = usermodCmd
	}

	return &UserModule{
		config: config,
		cmd:    cmd,
	}
}

// Execute passes the command to executor and returns its results, the executor
// should be already initialized.
func (mod *UserModule) Execute(ctx context.Context, exec ctxt.Executor) ([]byte, []byte, error) {
	a, b, err := exec.Execute(ctx, mod.cmd, true)
	if err != nil {
		switch mod.config.Action {
		case UserActionAdd:
			return a, b, ErrUserAddFailed.
				Wrap(err, "Failed to create new system user '%s' on remote host", mod.config.Name)
		case UserActionDel:
			return a, b, ErrUserDeleteFailed.
				Wrap(err, "Failed to delete system user '%s' on remote host", mod.config.Name)
		}
	}
	return a, b, nil
}
