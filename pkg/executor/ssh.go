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

package executor

import (
	"fmt"
	"time"

	"github.com/appleboy/easyssh-proxy"
)

type (
	// SSHExecutor implements TiOpsExecutor with SSH as transportation layer.
	SSHExecutor struct {
		Config *easyssh.MakeConfig
	}

	// SSHConfig is the configuration needed to establish SSH connection.
	SSHConfig struct {
		Host       string // hostname of the SSH server
		Port       int    // port of the SSH server
		User       string // username to login to the SSH server
		Password   string // password of the user
		KeyFile    string // path to the private key file
		Passphrase string // passphrase of the private key file
	}
)

// Initialize builds and initializes a SSHExecutor
func (sshExec *SSHExecutor) Initialize(config SSHConfig) error {
	// build easyssh config
	sshExec.Config = &easyssh.MakeConfig{
		Server:  config.Host,
		Port:    fmt.Sprintf("%d", config.Port),
		User:    config.User,
		Timeout: time.Second * 5, // default timeout is 5 sec
	}

	// prefer private key authentication
	if len(config.KeyFile) > 0 {
		sshExec.Config.KeyPath = config.KeyFile
		sshExec.Config.Passphrase = config.Passphrase
	} else {
		sshExec.Config.Password = config.Password
	}

	return nil
}

// Execute run the command via SSH, it's not invoking any specific shell by default.
func (sshExec *SSHExecutor) Execute(cmd string, sudo bool) ([]byte, []byte, error) {
	// try to acquire root permission
	if sudo {
		cmd = fmt.Sprintf("sudo -H -u root %s", cmd)
	}

	// run command on remote host
	stdout, stderr, done, err := sshExec.Config.Run(cmd)
	if !done {
		return nil, nil, fmt.Errorf("connection timed out to %s:%s",
			sshExec.Config.Server,
			sshExec.Config.Port)
	}
	return []byte(stdout), []byte(stderr), err
}

// Transfer copies files via SCP
// This function depends on `scp` (a tool from OpenSSH)
func (sshExec *SSHExecutor) Transfer(src string, dst string) error {
	return sshExec.Config.Scp(src, dst)
}
