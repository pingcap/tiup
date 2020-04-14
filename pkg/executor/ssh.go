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
	"strings"
	"time"

	"github.com/appleboy/easyssh-proxy"
	"github.com/fatih/color"
	"github.com/joomcode/errorx"
	"github.com/pingcap-incubator/tiup-cluster/pkg/cliutil"
	"github.com/pingcap-incubator/tiup-cluster/pkg/errutil"
	"github.com/pingcap-incubator/tiup-cluster/pkg/utils"
	"go.uber.org/zap"
)

var (
	errNSSSH = errNS.NewSubNamespace("ssh")

	// ErrPropSSHCommand is ErrPropSSHCommand
	ErrPropSSHCommand = errorx.RegisterPrintableProperty("ssh_command")
	// ErrPropSSHStdout is ErrPropSSHStdout
	ErrPropSSHStdout = errorx.RegisterPrintableProperty("ssh_stdout")
	// ErrPropSSHStderr is ErrPropSSHStderr
	ErrPropSSHStderr = errorx.RegisterPrintableProperty("ssh_stderr")

	// ErrSSHRequireCredential is ErrSSHRequireCredential.
	// FIXME: This error should be removed since we should prompt for error if necessary.
	ErrSSHRequireCredential = errNSSSH.NewType("credential_required", errutil.ErrTraitPreCheck)
	// ErrSSHExecuteFailed is ErrSSHExecuteFailed
	ErrSSHExecuteFailed = errNSSSH.NewType("execute_failed")
	// ErrSSHExecuteTimedout is ErrSSHExecuteTimedout
	ErrSSHExecuteTimedout = errNSSSH.NewType("execute_timedout")
)

var executeDefaultTimeout = time.Second * 60

func init() {
	v := os.Getenv("TIUP_CLUSTER_EXECUTE_DEFAULT_TIMEOUT")
	if v != "" {
		d, err := time.ParseDuration(v)
		if err != nil {
			fmt.Println("ignore invalid TIUP_CLUSTER_EXECUTE_DEFAULT_TIMEOUT: ", v)
			return
		}

		executeDefaultTimeout = d
	}
}

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
		// Timeout is the maximum amount of time for the TCP connection to establish.
		Timeout time.Duration
	}
)

var _ TiOpsExecutor = &SSHExecutor{}

// NewSSHExecutor create a ssh executor.
func NewSSHExecutor(c SSHConfig) *SSHExecutor {
	e := new(SSHExecutor)
	e.Initialize(c)
	return e
}

// Initialize builds and initializes a SSHExecutor
func (e *SSHExecutor) Initialize(config SSHConfig) {
	// set default values
	if config.Port <= 0 {
		config.Port = 22
	}

	if config.Timeout == 0 {
		config.Timeout = time.Second * 5 // default timeout is 5 sec
	}

	// build easyssh config
	e.Config = &easyssh.MakeConfig{
		Server:  config.Host,
		Port:    strconv.Itoa(config.Port),
		User:    config.User,
		Timeout: config.Timeout, // timeout when connecting to remote
	}

	// prefer private key authentication
	if len(config.KeyFile) > 0 {
		e.Config.KeyPath = config.KeyFile
		e.Config.Passphrase = config.Passphrase
	} else {
		e.Config.Password = config.Password
	}
}

// Execute run the command via SSH, it's not invoking any specific shell by default.
func (e *SSHExecutor) Execute(cmd string, sudo bool, timeout ...time.Duration) ([]byte, []byte, error) {
	// try to acquire root permission
	if sudo {
		cmd = fmt.Sprintf("sudo -H -u root bash -c \"%s\"", cmd)
	}

	// set a basic PATH in case it's empty on login
	cmd = fmt.Sprintf("PATH=$PATH:/usr/bin:/usr/sbin %s", cmd)

	// run command on remote host
	// default timeout is 60s in easyssh-proxy
	if len(timeout) == 0 {
		timeout = append(timeout, executeDefaultTimeout)
	}
	stdout, stderr, done, err := e.Config.Run(cmd, timeout...)

	zap.L().Info("ssh command",
		zap.String("host", e.Config.Server),
		zap.String("port", e.Config.Port),
		zap.String("cmd", cmd),
		zap.String("stdout", stdout),
		zap.String("stderr", stderr))

	if err != nil {
		baseErr := ErrSSHExecuteFailed.
			Wrap(err, "Failed to execute command over SSH for '%s@%s:%s'", e.Config.User, e.Config.Server, e.Config.Port).
			WithProperty(ErrPropSSHCommand, cmd).
			WithProperty(ErrPropSSHStdout, stdout).
			WithProperty(ErrPropSSHStderr, stderr)
		if len(stdout) > 0 || len(stderr) > 0 {
			output := strings.TrimSpace(strings.Join([]string{stdout, stderr}, "\n"))
			baseErr = baseErr.
				WithProperty(cliutil.SuggestionFromFormat("Command output on remote host %s:\n%s\n",
					e.Config.Server,
					color.YellowString(output)))
		}
		return []byte(stdout), []byte(stderr), baseErr
	}

	if !done { // timeout case,
		return []byte(stdout), []byte(stderr), ErrSSHExecuteTimedout.
			Wrap(err, "Execute command over SSH timedout for '%s@%s:%s'", e.Config.User, e.Config.Server, e.Config.Port).
			WithProperty(ErrPropSSHCommand, cmd).
			WithProperty(ErrPropSSHStdout, stdout).
			WithProperty(ErrPropSSHStderr, stderr)
	}

	return []byte(stdout), []byte(stderr), nil
}

// Transfer copies files via SCP
// This function depends on `scp` (a tool from OpenSSH or other SSH implementation)
// This function is based on easyssh.MakeConfig.Scp() but with support of copying
// file from remote to local.
func (e *SSHExecutor) Transfer(src string, dst string, download bool) error {
	if !download {
		return e.Config.Scp(src, dst)
	}

	// download file from remote
	session, err := e.Config.Connect()
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
