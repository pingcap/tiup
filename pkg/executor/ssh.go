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
	"os"
	"path/filepath"
	"strconv"
	"time"

	"github.com/appleboy/easyssh-proxy"
	"github.com/pingcap-incubator/tiops/pkg/utils"
	"github.com/pingcap/errors"
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

var _ TiOpsExecutor = &SSHExecutor{}

// NewSSHExecutor create a ssh executor.
func NewSSHExecutor(c SSHConfig) (e *SSHExecutor, err error) {
	e = new(SSHExecutor)
	err = e.Initialize(c)
	if err != nil {
		return nil, errors.Trace(err)
	}

	return
}

// Initialize builds and initializes a SSHExecutor
func (sshExec *SSHExecutor) Initialize(config SSHConfig) error {
	// set default values
	if config.Port <= 0 {
		config.Port = 22
	}

	// build easyssh config
	sshExec.Config = &easyssh.MakeConfig{
		Server: config.Host,
		Port:   strconv.Itoa(config.Port),
		User:   config.User,
		// Timeout is the maximum amount of time for the TCP connection to establish.
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
func (sshExec *SSHExecutor) Execute(cmd string, sudo bool, timeout ...time.Duration) ([]byte, []byte, error) {
	// try to acquire root permission
	if sudo {
		cmd = fmt.Sprintf("sudo -H -u root bash -c \"%s\"", cmd)
	}

	// set a basic PATH in case it's empty on login
	cmd = fmt.Sprintf("PATH=$PATH:/usr/bin:/usr/sbin %s", cmd)

	// run command on remote host
	// default timeout is 60s in easyssh-proxy
	stdout, stderr, done, err := sshExec.Config.Run(cmd, timeout...)
	if err != nil {
		return []byte(stdout), []byte(stderr),
			errors.Annotatef(err, "cmd: '%s' on %s:%s", cmd, sshExec.Config.Server, sshExec.Config.Port)
	}

	if !done { // timeout case,
		return []byte(stdout), []byte(stderr),
			fmt.Errorf("timed out running \"%s\" on %s:%s",
				cmd,
				sshExec.Config.Server,
				sshExec.Config.Port)
	}

	return []byte(stdout), []byte(stderr), nil
}

// Transfer copies files via SCP
// This function depends on `scp` (a tool from OpenSSH or other SSH implementation)
// This function is based on easyssh.MakeConfig.Scp() but with support of copying
// file from remote to local.
func (sshExec *SSHExecutor) Transfer(src string, dst string, download bool) error {
	if !download {
		return sshExec.Config.Scp(src, dst)
	}

	// download file from remote
	session, err := sshExec.Config.Connect()
	if err != nil {
		return err
	}
	defer session.Close()

	targetPath := filepath.Dir(dst)
	if err = utils.CreateDir(targetPath); err != nil {
		return err
	}
	targetFile, err := os.Create(dst)
	if err != nil {
		return err
	}

	session.Stdout = targetFile

	return session.Run(fmt.Sprintf("cat %s", src))
}
