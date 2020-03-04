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
	"path"
	"path/filepath"
	"strings"

	"github.com/c4pt0r/tiup/pkg/localdata"
	"github.com/c4pt0r/tiup/pkg/utils"
	"github.com/pingcap/errors"
)

// TiKVInstance represent a running tikv-server
type TiKVInstance struct {
	id     int
	dir    string
	port   int
	status int
	pds    []*PDInstance
	cmd    *exec.Cmd
}

// NewTiKVInstance return a TiKVInstance
func NewTiKVInstance(dir string, id int, pds []*PDInstance) *TiKVInstance {
	return &TiKVInstance{
		id:     id,
		dir:    dir,
		port:   utils.MustGetFreePort(20160),
		status: utils.MustGetFreePort(20180),
		pds:    pds,
	}
}

// Start calls set inst.cmd and Start
func (inst *TiKVInstance) Start() error {
	if err := os.MkdirAll(inst.dir, 0755); err != nil {
		return err
	}
	configPath := path.Join(inst.dir, "tikv.toml")
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
		endpoints = append(endpoints, fmt.Sprintf("http://127.0.0.1:%d", pd.clientPort))
	}
	inst.cmd = exec.Command(
		"tiup", "run", "tikv", "--",
		fmt.Sprintf("--addr=127.0.0.1:%d", inst.port),
		fmt.Sprintf("--status-addr=127.0.0.1:%d", inst.status),
		fmt.Sprintf("--pd=%s", strings.Join(endpoints, ",")),
		fmt.Sprintf("--config=%s", configPath),
		fmt.Sprintf("--data-dir=%s", filepath.Join(inst.dir, "data")),
		fmt.Sprintf("--log-file=g%s", filepath.Join(inst.dir, "tikv.log")),
	)
	inst.cmd.Env = append(
		os.Environ(),
		fmt.Sprintf("%s=%s", localdata.EnvNameInstanceDataDir, inst.dir),
	)
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
