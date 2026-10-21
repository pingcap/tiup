// Copyright 2021 PingCAP, Inc.
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
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/pingcap/tiup/pkg/localdata"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNativeSSHConfigArgs(t *testing.T) {
	testcases := []struct {
		c *SSHConfig
		s bool
		e string
	}{
		{
			&SSHConfig{
				KeyFile: "id_rsa",
			},
			false,
			"-i id_rsa",
		},
		{
			&SSHConfig{
				Timeout: 60 * time.Second,
				Port:    23,
				KeyFile: "id_rsa",
			},
			false,
			"-p 23 -o ConnectTimeout=60 -i id_rsa",
		},
		{
			&SSHConfig{
				Timeout: 60 * time.Second,
				Port:    23,
				KeyFile: "id_rsa",
			},
			true,
			"-P 23 -o ConnectTimeout=60 -i id_rsa",
		},
		{
			&SSHConfig{
				Timeout:    60 * time.Second,
				KeyFile:    "id_rsa",
				Port:       23,
				Passphrase: "tidb",
			},
			false,
			"sshpass -p tidb -P passphrase -p 23 -o ConnectTimeout=60 -i id_rsa",
		},
		{
			&SSHConfig{
				Timeout:    60 * time.Second,
				KeyFile:    "id_rsa",
				Port:       23,
				Passphrase: "tidb",
			},
			true,
			"sshpass -p tidb -P passphrase -P 23 -o ConnectTimeout=60 -i id_rsa",
		},
		{
			&SSHConfig{
				Timeout:  60 * time.Second,
				Password: "tidb",
			},
			true,
			"sshpass -p tidb -P password -o ConnectTimeout=60",
		},
		{
			&SSHConfig{
				Timeout: 60 * time.Second,
				KeyFile: "id_rsa",
				Proxy: &SSHConfig{
					User:    "root",
					Host:    "proxy1",
					Port:    222,
					KeyFile: "b.id_rsa",
				},
			},
			false,
			"-o ConnectTimeout=60 -i id_rsa -o ProxyCommand=ssh -i b.id_rsa root@proxy1 -p 222 -W %h:%p",
		},
		{
			&SSHConfig{
				Timeout: 60 * time.Second,
				Port:    1203,
				KeyFile: "id_rsa",
				Proxy: &SSHConfig{
					User:    "root",
					Host:    "proxy1",
					Port:    222,
					KeyFile: "b.id_rsa",
					Timeout: 10 * time.Second,
				},
			},
			false,
			"-p 1203 -o ConnectTimeout=60 -i id_rsa -o ProxyCommand=ssh -o ConnectTimeout=10 -i b.id_rsa root@proxy1 -p 222 -W %h:%p",
		},
		{
			&SSHConfig{
				Timeout:  60 * time.Second,
				Password: "pass",
				Proxy: &SSHConfig{
					User:     "root",
					Host:     "proxy1",
					Port:     222,
					Password: "word",
					Timeout:  10 * time.Second,
				},
			},
			false,
			"sshpass -p pass -P password -o ConnectTimeout=60 -o ProxyCommand=sshpass -p word -P password ssh -o ConnectTimeout=10 root@proxy1 -p 222 -W %h:%p",
		},
	}

	e := &NativeSSHExecutor{}
	for _, tc := range testcases {
		e.Config = tc.c
		assert.Equal(t, tc.e, strings.Join(e.configArgs([]string{}, tc.s), " "))
	}
}

func TestNativeSSHTransferRemoteAddress(t *testing.T) {
	dir := t.TempDir()
	argsFile := filepath.Join(dir, "args")
	// fake scp that records its arguments
	scp := filepath.Join(dir, "scp")
	require.NoError(t, os.WriteFile(scp, []byte("#!/bin/sh\necho \"$@\" > "+argsFile+"\n"), 0o755))
	t.Setenv(localdata.EnvNameSCPPath, scp)

	testcases := []struct {
		host     string
		download bool
		e        string
	}{
		{"192.168.1.1", false, "/src tidb@192.168.1.1:/dst"},
		{"192.168.1.1", true, "tidb@192.168.1.1:/src " + filepath.Join(dir, "dst")},
		{"fdbd:dc0c:1:121a::51", false, "/src tidb@[fdbd:dc0c:1:121a::51]:/dst"},
		{"fdbd:dc0c:1:121a::51", true, "tidb@[fdbd:dc0c:1:121a::51]:/src " + filepath.Join(dir, "dst")},
	}

	for _, tc := range testcases {
		e := &NativeSSHExecutor{Config: &SSHConfig{User: "tidb", Host: tc.host}}
		dst := "/dst"
		if tc.download {
			dst = filepath.Join(dir, "dst")
		}
		require.NoError(t, e.Transfer(context.Background(), "/src", dst, tc.download, 0, false))
		args, err := os.ReadFile(argsFile)
		require.NoError(t, err)
		require.True(t, strings.HasSuffix(strings.TrimSpace(string(args)), tc.e), string(args))
	}
}
