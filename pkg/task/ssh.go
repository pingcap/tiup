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
	"github.com/pingcap-incubator/tiops/pkg/executor"
	"github.com/pingcap/errors"
)

// RootSSH is used to establish a SSH connection to the target host with specific key
type RootSSH struct {
	host       string // hostname of the SSH server
	port       int    // port of the SSH server
	user       string // username to login to the SSH server
	password   string // password of the user
	keyFile    string // path to the private key file
	passphrase string // passphrase of the private key file
}

// Execute implements the Task interface
func (s RootSSH) Execute(ctx *Context) error {
	e, err := executor.NewSSHExecutor(executor.SSHConfig{
		Host:       s.host,
		Port:       s.port,
		User:       s.user,
		Password:   s.password,
		KeyFile:    s.keyFile,
		Passphrase: s.passphrase,
	})

	if err != nil {
		return errors.Trace(err)
	}

	ctx.SetExecutor(s.host, e)
	return nil
}

// Rollback implements the Task interface
func (s RootSSH) Rollback(ctx *Context) error {
	ctx.exec.Lock()
	delete(ctx.exec.executors, s.host)
	ctx.exec.Unlock()
	return nil
}

// UserSSH is used to establish a SSH connection to the target host with generated key
type UserSSH struct {
	host       string
	deployUser string
}

// Execute implements the Task interface
func (s UserSSH) Execute(ctx *Context) error {
	e, err := executor.NewSSHExecutor(executor.SSHConfig{
		Host:    s.host,
		KeyFile: ctx.PrivateKeyPath,
		User:    s.deployUser,
	})

	if err != nil {
		return errors.Trace(err)
	}

	ctx.SetExecutor(s.host, e)
	return nil
}

// Rollback implements the Task interface
func (s UserSSH) Rollback(ctx *Context) error {
	ctx.exec.Lock()
	delete(ctx.exec.executors, s.host)
	ctx.exec.Unlock()
	return nil
}
