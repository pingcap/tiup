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

package main

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"

	"github.com/pingcap/errors"
	"github.com/pingcap/tiup/components/playground/instance"
	"github.com/pingcap/tiup/pkg/environment"
	"github.com/pingcap/tiup/pkg/utils"
)

type ngMonitoring struct {
	host string
	port int
	cmd  *exec.Cmd

	waitErr  error
	waitOnce sync.Once
}

func (m *ngMonitoring) wait() error {
	m.waitOnce.Do(func() {
		m.waitErr = m.cmd.Wait()
	})

	return m.waitErr
}

// the cmd is not started after return
func newNGMonitoring(ctx context.Context, version string, host, dir string, pds []*instance.PDInstance) (*ngMonitoring, error) {
	if err := os.MkdirAll(dir, 0755); err != nil {
		return nil, errors.AddStack(err)
	}

	port, err := utils.GetFreePort(host, 12020)
	if err != nil {
		return nil, err
	}

	m := new(ngMonitoring)
	var endpoints []string
	for _, pd := range pds {
		endpoints = append(endpoints, fmt.Sprintf("%s:%d", pd.Host, pd.StatusPort))
	}
	args := []string{
		fmt.Sprintf("--pd.endpoints=%s", strings.Join(endpoints, ",")),
		fmt.Sprintf("--address=%s:%d", host, port),
		fmt.Sprintf("--advertise-address=%s:%d", host, port),
		fmt.Sprintf("--storage.path=%s", filepath.Join(dir, "data")),
		fmt.Sprintf("--log.path=%s", filepath.Join(dir, "logs")),
	}

	env := environment.GlobalEnv()
	binDir, err := env.Profile().ComponentInstalledPath("prometheus", utils.Version(version))
	if err != nil {
		return nil, err
	}

	cmd := instance.PrepareCommand(ctx, filepath.Join(binDir, "ng-monitoring-server"), args, nil, dir)

	m.port = port
	m.cmd = cmd
	m.host = host

	return m, nil
}
