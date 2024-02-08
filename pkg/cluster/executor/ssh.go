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
	"github.com/pingcap/errors"
	"github.com/pingcap/tiup/pkg/cluster/ctxt"
	"github.com/pingcap/tiup/pkg/localdata"
	"github.com/pingcap/tiup/pkg/tui"
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
		Host       string        // hostname of the SSH server
		Port       int           // port of the SSH server
		User       string        // username to login to the SSH server
		Password   string        // password of the user
		KeyFile    string        // path to the private key file
		Passphrase string        // passphrase of the private key file
		Timeout    time.Duration // Timeout is the maximum amount of time for the TCP connection to establish.
		ExeTimeout time.Duration // ExeTimeout is the maximum amount of time for the command to finish
		Proxy      *SSHConfig    // ssh proxy config
	}
)

var _ ctxt.Executor = &EasySSHExecutor{}
var _ ctxt.Executor = &NativeSSHExecutor{}

// initialize builds and initializes a EasySSHExecutor
func (e *EasySSHExecutor) initialize(config SSHConfig) {
	// build easyssh config
	e.Config = &easyssh.MakeConfig{
		Server:  config.Host,
		Port:    strconv.Itoa(config.Port),
		User:    config.User,
		Timeout: config.Timeout, // timeout when connecting to remote
	}

	if config.ExeTimeout > 0 {
		executeDefaultTimeout = config.ExeTimeout
	}

	// prefer private key authentication
	if len(config.KeyFile) > 0 {
		e.Config.KeyPath = config.KeyFile
		e.Config.Passphrase = config.Passphrase
	} else if len(config.Password) > 0 {
		e.Config.Password = config.Password
	}

	if proxy := config.Proxy; proxy != nil {
		e.Config.Proxy = easyssh.DefaultConfig{
			Server:  proxy.Host,
			Port:    strconv.Itoa(proxy.Port),
			User:    proxy.User,
			Timeout: proxy.Timeout, // timeout when connecting to remote
		}
		if len(proxy.KeyFile) > 0 {
			e.Config.Proxy.KeyPath = proxy.KeyFile
			e.Config.Proxy.Passphrase = proxy.Passphrase
		} else if len(proxy.Password) > 0 {
			e.Config.Proxy.Password = proxy.Password
		}
	}
}

