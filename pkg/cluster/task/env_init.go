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
	"github.com/pingcap/tiup/pkg/cluster/executor"
	"github.com/pingcap/tiup/pkg/cluster/module"
)

// PrivilegeEscalationMethod defiles availabile linux privilege escalation commands
type PrivilegeEscalationMethod int

const (
	// Invalid privilege escalation mathod as a default
	Invalid PrivilegeEscalationMethod = 0
	// Sudo command
	Sudo PrivilegeEscalationMethod = 1
	//Su command
	Su PrivilegeEscalationMethod = 2
	//Runuser command
	Runuser PrivilegeEscalationMethod = 3
)

var (
	errNSEnvInit               = errNS.NewSubNamespace("env_init")
	errEnvInitSubCommandFailed = errNSEnvInit.NewType("sub_command_failed")
	// ErrEnvInitFailed is ErrEnvInitFailed
	ErrEnvInitFailed = errNSEnvInit.NewType("failed")
)

// EnvInit is used to initialize the remote environment, e.g:
// 1. Generate SSH key
// 2. ssh-copy-id
type EnvInit struct {
	host           string
	deployUser     string
	userGroup      string
	skipCreateUser bool
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

	if !e.skipCreateUser {
		um := module.NewUserModule(module.UserModuleConfig{
			Action: module.UserActionAdd,
			Name:   e.deployUser,
			Group:  e.userGroup,
			Sudoer: true,
		})

		_, _, errx := um.Execute(exec)
		if errx != nil {
			return wrapError(errx)
		}
	}

	pubKey, err := ioutil.ReadFile(ctx.PublicKeyPath)
	if err != nil {
		return wrapError(err)
	}

	// some of the privilege escalation commands might be prohibited by the linux administrator
	// try different methods and choose the one that works (su, runuser, sudo)
	// raise an error when there is no privilege escalation method available
	var privilegeEscalationMethod PrivilegeEscalationMethod = Invalid
	pems := map[PrivilegeEscalationMethod]string{
		Su:      fmt.Sprintf(`su - %s -c 'echo 0'`, e.deployUser),
		Runuser: fmt.Sprintf(`runuser -l %s -c 'echo 0'`, e.deployUser),
		Sudo:    `sudo echo 0`,
	}
	for pem, cmd := range pems {
		_, _, err = exec.Execute(cmd, true)
		if err == nil {
			privilegeEscalationMethod = pem
			break
		}
	}
	if privilegeEscalationMethod == Invalid {
		return wrapError(errEnvInitSubCommandFailed.
			Wrap(err, "No privilege escalation method available"))
	}

	// Authorize
	var cmd string
	switch privilegeEscalationMethod {
	case Su:
		cmd = fmt.Sprintf(`su - %s -c 'test -d ~/.ssh || mkdir -p ~/.ssh && chmod 700 ~/.ssh'`, e.deployUser)
	case Runuser:
		cmd = fmt.Sprintf(`runuser -l %s -c 'test -d ~/.ssh || mkdir -p ~/.ssh && chmod 700 ~/.ssh'`, e.deployUser)
	case Sudo:
		cmd = fmt.Sprintf(`sudo test -d ~/.ssh || sudo mkdir -p ~/.ssh && sudo chmod 700 ~/.ssh && sudo chown %s:%s ~/.ssh`, e.deployUser, e.userGroup)
	}
	_, _, err = exec.Execute(cmd, true)
	if err != nil {
		return wrapError(errEnvInitSubCommandFailed.
			Wrap(err, "Failed to create '~/.ssh' directory for user '%s'", e.deployUser))
	}

	pk := strings.TrimSpace(string(pubKey))

	sshAuthorizedKeys := executor.FindSSHAuthorizedKeysFile(exec)
	switch privilegeEscalationMethod {
	case Su:
		cmd = fmt.Sprintf(`su - %[1]s -c 'grep $(echo %[2]s) %[3]s || echo %[2]s >> %[3]s && chmod 600 %[3]s'`,
			e.deployUser, pk, sshAuthorizedKeys)
	case Runuser:
		cmd = fmt.Sprintf(`runuser -l %[1]s -c 'grep $(echo %[2]s) %[3]s || echo %[2]s >> %[3]s && chmod 600 %[3]s'`,
			e.deployUser, pk, sshAuthorizedKeys)
	case Sudo:
		cmd = fmt.Sprintf(`sudo grep $(echo %[1]s) %[2]s || sudo echo %[1]s >> %[2]s && chmod 600 %[2]s && sudo chown %[3]s:%[4]s %[2]s`,
			pk, sshAuthorizedKeys, e.deployUser, e.userGroup)
	}

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
