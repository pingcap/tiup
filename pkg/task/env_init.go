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

package task

import (
	"fmt"
	"io/ioutil"
	"strings"

	"github.com/joomcode/errorx"
	"github.com/pingcap-incubator/tiup-cluster/pkg/module"
)

var (
	errNSEnvInit               = errNS.NewSubNamespace("env_init")
	errEnvInitSubCommandFailed = errNSEnvInit.NewType("sub_command_failed")
	// ErrEnvInitFailed is ErrEnvInitFailed
	ErrEnvInitFailed = errNSEnvInit.NewType("failed")
	// SSH authorized_keys file
	defaultSSHAuthorizedKeys = "~/.ssh/authorized_keys"
)

// EnvInit is used to initialize the remote environment, e.g:
// 1. Generate SSH key
// 2. ssh-copy-id
type EnvInit struct {
	host       string
	deployUser string
}

// Execute implements the Task interface
func (e *EnvInit) Execute(ctx *Context) error {
	return e.execute(ctx)
}

func (e *EnvInit) execute(ctx *Context) error {
	wrapError := func(err error) *errorx.Error {
		return ErrEnvInitFailed.Wrap(err, "Failed to initialize TiDB environment on remote host '%s'", e.host)
	}

	exec, found := ctx.GetExecutor(e.host)
	if !found {
		panic(ErrNoExecutor)
	}

	um := module.NewUserModule(module.UserModuleConfig{
		Action: module.UserActionAdd,
		Name:   e.deployUser,
		Sudoer: true,
	})

	_, _, errx := um.Execute(exec)
	if errx != nil {
		return wrapError(errx)
	}

	pubKey, err := ioutil.ReadFile(ctx.PublicKeyPath)
	if err != nil {
		return wrapError(err)
	}

	// Authorize
	cmd := `su - ` + e.deployUser + ` -c 'test -d ~/.ssh || mkdir -p ~/.ssh && chmod 700 ~/.ssh'`
	_, _, err = exec.Execute(cmd, true)
	if err != nil {
		return wrapError(errEnvInitSubCommandFailed.
			Wrap(err, "Failed to create '~/.ssh' directory for user '%s'", e.deployUser))
	}

	// detect if custom path of authorized keys file is set
	// NOTE: we do not yet support:
	//   - custom config for user (~/.ssh/config)
	//   - sshd started with custom config (other than /etc/ssh/sshd_config)
	//   - ssh server implementations other than OpenSSH (such as dropbear)
	sshAuthorizedKeys := defaultSSHAuthorizedKeys
	cmd = "grep -Ev '^#|^\\s*$' /etc/ssh/sshd_config"
	stdout, _, _ := exec.Execute(cmd, true) // error ignored as we have default value
	for _, line := range strings.Split(string(stdout), "\n") {
		if !strings.Contains(line, "AuthorizedKeysFile") {
			continue
		}
		fields := strings.Fields(line)
		if len(fields) >= 2 {
			sshAuthorizedKeys = fields[1]
		}
	}

	if !strings.HasPrefix(sshAuthorizedKeys, "/") && !strings.HasPrefix(sshAuthorizedKeys, "~") {
		sshAuthorizedKeys = fmt.Sprintf("~/%s", sshAuthorizedKeys)
	}

	pk := strings.TrimSpace(string(pubKey))
	cmd = fmt.Sprintf(`su - %[1]s -c 'grep $(echo %[2]s) %[3]s || echo %[2]s >> %[3]s && chmod 600 %[3]s'`,
		e.deployUser, pk, sshAuthorizedKeys)
	_, _, err = exec.Execute(cmd, true)
	if err != nil {
		return wrapError(errEnvInitSubCommandFailed.
			Wrap(err, "Failed to write public keys to '%s' for user '%s'", sshAuthorizedKeys, e.deployUser))
	}

	return nil
}

// Rollback implements the Task interface
func (e *EnvInit) Rollback(ctx *Context) error {
	return ErrUnsupportedRollback
}

// String implements the fmt.Stringer interface
func (e *EnvInit) String() string {
	return fmt.Sprintf("EnvInit: user=%s, host=%s", e.deployUser, e.host)
}
