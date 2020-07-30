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
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/appleboy/easyssh-proxy"
	"github.com/fatih/color"
	"github.com/joomcode/errorx"
	"github.com/pingcap/failpoint"
	"github.com/pingcap/tiup/pkg/cliutil"
	"github.com/pingcap/tiup/pkg/localdata"
	"github.com/pingcap/tiup/pkg/utils"
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

	// ErrSSHExecuteFailed is ErrSSHExecuteFailed
	ErrSSHExecuteFailed = errNSSSH.NewType("execute_failed")
	// ErrSSHExecuteTimedout is ErrSSHExecuteTimedout
	ErrSSHExecuteTimedout = errNSSSH.NewType("execute_timedout")
)

var executeDefaultTimeout = time.Second * 60

// This command will be execute once the NativeSSHExecutor is created.
// It's used to predict if the connection can establish success in the future.
// Its main purpose is to avoid sshpass hang when user speficied a wrong prompt.
var connectionTestCommand = "echo connection test, if killed, check the password prompt"

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
	// EasySSHExecutor implements Executor with EasySSH as transportation layer.
	EasySSHExecutor struct {
		Config *easyssh.MakeConfig
		Locale string // the locale used when executing the command
		Sudo   bool   // all commands run with this executor will be using sudo
	}

	// NativeSSHExecutor implements Excutor with native SSH transportation layer.
	NativeSSHExecutor struct {
		Config               *SSHConfig
		Locale               string // the locale used when executing the command
		Sudo                 bool   // all commands run with this executor will be using sudo
		ConnectionTestResult error  // test if the connection can be established in initialization phase
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

var _ Executor = &EasySSHExecutor{}
var _ Executor = &NativeSSHExecutor{}

// NewSSHExecutor create a ssh executor.
func NewSSHExecutor(c SSHConfig, sudo bool, native bool) Executor {
	// set default values
	if c.Port <= 0 {
		c.Port = 22
	}

	if c.Timeout == 0 {
		c.Timeout = time.Second * 5 // default timeout is 5 sec
	}

	if native {
		e := &NativeSSHExecutor{
			Config: &c,
			Locale: "C",
			Sudo:   sudo,
		}
		if c.Password != "" || (c.KeyFile != "" && c.Passphrase != "") {
			_, _, e.ConnectionTestResult = e.Execute(connectionTestCommand, false, executeDefaultTimeout)
		}
		return e
	}

	// Used in integration testing, to check if native ssh client is really used when it need to be.
	failpoint.Inject("assertNativeSSH", func() {
		msg := fmt.Sprintf(
			"native ssh client should be used in this case, os.Args: %s, %s = %s",
			os.Args, localdata.EnvNameNativeSSHClient, os.Getenv(localdata.EnvNameNativeSSHClient),
		)
		panic(msg)
	})

	e := new(EasySSHExecutor)
	e.initialize(c)
	e.Locale = "C" // default locale, hard coded for now
	e.Sudo = sudo
	return e
}

// Initialize builds and initializes a EasySSHExecutor
func (e *EasySSHExecutor) initialize(config SSHConfig) {
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
	} else if len(config.Password) > 0 {
		e.Config.Password = config.Password
	}
}

