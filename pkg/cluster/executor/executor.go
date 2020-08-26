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
	"time"

	"github.com/joomcode/errorx"
	"github.com/pingcap/failpoint"
	"github.com/pingcap/tiup/pkg/localdata"
)

var (
	errNS = errorx.NewNamespace("executor")

	// ExecutorTypeBuiltin is the type of easy ssh executor
	ExecutorTypeBuiltin = "builtin"

	// ExecutorTypeSystem is the type of host ssh client
	ExecutorTypeSystem = "system"

	// ExecutorTypeNone is the type of local executor (no ssh will be used)
	ExecutorTypeNone = "none"

	executeDefaultTimeout = time.Second * 60

	// This command will be execute once the NativeSSHExecutor is created.
	// It's used to predict if the connection can establish success in the future.
	// Its main purpose is to avoid sshpass hang when user speficied a wrong prompt.
	connectionTestCommand = "echo connection test, if killed, check the password prompt"

	defaultLocalIP = "127.0.0.1"
)

// Executor is the executor interface for TiOps, all tasks will in the end
// be passed to a executor and then be actually performed.
type Executor interface {
	// Execute run the command, then return it's stdout and stderr
	// NOTE: stdin is not supported as it seems we don't need it (for now). If
	// at some point in the future we need to pass stdin to a command, we'll
	// need to refactor this function and its implementations.
	// If the cmd can't quit in timeout, it will return error, the default timeout is 60 seconds.
	Execute(cmd string, sudo bool, timeout ...time.Duration) (stdout []byte, stderr []byte, err error)

	// Transfer copies files from or to a target
	Transfer(src string, dst string, download bool) error
}

// New create a new Executor
func New(etype string, sudo bool, c SSHConfig) (Executor, error) {
	// Used in integration testing, to check if native ssh client is really used when it need to be.
	failpoint.Inject("assertNativeSSH", func() {
		// XXX: We call system executor 'native' by mistake in commit f1142b1
		// this should be fixed after we remove --native-ssh flag
		if etype != ExecutorTypeSystem {
			msg := fmt.Sprintf(
				"native ssh client should be used in this case, os.Args: %s, %s = %s",
				os.Args, localdata.EnvNameNativeSSHClient, os.Getenv(localdata.EnvNameNativeSSHClient),
			)
			panic(msg)
		}
	})

	// set default values
	if c.Port <= 0 {
		c.Port = 22
	}

	if c.Timeout == 0 {
		c.Timeout = time.Second * 5 // default timeout is 5 sec
	}

	switch etype {
	case ExecutorTypeBuiltin:
		e := &EasySSHExecutor{
			Locale: "C",
			Sudo:   sudo,
		}
		e.initialize(c)
		return e, nil
	case ExecutorTypeSystem:
		e := &NativeSSHExecutor{
			Config: &c,
			Locale: "C",
			Sudo:   sudo,
		}
		if c.Password != "" || (c.KeyFile != "" && c.Passphrase != "") {
			_, _, e.ConnectionTestResult = e.Execute(connectionTestCommand, false, executeDefaultTimeout)
		}
		return e, nil
	case ExecutorTypeNone:
		local := os.Getenv(localdata.EnvNameLocalHost)
		if local == "" {
			local = defaultLocalIP
		}
		if c.Host != local {
			return nil, fmt.Errorf(`you are trying to connect ip address %s
this is not the local address %s
if you have another ip address, you can set it with %s = <your-local-ip>`, c.Host, local, localdata.EnvNameLocalHost)
		}
		e := &Local{
			Config: &c,
			Sudo:   sudo,
			Locale: "C",
		}
		return e, nil
	default:
		return nil, fmt.Errorf("unregistered executor: %s", etype)
	}
}
