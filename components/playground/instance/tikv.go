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
	"path"
	"path/filepath"
	"strings"

	"github.com/pingcap-incubator/tiup/pkg/localdata"
	"github.com/pingcap-incubator/tiup/pkg/repository"
	"github.com/pingcap-incubator/tiup/pkg/utils"
	"github.com/pingcap/errors"
)

// TiKVInstance represent a running tikv-server
type TiKVInstance struct {
	instance
	pds []*PDInstance
	cmd *exec.Cmd
}

// NewTiKVInstance return a TiKVInstance
func NewTiKVInstance(dir, host string, id int, pds []*PDInstance) *TiKVInstance {
	return &TiKVInstance{
		instance: instance{
			ID:         id,
			Dir:        dir,
			Host:       host,
			Port:       utils.MustGetFreePort(host, 20160),
			StatusPort: utils.MustGetFreePort(host, 20180),
		},
		pds: pds,
	}
}

// Start calls set inst.cmd and Start
func (inst *TiKVInstance) Start(ctx context.Context, version repository.Version) error {
	if err := os.MkdirAll(inst.Dir, 0755); err != nil {
		return err
	}
	configPath := path.Join(inst.Dir, "tikv.toml")
	cf, err := os.Create(configPath)
	if err != nil {
		return errors.Trace(err)
	}
	defer cf.Close()
	if err := writeConfig(cf); err != nil {
		return errors.Trace(err)
	}

	endpoints := make([]string, 0, len(inst.pds))
	for _, pd := range inst.pds {
		endpoints = append(endpoints, fmt.Sprintf("http://%s:%d", inst.Host, pd.StatusPort))
	}
	inst.cmd = exec.CommandContext(ctx,
		"tiup", compVersion("tikv", version),
		fmt.Sprintf("--addr=%s:%d", inst.Host, inst.Port),
		fmt.Sprintf("--status-addr=%s:%d", inst.Host, inst.StatusPort),
		fmt.Sprintf("--pd=%s", strings.Join(endpoints, ",")),
		fmt.Sprintf("--config=%s", configPath),
		fmt.Sprintf("--data-dir=%s", filepath.Join(inst.Dir, "data")),
		fmt.Sprintf("--log-file=%s", filepath.Join(inst.Dir, "tikv.log")),
	)
	inst.cmd.Env = append(
		os.Environ(),
		fmt.Sprintf("%s=%s", localdata.EnvNameInstanceDataDir, inst.Dir),
	)
	inst.cmd.Stderr = os.Stderr
	inst.cmd.Stdout = os.Stdout
	return inst.cmd.Start()
}

// Wait calls inst.cmd.Wait
func (inst *TiKVInstance) Wait() error {
	return inst.cmd.Wait()
}

// Pid return the PID of the instance
func (inst *TiKVInstance) Pid() int {
	return inst.cmd.Process.Pid
}
