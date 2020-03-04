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

package instance

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/c4pt0r/tiup/pkg/localdata"
	"github.com/c4pt0r/tiup/pkg/utils"
)

// TiDBInstance represent a running tidb-server
type TiDBInstance struct {
	id     int
	dir    string
	port   int
	status int
	pds    []*PDInstance
	cmd    *exec.Cmd
}

// NewTiDBInstance return a TiDBInstance
func NewTiDBInstance(dir string, id int, pds []*PDInstance) *TiDBInstance {
	return &TiDBInstance{
		id:     id,
		dir:    dir,
		port:   utils.MustGetFreePort(4000),
		status: utils.MustGetFreePort(10080),
		pds:    pds,
	}
}

// Start calls set inst.cmd and Start
func (inst *TiDBInstance) Start() error {
	if err := os.MkdirAll(inst.dir, 0755); err != nil {
		return err
	}
	endpoints := make([]string, 0, len(inst.pds))
	for _, pd := range inst.pds {
		endpoints = append(endpoints, fmt.Sprintf("127.0.0.1:%d", pd.clientPort))
	}
	args := []string{
		"tiup", "run", "tidb", "--",
		"-P", strconv.Itoa(inst.port),
		fmt.Sprintf("--status=%d", inst.status),
		"--host=127.0.0.1",
		"--store=tikv",
		fmt.Sprintf("--path=%s", strings.Join(endpoints, ",")),
		fmt.Sprintf("--log-file=%s", filepath.Join(inst.dir, "tidb.log")),
	}
	inst.cmd = exec.Command(args[0], args[1:]...)
	inst.cmd.Env = append(
		os.Environ(),
		fmt.Sprintf("%s=%s", localdata.EnvNameInstanceDataDir, inst.dir),
	)
	return inst.cmd.Start()
}

// Wait calls inst.cmd.Wait
func (inst *TiDBInstance) Wait() error {
	return inst.cmd.Wait()
}

// Pid return the PID of the instance
func (inst *TiDBInstance) Pid() int {
	return inst.cmd.Process.Pid
}

// Addr return the listen address of TiDB
func (inst *TiDBInstance) Addr() string {
	return fmt.Sprintf("127.0.0.1:%d", inst.port)
}
