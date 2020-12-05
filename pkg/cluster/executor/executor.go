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
	"net"
	"os"
	"strings"
	"time"

	"github.com/joomcode/errorx"
	"github.com/pingcap/errors"
	"github.com/pingcap/failpoint"
	"github.com/pingcap/tiup/pkg/localdata"
)

// SSHType represent the type of the chanel used by ssh
type SSHType string

var (
	errNS = errorx.NewNamespace("executor")

	// SSHTypeBuiltin is the type of easy ssh executor
	SSHTypeBuiltin SSHType = "builtin"

	// SSHTypeSystem is the type of host ssh client
	SSHTypeSystem SSHType = "system"

	// SSHTypeNone is the type of local executor (no ssh will be used)
	SSHTypeNone SSHType = "none"

	executeDefaultTimeout = time.Second * 60

	// This command will be execute once the NativeSSHExecutor is created.
	// It's used to predict if the connection can establish success in the future.
	// Its main purpose is to avoid sshpass hang when user speficied a wrong prompt.
	connectionTestCommand = "echo connection test, if killed, check the password prompt"

	// SSH authorized_keys file
	defaultSSHAuthorizedKeys = "~/.ssh/authorized_keys"
)

// Executor is the executor interface for TiUP, all tasks will in the end
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
func New(etype SSHType, sudo bool, c SSHConfig) (Executor, error) {
	if etype == "" {
		etype = SSHTypeBuiltin
	}

	// Used in integration testing, to check if native ssh client is really used when it need to be.
	failpoint.Inject("assertNativeSSH", func() {
		// XXX: We call system executor 'native' by mistake in commit f1142b1
		// this should be fixed after we remove --native-ssh flag
		if etype != SSHTypeSystem {
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
	case SSHTypeBuiltin:
		e := &EasySSHExecutor{
			Locale: "C",
			Sudo:   sudo,
		}
		e.initialize(c)
		return e, nil
	case SSHTypeSystem:
		e := &NativeSSHExecutor{
			Config: &c,
			Locale: "C",
			Sudo:   sudo,
		}
		if c.Password != "" || (c.KeyFile != "" && c.Passphrase != "") {
			_, _, e.ConnectionTestResult = e.Execute(connectionTestCommand, false, executeDefaultTimeout)
		}
		return e, nil
	case SSHTypeNone:
		if err := checkLocalIP(c.Host); err != nil {
			return nil, err
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

func checkLocalIP(ip string) error {
	ifaces, err := net.Interfaces()
	if err != nil {
		return errors.AddStack(err)
	}

	foundIps := []string{}
	for _, i := range ifaces {
		addrs, err := i.Addrs()
		if err != nil {
			continue
		}

		for _, addr := range addrs {
			switch v := addr.(type) {
			case *net.IPNet:
				if ip == v.IP.String() {
					return nil
				}
				foundIps = append(foundIps, v.IP.String())
			case *net.IPAddr:
				if ip == v.IP.String() {
					return nil
				}
				foundIps = append(foundIps, v.IP.String())
			}
		}
	}

	return fmt.Errorf("address %s not found in all interfaces, found ips: %s", ip, strings.Join(foundIps, ","))
}

// FindSSHAuthorizedKeysFile finds the correct path of SSH authorized keys file
func FindSSHAuthorizedKeysFile(exec Executor) string {
	// detect if custom path of authorized keys file is set
	// NOTE: we do not yet support:
	//   - custom config for user (~/.ssh/config)
	//   - sshd started with custom config (other than /etc/ssh/sshd_config)
	//   - ssh server implementations other than OpenSSH (such as dropbear)
	sshAuthorizedKeys := defaultSSHAuthorizedKeys
	cmd := "grep -Ev '^\\s*#|^\\s*$' /etc/ssh/sshd_config"
	stdout, _, _ := exec.Execute(cmd, true) // error ignored as we have default value
	for _, line := range strings.Split(string(stdout), "\n") {
		if !strings.Contains(line, "AuthorizedKeysFile") {
			continue
		}
		fields := strings.Fields(line)
		if len(fields) >= 2 {
			sshAuthorizedKeys = fields[1]
			break
		}
	}

	if !strings.HasPrefix(sshAuthorizedKeys, "/") && !strings.HasPrefix(sshAuthorizedKeys, "~") {
		sshAuthorizedKeys = fmt.Sprintf("~/%s", sshAuthorizedKeys)
	}
	return sshAuthorizedKeys
}