// Execute run the command via SSH, it's not invoking any specific shell by default.
func (e *EasySSHExecutor) Execute(ctx context.Context, cmd string, sudo bool, timeout ...time.Duration) ([]byte, []byte, error) {
	// try to acquire root permission
	if e.Sudo || sudo {
		cmd = fmt.Sprintf("/usr/bin/sudo -H bash -c \"%s\"", cmd)
	}

	// set a basic PATH in case it's empty on login
	cmd = fmt.Sprintf("PATH=$PATH:/bin:/sbin:/usr/bin:/usr/sbin; %s", cmd)

	if e.Locale != "" {
		cmd = fmt.Sprintf("export LANG=%s; %s", e.Locale, cmd)
	}

	// run command on remote host
	// default timeout is 60s in easyssh-proxy
	if len(timeout) == 0 {
		timeout = append(timeout, executeDefaultTimeout)
	}

	stdout, stderr, done, err := e.Config.Run(cmd, timeout...)

	logfn := zap.L().Info
	if err != nil {
		logfn = zap.L().Error
	}
	logfn("SSHCommand",
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
				WithProperty(tui.SuggestionFromFormat("Command output on remote host %s:\n%s\n",
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
func (e *EasySSHExecutor) Transfer(ctx context.Context, src, dst string, download bool, limit int, compress bool) error {
	if !download {
		err := e.Config.Scp(src, dst)
		if err != nil {
			return errors.Annotatef(err, "failed to scp %s to %s@%s:%s", src, e.Config.User, e.Config.Server, dst)
		}
		return nil
	}

	// download file from remote
	session, client, err := e.Config.Connect()
	if err != nil {
		return err
	}
	defer client.Close()
	defer session.Close()

	err = utils.MkdirAll(filepath.Dir(dst), 0755)
	if err != nil {
		return nil
	}
	return ScpDownload(session, client, src, dst, limit, compress)
}

func (e *NativeSSHExecutor) prompt(def string) string {
	if prom := os.Getenv(localdata.EnvNameSSHPassPrompt); prom != "" {
		return prom
	}
	return def
}

func (e *NativeSSHExecutor) configArgs(args []string, isScp bool) []string {
	if e.Config.Port != 0 && e.Config.Port != 22 {
		if isScp {
			args = append(args, "-P", strconv.Itoa(e.Config.Port))
		} else {
			args = append(args, "-p", strconv.Itoa(e.Config.Port))
		}
	}
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

	proxy := e.Config.Proxy
	if proxy != nil {
		proxyArgs := []string{"ssh"}
		if proxy.Timeout != 0 {
			proxyArgs = append(proxyArgs, "-o", fmt.Sprintf("ConnectTimeout=%d", int64(proxy.Timeout.Seconds())))
		}
		if proxy.Password != "" {
			proxyArgs = append([]string{"sshpass", "-p", proxy.Password, "-P", e.prompt("password")}, proxyArgs...)
		} else if proxy.KeyFile != "" {
			proxyArgs = append(proxyArgs, "-i", proxy.KeyFile)
			if proxy.Passphrase != "" {
				proxyArgs = append([]string{"sshpass", "-p", proxy.Passphrase, "-P", e.prompt("passphrase")}, proxyArgs...)
			}
		}
		// Don't need to extra quote it, exec.Command will handle it right
		// ref https://stackoverflow.com/a/26473771/2298986
		args = append(args, []string{"-o", fmt.Sprintf(`ProxyCommand=%s %s@%s -p %d -W %%h:%%p`, strings.Join(proxyArgs, " "), proxy.User, proxy.Host, proxy.Port)}...)
	}
	return args
}

// Execute run the command via SSH, it's not invoking any specific shell by default.
func (e *NativeSSHExecutor) Execute(ctx context.Context, cmd string, sudo bool, timeout ...time.Duration) ([]byte, []byte, error) {
	if e.ConnectionTestResult != nil {
		return nil, nil, e.ConnectionTestResult
	}

	// try to acquire root permission
	if e.Sudo || sudo {
		cmd = fmt.Sprintf("/usr/bin/sudo -H bash -c \"%s\"", cmd)
	}

	// set a basic PATH in case it's empty on login
	cmd = fmt.Sprintf("PATH=$PATH:/bin:/sbin:/usr/bin:/usr/sbin %s", cmd)

	if e.Locale != "" {
		cmd = fmt.Sprintf("export LANG=%s; %s", e.Locale, cmd)
	}

	// run command on remote host
	// default timeout is 60s in easyssh-proxy
	if len(timeout) == 0 {
		timeout = append(timeout, executeDefaultTimeout)
	}

	if len(timeout) > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, timeout[0])
		defer cancel()
	}

	ssh := "ssh"

	if val := os.Getenv(localdata.EnvNameSSHPath); val != "" {
		if isExec := utils.IsExecBinary(val); !isExec {
			return nil, nil, fmt.Errorf("specified SSH in the environment variable `%s` does not exist or is not executable", localdata.EnvNameSSHPath)
		}
		ssh = val
	}

	args := []string{ssh, "-o", "StrictHostKeyChecking=no"}

	args = e.configArgs(args, false) // prefix and postfix args
	args = append(args, fmt.Sprintf("%s@%s", e.Config.User, e.Config.Host), cmd)

	command := exec.CommandContext(ctx, args[0], args[1:]...)

	stdout := new(bytes.Buffer)
	stderr := new(bytes.Buffer)
	command.Stdout = stdout
	command.Stderr = stderr

	err := command.Run()

	logfn := zap.L().Info
	if err != nil {
		logfn = zap.L().Error
	}
	logfn("SSHCommand",
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
				WithProperty(tui.SuggestionFromFormat("Command output on remote host %s:\n%s\n",
					e.Config.Host,
					color.YellowString(output)))
		}
		return stdout.Bytes(), stderr.Bytes(), baseErr
	}

	return stdout.Bytes(), stderr.Bytes(), err
}

// Transfer copies files via SCP
// This function depends on `scp` (a tool from OpenSSH or other SSH implementation)
func (e *NativeSSHExecutor) Transfer(ctx context.Context, src, dst string, download bool, limit int, compress bool) error {
	if e.ConnectionTestResult != nil {
		return e.ConnectionTestResult
	}

	scp := "scp"

	if val := os.Getenv(localdata.EnvNameSCPPath); val != "" {
		if isExec := utils.IsExecBinary(val); !isExec {
			return fmt.Errorf("specified SCP in the environment variable `%s` does not exist or is not executable", localdata.EnvNameSCPPath)
		}
		scp = val
	}

	args := []string{scp, "-r", "-o", "StrictHostKeyChecking=no"}
	if limit > 0 {
		args = append(args, "-l", fmt.Sprint(limit))
	}
	if compress {
		args = append(args, "-C")
	}
	args = e.configArgs(args, true) // prefix and postfix args

	if download {
		targetPath := filepath.Dir(dst)
		if err := utils.MkdirAll(targetPath, 0755); err != nil {
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

	err := command.Run()

	logfn := zap.L().Info
	if err != nil {
		logfn = zap.L().Error
	}
	logfn("SCPCommand",
		zap.String("host", e.Config.Host),
		zap.Int("port", e.Config.Port),
		zap.String("cmd", strings.Join(args, " ")),
		zap.Error(err),
		zap.String("stdout", stdout.String()),
		zap.String("stderr", stderr.String()))

	if err != nil {
		baseErr := ErrSSHExecuteFailed.
			Wrap(err, "Failed to transfer file over SCP for '%s@%s:%d'", e.Config.User, e.Config.Host, e.Config.Port).
			WithProperty(ErrPropSSHCommand, strings.Join(args, " ")).
			WithProperty(ErrPropSSHStdout, stdout).
			WithProperty(ErrPropSSHStderr, stderr)
		if len(stdout.Bytes()) > 0 || len(stderr.Bytes()) > 0 {
			output := strings.TrimSpace(strings.Join([]string{stdout.String(), stderr.String()}, "\n"))
			baseErr = baseErr.
				WithProperty(tui.SuggestionFromFormat("Command output on remote host %s:\n%s\n",
					e.Config.Host,
					color.YellowString(output)))
		}
		return baseErr
	}

	return err
}
