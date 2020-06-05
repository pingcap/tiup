package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/pingcap/errors"
	"github.com/pingcap/tiup/pkg/localdata"
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

	err = ioutil.WriteFile(m.sdFname, data, 0644)
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
}

func newMonitor() *monitor {
	return &monitor{}
}

func (m *monitor) startMonitor(ctx context.Context, host, dir string) (int, *exec.Cmd, error) {
	if err := os.MkdirAll(dir, 0755); err != nil {
		return 0, nil, err
	}

	port, err := utils.GetFreePort(host, 9090)
	if err != nil {
		return 0, nil, err
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

	m.sdFname = filepath.Join(dir, "targets.json")

	if err := ioutil.WriteFile(filepath.Join(dir, "prometheus.yml"), []byte(tmpl), os.ModePerm); err != nil {
		return 0, nil, err
	}

	args := []string{
		"tiup", "prometheus",
		fmt.Sprintf("--config.file=%s", filepath.Join(dir, "prometheus.yml")),
		fmt.Sprintf("--web.external-url=http://%s", addr),
		fmt.Sprintf("--web.listen-address=0.0.0.0:%d", port),
		fmt.Sprintf("--storage.tsdb.path='%s'", filepath.Join(dir, "data")),
	}
	cmd := exec.CommandContext(ctx, args[0], args[1:]...)
	cmd.Env = append(
		os.Environ(),
		fmt.Sprintf("%s=%s", localdata.EnvNameInstanceDataDir, dir),
	)

	m.port = port
	m.cmd = cmd
	m.host = host
	return port, cmd, nil
}
