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
	"strings"

	"github.com/c4pt0r/tiup/pkg/localdata"
	"github.com/c4pt0r/tiup/pkg/utils"
)

// PDInstance represent a running pd-server
type PDInstance struct {
	id         int
	dir        string
	peerPort   int
	clientPort int
	endpoints  []*PDInstance
	cmd        *exec.Cmd
}

// NewPDInstance return a PDInstance
func NewPDInstance(dir string, id int) *PDInstance {
	return &PDInstance{
		id:         id,
		dir:        dir,
		clientPort: utils.MustGetFreePort(2379),
		peerPort:   utils.MustGetFreePort(2380),
	}
}

// Join set endpoints field of PDInstance
func (inst *PDInstance) Join(pds []*PDInstance) *PDInstance {
	inst.endpoints = pds
	return inst
}

// Start calls set inst.cmd and Start
func (inst *PDInstance) Start() error {
	if err := os.MkdirAll(inst.dir, 0755); err != nil {
		return err
	}
	uid := fmt.Sprintf("pd-%d", inst.id)
	args := []string{
		"tiup", "run", "pd", "--",
		"--name=" + uid,
		fmt.Sprintf("--data-dir=%s", filepath.Join(inst.dir, "data")),
		fmt.Sprintf("--peer-urls=http://127.0.0.1:%d", inst.peerPort),
		fmt.Sprintf("--advertise-peer-urls=http://127.0.0.1:%d", inst.peerPort),
		fmt.Sprintf("--client-urls=http://127.0.0.1:%d", inst.clientPort),
		fmt.Sprintf("--advertise-client-urls=http://127.0.0.1:%d", inst.clientPort),
		fmt.Sprintf("--log-file=%s", filepath.Join(inst.dir, "pd.log")),
	}
	endpoints := make([]string, 0, len(inst.endpoints))
	for _, pd := range inst.endpoints {
		uid := fmt.Sprintf("pd-%d", pd.id)
		endpoints = append(endpoints, fmt.Sprintf("%s=http://127.0.0.1:%d", uid, pd.peerPort))
	}
	if len(endpoints) > 0 {
		args = append(args, fmt.Sprintf("--initial-cluster=%s", strings.Join(endpoints, ",")))
	}
	inst.cmd = exec.Command(args[0], args[1:]...)
	inst.cmd.Env = append(
		os.Environ(),
		fmt.Sprintf("%s=%s", localdata.EnvNameInstanceDataDir, inst.dir),
	)
	return inst.cmd.Start()
}

// Wait calls inst.cmd.Wait
func (inst *PDInstance) Wait() error {
	return inst.cmd.Wait()
}

// Pid return the PID of the instance
func (inst *PDInstance) Pid() int {
	return inst.cmd.Process.Pid
}

// Addr return the listen address of PD
func (inst *PDInstance) Addr() string {
	return fmt.Sprintf("127.0.0.1:%d", inst.clientPort)
}
