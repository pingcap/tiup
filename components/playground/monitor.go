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

package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sync"

	"github.com/pingcap/errors"
	"github.com/pingcap/tiup/components/playground/instance"
	tiupexec "github.com/pingcap/tiup/pkg/exec"
	"github.com/pingcap/tiup/pkg/utils"
)

// ref: https://prometheus.io/docs/prometheus/latest/configuration/configuration/#file_sd_config
func (m *monitor) renderSDFile(cid2targets map[string][]string) error {
	type Item struct {
		Targets []string          `json:"targets"`
		Labels  map[string]string `json:"labels"`
	}

	cid2targets["prometheus"] = []string{fmt.Sprintf("%s:%d", m.host, m.port)}

	var items []Item

	for id, targets := range cid2targets {
		item := Item{
			Targets: targets,
			Labels: map[string]string{
				"job": id,
			},
		}
		items = append(items, item)
	}

	data, err := json.MarshalIndent(&items, "", "\t")
	if err != nil {
		return errors.AddStack(err)
	}

	err = os.WriteFile(m.sdFname, data, 0644)
	if err != nil {
		return errors.AddStack(err)
	}

	return nil
}

type monitor struct {
	host string
	port int
	cmd  *exec.Cmd

	sdFname string

	waitErr  error
	waitOnce sync.Once
}

func (m *monitor) wait() error {
	m.waitOnce.Do(func() {
		m.waitErr = m.cmd.Wait()
	})

	return m.waitErr
}

// the cmd is not started after return
func newMonitor(ctx context.Context, version string, host, dir string) (*monitor, error) {
	if err := os.MkdirAll(dir, 0755); err != nil {
		return nil, errors.AddStack(err)
	}

	port, err := utils.GetFreePort(host, 9090)
	if err != nil {
		return nil, err
	}
	addr := fmt.Sprintf("%s:%d", host, port)

	tmpl := `
global:
  scrape_interval:     15s # Set the scrape interval to every 15 seconds. Default is every 1 minute.
  evaluation_interval: 15s # Evaluate rules every 15 seconds. The default is every 1 minute.
  # scrape_timeout is set to the global default (10s).

# Alertmanager configuration
alerting:
  alertmanagers:
  - static_configs:
    - targets:
      # - alertmanager:9093

# Load rules once and periodically evaluate them according to the global 'evaluation_interval'.
rule_files:
  # - "first_rules.yml"
  # - "second_rules.yml"

# A scrape configuration containing exactly one endpoint to scrape:
# Here it's Prometheus itself.
scrape_configs:
  - job_name: 'cluster'
    file_sd_configs:
    - files:
      - targets.json

`

	m := new(monitor)
	m.sdFname = filepath.Join(dir, "targets.json")

	if err := os.WriteFile(filepath.Join(dir, "prometheus.yml"), []byte(tmpl), os.ModePerm); err != nil {
		return nil, errors.AddStack(err)
	}

	args := []string{
		fmt.Sprintf("--config.file=%s", filepath.Join(dir, "prometheus.yml")),
		fmt.Sprintf("--web.external-url=http://%s", addr),
		fmt.Sprintf("--web.listen-address=%s:%d", host, port),
		fmt.Sprintf("--storage.tsdb.path=%s", filepath.Join(dir, "data")),
	}

	var binPath string
	if binPath, err = tiupexec.PrepareBinary("prometheus", utils.Version(version), binPath); err != nil {
		return nil, err
	}
	cmd := instance.PrepareCommand(ctx, binPath, args, nil, dir)

	m.port = port
	m.cmd = cmd
	m.host = host

	return m, nil
}
