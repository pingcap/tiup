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
	"os/exec"
	"os/user"
	"path/filepath"
	"strings"
	"time"

	"github.com/fatih/color"
	"github.com/pingcap/tiup/pkg/cluster/ctxt"
	"github.com/pingcap/tiup/pkg/tui"
	"github.com/pingcap/tiup/pkg/utils"
	"go.uber.org/zap"
)

// Local execute the command at local host.
type Local struct {
	Config *SSHConfig
	Sudo   bool   // all commands run with this executor will be using sudo
	Locale string // the locale used when executing the command
}

var _ ctxt.Executor = &Local{}

// Execute implements Executor interface.
func (l *Local) Execute(ctx context.Context, cmd string, sudo bool, timeout ...time.Duration) ([]byte, []byte, error) {
	// change wd to default home
	cmd = fmt.Sprintf("cd; %s", cmd)
	// get current user name
	user, err := user.Current()
	if err != nil {
		return nil, nil, err
	}

	// try to acquire root permission
	if l.Sudo || sudo {
		cmd = fmt.Sprintf("/usr/bin/sudo -H -u root bash -c \"%s\"", strings.ReplaceAll(cmd, "\"", "\\\""))
	} else if l.Config.User != user.Name {
		cmd = fmt.Sprintf("/usr/bin/sudo -H -u %s bash -c \"%s\"", l.Config.User, strings.ReplaceAll(cmd, "\"", "\\\""))
	}

	// set a basic PATH in case it's empty on login
	cmd = fmt.Sprintf("PATH=$PATH:/bin:/sbin:/usr/bin:/usr/sbin %s", cmd)

	if l.Locale != "" {
		cmd = fmt.Sprintf("export LANG=%s; %s", l.Locale, cmd)
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

	command := exec.CommandContext(ctx, "/bin/bash", "-c", cmd)

	stdout := new(bytes.Buffer)
	stderr := new(bytes.Buffer)
	command.Stdout = stdout
	command.Stderr = stderr

	err = command.Run()

	zap.L().Info("LocalCommand",
		zap.String("cmd", cmd),
		zap.Error(err),
		zap.String("stdout", stdout.String()),
		zap.String("stderr", stderr.String()))

	if err != nil {
		baseErr := ErrSSHExecuteFailed.
			Wrap(err, "Failed to execute command locally").
			WithProperty(ErrPropSSHCommand, cmd).
			WithProperty(ErrPropSSHStdout, stdout).
			WithProperty(ErrPropSSHStderr, stderr)
		if len(stdout.Bytes()) > 0 || len(stderr.Bytes()) > 0 {
			output := strings.TrimSpace(strings.Join([]string{stdout.String(), stderr.String()}, "\n"))
			baseErr = baseErr.
				WithProperty(tui.SuggestionFromFormat("Command output:\n%s\n", color.YellowString(output)))
		}
		return stdout.Bytes(), stderr.Bytes(), baseErr
	}

	return stdout.Bytes(), stderr.Bytes(), err
}

// Transfer implements Executer interface.
func (l *Local) Transfer(ctx context.Context, src, dst string, download bool, limit int, _ bool) error {
	targetPath := filepath.Dir(dst)
	if err := utils.MkdirAll(targetPath, 0755); err != nil {
		return err
	}

	cmd := ""
	user, err := user.Current()
	if err != nil {
		return err
	}
	if download || user.Username == l.Config.User {
		cmd = fmt.Sprintf("cp %s %s", src, dst)
	} else {
		cmd = fmt.Sprintf("/usr/bin/sudo -H -u root bash -c \"cp %[1]s %[2]s && chown %[3]s:$(id -g -n %[3]s) %[2]s\"", src, dst, l.Config.User)
	}

	command := exec.Command("/bin/bash", "-c", cmd)
	stdout := new(bytes.Buffer)
	stderr := new(bytes.Buffer)
	command.Stdout = stdout
	command.Stderr = stderr

	err = command.Run()

	zap.L().Info("CPCommand",
		zap.String("cmd", cmd),
		zap.Error(err),
		zap.String("stdout", stdout.String()),
		zap.String("stderr", stderr.String()))

	if err != nil {
		baseErr := ErrSSHExecuteFailed.
			Wrap(err, "Failed to transfer file over local cp").
			WithProperty(ErrPropSSHCommand, cmd).
			WithProperty(ErrPropSSHStdout, stdout).
			WithProperty(ErrPropSSHStderr, stderr)
		if len(stdout.Bytes()) > 0 || len(stderr.Bytes()) > 0 {
			output := strings.TrimSpace(strings.Join([]string{stdout.String(), stderr.String()}, "\n"))
			baseErr = baseErr.
				WithProperty(tui.SuggestionFromFormat("Command output:\n%s\n", color.YellowString(output)))
		}
		return baseErr
	}

	return err
}
