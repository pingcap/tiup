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
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
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
	}

	e := &NativeSSHExecutor{}
	for _, tc := range testcases {
		e.Config = tc.c
		assert.Equal(t, tc.e, strings.Join(e.configArgs([]string{}, tc.s), " "))
	}
}
