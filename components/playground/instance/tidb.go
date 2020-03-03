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
	"strconv"
	"strings"

	"github.com/c4pt0r/tiup/pkg/utils"
)

// TiDBInstance represent a running tidb-server
type TiDBInstance struct {
	id     int
	port   int
	status int
	pds    []*PDInstance
	cmd    *exec.Cmd
}

// NewTiDBInstance return a TiDBInstance
func NewTiDBInstance(id int, pds []*PDInstance) *TiDBInstance {
	return &TiDBInstance{
		id:     id,
		port:   utils.MustGetFreePort("127.0.0.1", 4000),
		status: utils.MustGetFreePort("127.0.0.1", 10080),
		pds:    pds,
	}
}

// Start calls set inst.cmd and Start
func (inst *TiDBInstance) Start() error {
	endpoints := []string{}
	for _, pd := range inst.pds {
		endpoints = append(endpoints, fmt.Sprintf("127.0.0.1:%d", pd.clientPort))
	}
	args := []string{
		"tiup", "run", fmt.Sprintf("--name=%s-tidb-%d", os.Getenv("TIUP_INSTANCE"), inst.id), "tidb",
		"-P", strconv.Itoa(inst.port),
		fmt.Sprintf("--status=%d", inst.status),
		"--host=127.0.0.1",
		"--store=tikv",
		fmt.Sprintf("--path=%s", strings.Join(endpoints, ",")),
	}
	inst.cmd = exec.Command(args[0], args[1:]...)
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
