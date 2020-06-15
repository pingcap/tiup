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
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/pingcap/errors"
	"github.com/pingcap/tiup/pkg/localdata"
	"github.com/pingcap/tiup/pkg/repository/v0manifest"
	"github.com/pingcap/tiup/pkg/utils"
)

// PDInstance represent a running pd-server
type PDInstance struct {
	instance
	initEndpoints []*PDInstance
	joinEndpoints []*PDInstance
	cmd           *exec.Cmd
}

// NewPDInstance return a PDInstance
func NewPDInstance(binPath, dir, host, configPath string, id int) *PDInstance {
	return &PDInstance{
		instance: instance{
			BinPath:    binPath,
			ID:         id,
			Dir:        dir,
			Host:       host,
			Port:       utils.MustGetFreePort(host, 2380),
			StatusPort: utils.MustGetFreePort(host, 2379),
			ConfigPath: configPath,
		},
	}
}

// Join set endpoints field of PDInstance
func (inst *PDInstance) Join(pds []*PDInstance) *PDInstance {
	inst.joinEndpoints = pds
	return inst
}

// InitCluster set the init cluster instance.
func (inst *PDInstance) InitCluster(pds []*PDInstance) *PDInstance {
	inst.initEndpoints = pds
	return inst
}

// Name return the name of pd.
func (inst *PDInstance) Name() string {
	return fmt.Sprintf("pd-%d", inst.ID)
}

// Start calls set inst.cmd and Start
func (inst *PDInstance) Start(ctx context.Context, version v0manifest.Version) error {
	if err := os.MkdirAll(inst.Dir, 0755); err != nil {
		return err
	}
	uid := inst.Name()
	args := []string{
		"tiup", fmt.Sprintf("--binpath=%s", inst.BinPath),
		CompVersion("pd", version),
		"--name=" + uid,
		fmt.Sprintf("--data-dir=%s", filepath.Join(inst.Dir, "data")),
		fmt.Sprintf("--peer-urls=http://%s:%d", inst.Host, inst.Port),
		fmt.Sprintf("--advertise-peer-urls=http://%s:%d", advertiseHost(inst.Host), inst.Port),
		fmt.Sprintf("--client-urls=http://%s:%d", inst.Host, inst.StatusPort),
		fmt.Sprintf("--advertise-client-urls=http://%s:%d", advertiseHost(inst.Host), inst.StatusPort),
		fmt.Sprintf("--log-file=%s", filepath.Join(inst.Dir, "pd.log")),
	}
	if inst.ConfigPath != "" {
		args = append(args, fmt.Sprintf("--config=%s", inst.ConfigPath))
	}

	if len(inst.initEndpoints) > 0 {
		endpoints := make([]string, 0)
		for _, pd := range inst.initEndpoints {
			uid := fmt.Sprintf("pd-%d", pd.ID)
			endpoints = append(endpoints, fmt.Sprintf("%s=http://%s:%d", uid, advertiseHost(inst.Host), pd.Port))
		}
		args = append(args, fmt.Sprintf("--initial-cluster=%s", strings.Join(endpoints, ",")))
	} else if len(inst.joinEndpoints) > 0 {
		endpoints := make([]string, 0)
		for _, pd := range inst.joinEndpoints {
			endpoints = append(endpoints, fmt.Sprintf("http://%s:%d", advertiseHost(inst.Host), pd.Port))
		}
		args = append(args, fmt.Sprintf("--join=%s", strings.Join(endpoints, ",")))
	} else {
		return errors.Errorf("must set the init or join instances.")
	}

	inst.cmd = exec.CommandContext(ctx, args[0], args[1:]...)
	inst.cmd.Env = append(
		os.Environ(),
		fmt.Sprintf("%s=%s", localdata.EnvNameInstanceDataDir, inst.Dir),
	)
	inst.cmd.Stderr = os.Stderr
	inst.cmd.Stdout = os.Stdout
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
	return fmt.Sprintf("%s:%d", advertiseHost(inst.Host), inst.StatusPort)
}
