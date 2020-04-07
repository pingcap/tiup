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
	"time"

	"github.com/joomcode/errorx"
	"github.com/pingcap-incubator/tiup-cluster/pkg/executor"
)

var (
	errNS = errorx.NewNamespace("task")
)

// RootSSH is used to establish a SSH connection to the target host with specific key
type RootSSH struct {
	host       string // hostname of the SSH server
	port       int    // port of the SSH server
	user       string // username to login to the SSH server
	password   string // password of the user
	keyFile    string // path to the private key file
	passphrase string // passphrase of the private key file
	timeout    int64  // timeout in seconds when connecting via SSH
}

// Execute implements the Task interface
func (s *RootSSH) Execute(ctx *Context) error {
	e := executor.NewSSHExecutor(executor.SSHConfig{
		Host:       s.host,
		Port:       s.port,
		User:       s.user,
		Password:   s.password,
		KeyFile:    s.keyFile,
		Passphrase: s.passphrase,
		Timeout:    time.Second * time.Duration(s.timeout),
	})

	ctx.SetExecutor(s.host, e)
	return nil
}

// Rollback implements the Task interface
func (s *RootSSH) Rollback(ctx *Context) error {
	ctx.exec.Lock()
	delete(ctx.exec.executors, s.host)
	ctx.exec.Unlock()
	return nil
}

// String implements the fmt.Stringer interface
func (s RootSSH) String() string {
	if len(s.keyFile) > 0 {
		return fmt.Sprintf("RootSSH: user=%s, host=%s, port=%d, key=%s", s.user, s.host, s.port, s.keyFile)
	}
	return fmt.Sprintf("RootSSH: user=%s, host=%s, port=%d", s.user, s.host, s.port)
}

// UserSSH is used to establish a SSH connection to the target host with generated key
type UserSSH struct {
	host       string
	deployUser string
	timeout    int64
}

// Execute implements the Task interface
func (s *UserSSH) Execute(ctx *Context) error {
	e := executor.NewSSHExecutor(executor.SSHConfig{
		Host:    s.host,
		KeyFile: ctx.PrivateKeyPath,
		User:    s.deployUser,
		Timeout: time.Second * time.Duration(s.timeout),
	})

	ctx.SetExecutor(s.host, e)
	return nil
}

// Rollback implements the Task interface
func (s *UserSSH) Rollback(ctx *Context) error {
	ctx.exec.Lock()
	delete(ctx.exec.executors, s.host)
	ctx.exec.Unlock()
	return nil
}

// String implements the fmt.Stringer interface
func (s UserSSH) String() string {
	return fmt.Sprintf("UserSSH: user=%s, host=%s", s.deployUser, s.host)
}
