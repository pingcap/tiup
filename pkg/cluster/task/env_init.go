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
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/joomcode/errorx"
	"github.com/pingcap/tiup/pkg/cluster/ctxt"
	"github.com/pingcap/tiup/pkg/cluster/executor"
	"github.com/pingcap/tiup/pkg/cluster/module"
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
	sudo           bool
}

// Execute implements the Task interface
func (e *EnvInit) Execute(ctx context.Context) error {
	return e.exec(ctx)
}

func (e *EnvInit) exec(ctx context.Context) error {
	wrapError := func(err error) *errorx.Error {
		return ErrEnvInitFailed.Wrap(err, "Failed to initialize TiDB environment on remote host '%s'", e.host)
	}

	exec, found := ctxt.GetInner(ctx).GetExecutor(e.host)
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

		_, _, errx := um.Execute(ctx, exec)
		if errx != nil {
			return wrapError(errx)
		}
	}

	pubKey, err := os.ReadFile(ctxt.GetInner(ctx).PublicKeyPath)
	if err != nil {
		return wrapError(err)
	}

	// Authorize
	var cmd string
	if e.sudo {
		cmd = `su - ` + e.deployUser + ` -c 'mkdir -p ~/.ssh && chmod 700 ~/.ssh'`
	} else {
		cmd = `mkdir -p ~/.ssh && chmod 700 ~/.ssh`
	}
	_, _, err = exec.Execute(ctx, cmd, e.sudo)
	if err != nil {
		return wrapError(errEnvInitSubCommandFailed.
			Wrap(err, "Failed to create '~/.ssh' directory for user '%s'", e.deployUser))
	}

	pk := strings.TrimSpace(string(pubKey))
	sshAuthorizedKeys := executor.FindSSHAuthorizedKeysFile(ctx, exec)
	if e.sudo {
		cmd = fmt.Sprintf(`su - %[1]s -c 'grep $(echo %[2]s) %[3]s || echo %[2]s >> %[3]s && chmod 600 %[3]s'`,
			e.deployUser, pk, sshAuthorizedKeys)
	} else {
		cmd = fmt.Sprintf(`grep $(echo %[1]s) %[2]s || echo %[1]s >> %[2]s && chmod 600 %[2]s`,
			pk, sshAuthorizedKeys)
	}

	_, _, err = exec.Execute(ctx, cmd, e.sudo)
	if err != nil {
		return wrapError(errEnvInitSubCommandFailed.
			Wrap(err, "Failed to write public keys to '%s' for user '%s'", sshAuthorizedKeys, e.deployUser))
	}

	return nil
}

// Rollback implements the Task interface
func (e *EnvInit) Rollback(ctx context.Context) error {
	return ErrUnsupportedRollback
}

// String implements the fmt.Stringer interface
func (e *EnvInit) String() string {
	return fmt.Sprintf("EnvInit: user=%s, host=%s", e.deployUser, e.host)
}
