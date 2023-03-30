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
)

// RotateSSH is used to rotate ssh key, e.g:
// 1. enable new public key
// 2. revoke old public key
type RotateSSH struct {
	host             string
	deployUser       string
	newPublicKeyPath string
}

// Execute implements the Task interface
func (e *RotateSSH) Execute(ctx context.Context) error {
	return e.exec(ctx)
}

func (e *RotateSSH) exec(ctx context.Context) error {
	wrapError := func(err error) *errorx.Error {
		return ErrEnvInitFailed.Wrap(err, "Failed to Rotate ssh public key on remote host '%s'", e.host)
	}

	exec, found := ctxt.GetInner(ctx).GetExecutor(e.host)
	if !found {
		panic(ErrNoExecutor)
	}

	pubKey, err := os.ReadFile(ctxt.GetInner(ctx).PublicKeyPath)
	if err != nil {
		return wrapError(err)
	}

	newPubKey, err := os.ReadFile(e.newPublicKeyPath)
	if err != nil {
		return wrapError(err)
	}

	sshAuthorizedKeys := executor.FindSSHAuthorizedKeysFile(ctx, exec)

	// enable new key
	cmd := fmt.Sprintf(`echo %s >> %s`, strings.TrimSpace(string(newPubKey)), sshAuthorizedKeys)
	_, _, err = exec.Execute(ctx, cmd, false)
	if err != nil {
		return wrapError(errEnvInitSubCommandFailed.
			Wrap(err, "Failed to write new public key to '%s' for user '%s'", sshAuthorizedKeys, e.deployUser))
	}

	// Revoke old key
	cmd = fmt.Sprintf(`sed -i '\|%[1]s|d' %[2]s`,
		strings.TrimSpace(string(pubKey)), sshAuthorizedKeys)

	_, _, err = exec.Execute(ctx, cmd, false)
	if err != nil {
		return wrapError(errEnvInitSubCommandFailed.
			Wrap(err, "Failed to revoke old key for user '%s'", e.deployUser))
	}

	return nil
}

// Rollback implements the Task interface
func (e *RotateSSH) Rollback(ctx context.Context) error {
	return ErrUnsupportedRollback
}

// String implements the fmt.Stringer interface
func (e *RotateSSH) String() string {
	return fmt.Sprintf("RotateSSH: user=%s, host=%s", e.deployUser, e.host)
}
