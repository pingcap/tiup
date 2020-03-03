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
	"strings"

	"github.com/c4pt0r/tiup/pkg/utils"
)

// PDInstance represent a running pd-server
type PDInstance struct {
	id         int
	peerPort   int
	clientPort int
	endpoints  []*PDInstance
	cmd        *exec.Cmd
}

// NewPDInstance return a PDInstance
func NewPDInstance(id int) *PDInstance {
	return &PDInstance{
		id:         id,
		clientPort: utils.MustGetFreePort("127.0.0.1", 2379),
		peerPort:   utils.MustGetFreePort("127.0.0.1", 2380),
	}
}

// Join set endpoints field of PDInstance
func (inst *PDInstance) Join(pds []*PDInstance) *PDInstance {
	inst.endpoints = pds
	return inst
}

// Start calls set inst.cmd and Start
func (inst *PDInstance) Start() error {
	uid := fmt.Sprintf("%s-pd-%d", os.Getenv("TIUP_INSTANCE"), inst.id)
	args := []string{
		"tiup", "run", "--name=" + uid, "pd",
		"--name=" + uid,
		fmt.Sprintf("--peer-urls=http://127.0.0.1:%d", inst.peerPort),
		fmt.Sprintf("--advertise-peer-urls=http://127.0.0.1:%d", inst.peerPort),
		fmt.Sprintf("--client-urls=http://127.0.0.1:%d", inst.clientPort),
		fmt.Sprintf("--advertise-client-urls=http://127.0.0.1:%d", inst.clientPort),
		fmt.Sprintf("--log-file=%s.log", uid),
	}
	endpoints := []string{}
	for _, pd := range inst.endpoints {
		uid := fmt.Sprintf("%s-pd-%d", os.Getenv("TIUP_INSTANCE"), pd.id)
		endpoints = append(endpoints, fmt.Sprintf("%s=http://127.0.0.1:%d", uid, pd.peerPort))
	}
	if len(endpoints) > 0 {
		args = append(args, fmt.Sprintf("--initial-cluster=%s", strings.Join(endpoints, ",")))
	}
	inst.cmd = exec.Command(args[0], args[1:]...)
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
