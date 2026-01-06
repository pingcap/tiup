package proc

import (
	"context"
	"encoding/json"
	"fmt"
	"path/filepath"
	"slices"
	"strings"

	"github.com/pingcap/errors"
	"github.com/pingcap/tiup/pkg/utils"
)

const (
	// ServicePrometheus is the service ID for the Prometheus instance.
	ServicePrometheus ServiceID = "prometheus"

	// ComponentPrometheus is the repository component ID for Prometheus.
	ComponentPrometheus RepoComponentID = "prometheus"
)

func init() {
	RegisterComponentDisplayName(ComponentPrometheus, "Prometheus")
	RegisterServiceDisplayName(ServicePrometheus, "Prometheus")
}

// PrometheusInstance represents a running Prometheus server.
type PrometheusInstance struct {
	ProcessInfo

	sdFile string
}

var _ Process = &PrometheusInstance{}

// LogFile returns the log file path for the instance.
func (inst *PrometheusInstance) LogFile() string {
	return filepath.Join(inst.Dir, "prom.log")
}

// RenderSDFile writes Prometheus file_sd targets for all instances.
// ref: https://prometheus.io/docs/prometheus/latest/configuration/configuration/#file_sd_config
func (inst *PrometheusInstance) RenderSDFile(sid2targets map[ServiceID]MetricAddr) error {
	if inst == nil {
		return nil
	}

	if inst.sdFile == "" {
		inst.sdFile = filepath.Join(inst.Dir, "targets.json")
	}
	if sid2targets == nil {
		sid2targets = make(map[ServiceID]MetricAddr)
	}

	sid2targets[ServicePrometheus] = MetricAddr{Targets: []string{utils.JoinHostPort(inst.Host, inst.Port)}}

	var orderedIDs []ServiceID
	for id, t := range sid2targets {
		if len(t.Targets) == 0 {
			continue
		}
		orderedIDs = append(orderedIDs, id)
	}
	slices.SortStableFunc(orderedIDs, func(a, b ServiceID) int {
		return strings.Compare(a.String(), b.String())
	})

	items := make([]MetricAddr, 0, len(orderedIDs))
	for _, id := range orderedIDs {
		t := sid2targets[id]

		targets := append([]string(nil), t.Targets...)
		slices.Sort(targets)
		targets = slices.Compact(targets)

		it := MetricAddr{
			Targets: targets,
			Labels:  map[string]string{"job": id.String()},
		}
		for k, v := range t.Labels {
			it.Labels[k] = v
		}
		items = append(items, it)
	}

	data, err := json.MarshalIndent(&items, "", "\t")
	if err != nil {
		return errors.AddStack(err)
	}
	if err := utils.WriteFile(inst.sdFile, data, 0644); err != nil {
		return errors.AddStack(err)
	}
	return nil
}

// Prepare builds the Prometheus process command.
func (inst *PrometheusInstance) Prepare(ctx context.Context) error {
	if inst == nil {
		return errors.New("prometheus instance is nil")
	}
	if inst.Dir == "" {
		return errors.New("prometheus dir is empty")
	}
	if inst.BinPath == "" {
		return errors.New("prometheus binary not resolved")
	}
	if err := utils.MkdirAll(inst.Dir, 0755); err != nil {
		return errors.AddStack(err)
	}

	addr := utils.JoinHostPort(inst.Host, inst.Port)

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

	configPath := filepath.Join(inst.Dir, "prometheus.yml")
	if err := utils.WriteFile(configPath, []byte(tmpl), 0644); err != nil {
		return errors.AddStack(err)
	}
	inst.sdFile = filepath.Join(inst.Dir, "targets.json")
	_ = utils.WriteFile(inst.sdFile, []byte("[]"), 0644)

	args := []string{
		fmt.Sprintf("--config.file=%s", configPath),
		fmt.Sprintf("--web.external-url=http://%s", addr),
		fmt.Sprintf("--web.listen-address=%s", utils.JoinHostPort(inst.Host, inst.Port)),
		fmt.Sprintf("--storage.tsdb.path=%s", filepath.Join(inst.Dir, "data")),
	}
	inst.Proc = &cmdProcess{cmd: PrepareCommand(ctx, inst.BinPath, args, nil, inst.Dir)}
	return nil
}