// Execute run the command via SSH, it's not invoking any specific shell by default.
func (e *EasySSHExecutor) Execute(cmd string, sudo bool, timeout ...time.Duration) ([]byte, []byte, error) {
	// try to acquire root permission
	if e.Sudo || sudo {
		cmd = fmt.Sprintf("sudo -H -u root bash -c \"%s\"", cmd)
	}

	// set a basic PATH in case it's empty on login
	cmd = fmt.Sprintf("PATH=$PATH:/usr/bin:/usr/sbin %s", cmd)

	if e.Locale != "" {
		cmd = fmt.Sprintf("export LANG=%s; %s", e.Locale, cmd)
	}

	// run command on remote host
	// default timeout is 60s in easyssh-proxy
	if len(timeout) == 0 {
		timeout = append(timeout, executeDefaultTimeout)
	}

	stdout, stderr, done, err := e.Config.Run(cmd, timeout...)

	zap.L().Info("SSHCommand",
		zap.String("host", e.Config.Server),
		zap.String("port", e.Config.Port),
		zap.String("cmd", cmd),
		zap.Error(err),
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
func (e *EasySSHExecutor) Transfer(src string, dst string, download bool) error {
	if !download {
		return e.Config.Scp(src, dst)
	}

	// download file from remote
	session, client, err := e.Config.Connect()
	if err != nil {
		return err
	}
	defer client.Close()
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

func (e *NativeSSHExecutor) prompt(def string) string {
	if prom := os.Getenv(localdata.EnvNameSSHPassPrompt); prom != "" {
		return prom
	}
	return def
}

func (e *NativeSSHExecutor) configArgs(args []string) []string {
	if e.Config.Timeout != 0 {
		args = append(args, "-o", fmt.Sprintf("ConnectTimeout=%d", int64(e.Config.Timeout.Seconds())))
	}
	if e.Config.Password != "" {
		args = append([]string{"sshpass", "-p", e.Config.Password, "-P", e.prompt("password")}, args...)
	} else if e.Config.KeyFile != "" {
		args = append(args, "-i", e.Config.KeyFile)
		if e.Config.Passphrase != "" {
			args = append([]string{"sshpass", "-p", e.Config.Passphrase, "-P", e.prompt("passphrase")}, args...)
		}
	}
	return args
}

// Execute run the command via SSH, it's not invoking any specific shell by default.
func (e *NativeSSHExecutor) Execute(cmd string, sudo bool, timeout ...time.Duration) ([]byte, []byte, error) {
	if e.ConnectionTestResult != nil {
		return nil, nil, e.ConnectionTestResult
	}

	// try to acquire root permission
	if e.Sudo || sudo {
		cmd = fmt.Sprintf("sudo -H -u root bash -c \"%s\"", cmd)
	}

	// set a basic PATH in case it's empty on login
	cmd = fmt.Sprintf("PATH=$PATH:/usr/bin:/usr/sbin %s", cmd)

	if e.Locale != "" {
		cmd = fmt.Sprintf("export LANG=%s; %s", e.Locale, cmd)
	}

	// run command on remote host
	// default timeout is 60s in easyssh-proxy
	if len(timeout) == 0 {
		timeout = append(timeout, executeDefaultTimeout)
	}

	ctx := context.Background()
	if len(timeout) > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(context.Background(), timeout[0])
		defer cancel()
	}

	args := []string{"ssh", "-o", "StrictHostKeyChecking=no"}
	args = e.configArgs(args) // prefix and postfix args
	args = append(args, fmt.Sprintf("%s@%s", e.Config.User, e.Config.Host), cmd)

	command := exec.CommandContext(ctx, args[0], args[1:]...)

	stdout := new(bytes.Buffer)
	stderr := new(bytes.Buffer)
	command.Stdout = stdout
	command.Stderr = stderr

	err := command.Run()

	zap.L().Info("SSHCommand",
		zap.String("host", e.Config.Host),
		zap.Int("port", e.Config.Port),
		zap.String("cmd", cmd),
		zap.Error(err),
		zap.String("stdout", stdout.String()),
		zap.String("stderr", stderr.String()))

	if err != nil {
		baseErr := ErrSSHExecuteFailed.
			Wrap(err, "Failed to execute command over SSH for '%s@%s:%d'", e.Config.User, e.Config.Host, e.Config.Port).
			WithProperty(ErrPropSSHCommand, cmd).
			WithProperty(ErrPropSSHStdout, stdout).
			WithProperty(ErrPropSSHStderr, stderr)
		if len(stdout.Bytes()) > 0 || len(stderr.Bytes()) > 0 {
			output := strings.TrimSpace(strings.Join([]string{stdout.String(), stderr.String()}, "\n"))
			baseErr = baseErr.
				WithProperty(cliutil.SuggestionFromFormat("Command output on remote host %s:\n%s\n",
					e.Config.Host,
					color.YellowString(output)))
		}
		return stdout.Bytes(), stderr.Bytes(), baseErr
	}

	return stdout.Bytes(), stderr.Bytes(), err
}

// Transfer copies files via SCP
// This function depends on `scp` (a tool from OpenSSH or other SSH implementation)
func (e *NativeSSHExecutor) Transfer(src string, dst string, download bool) error {
	if e.ConnectionTestResult != nil {
		return e.ConnectionTestResult
	}

	args := []string{"scp", "-r", "-o", "StrictHostKeyChecking=no"}
	args = e.configArgs(args) // prefix and postfix args

	if download {
		targetPath := filepath.Dir(dst)
		if err := utils.CreateDir(targetPath); err != nil {
			return err
		}
		args = append(args, fmt.Sprintf("%s@%s:%s", e.Config.User, e.Config.Host, src), dst)
	} else {
		args = append(args, src, fmt.Sprintf("%s@%s:%s", e.Config.User, e.Config.Host, dst))
	}

	command := exec.Command(args[0], args[1:]...)
	stdout := new(bytes.Buffer)
	stderr := new(bytes.Buffer)
	command.Stdout = stdout
	command.Stderr = stderr

	return command.Run()
}
