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

package spec

import (
	"crypto/tls"
	"fmt"
	"path"
	"path/filepath"
	"reflect"
	"strings"

	"github.com/pingcap/errors"
	"github.com/pingcap/tiup/pkg/cluster/executor"
	"github.com/pingcap/tiup/pkg/cluster/template/config"
	"github.com/pingcap/tiup/pkg/cluster/template/scripts"
	"github.com/pingcap/tiup/pkg/meta"
	"github.com/pingcap/tiup/pkg/set"
)

// PrometheusSpec represents the Prometheus Server topology specification in topology.yaml
type PrometheusSpec struct {
	Host            string               `yaml:"host"`
	SSHPort         int                  `yaml:"ssh_port,omitempty" validate:"ssh_port:editable"`
	Imported        bool                 `yaml:"imported,omitempty"`
	Port            int                  `yaml:"port" default:"9090"`
	DeployDir       string               `yaml:"deploy_dir,omitempty"`
	DataDir         string               `yaml:"data_dir,omitempty"`
	LogDir          string               `yaml:"log_dir,omitempty"`
	NumaNode        string               `yaml:"numa_node,omitempty" validate:"numa_node:editable"`
	Retention       string               `yaml:"storage_retention,omitempty" validate:"storage_retention:editable"`
	ResourceControl meta.ResourceControl `yaml:"resource_control,omitempty" validate:"resource_control:editable"`
	Arch            string               `yaml:"arch,omitempty"`
	OS              string               `yaml:"os,omitempty"`
	RuleDir         string               `yaml:"rule_dir,omitempty" validate:"rule_dir:editable"`
}

// Role returns the component role of the instance
func (s PrometheusSpec) Role() string {
	return ComponentPrometheus
}

// SSH returns the host and SSH port of the instance
func (s PrometheusSpec) SSH() (string, int) {
	return s.Host, s.SSHPort
}

// GetMainPort returns the main port of the instance
func (s PrometheusSpec) GetMainPort() int {
	return s.Port
}

// IsImported returns if the node is imported from TiDB-Ansible
func (s PrometheusSpec) IsImported() bool {
	return s.Imported
}

// MonitorComponent represents Monitor component.
type MonitorComponent struct{ Topology }

// Name implements Component interface.
func (c *MonitorComponent) Name() string {
	return ComponentPrometheus
}

// Role implements Component interface.
func (c *MonitorComponent) Role() string {
	return RoleMonitor
}

// Instances implements Component interface.
func (c *MonitorComponent) Instances() []Instance {
	servers := c.BaseTopo().Monitors
	ins := make([]Instance, 0, len(servers))

	for _, s := range servers {
		ins = append(ins, &MonitorInstance{BaseInstance{
			InstanceSpec: s,
			Name:         c.Name(),
			Host:         s.Host,
			Port:         s.Port,
			SSHP:         s.SSHPort,

			Ports: []int{
				s.Port,
			},
			Dirs: []string{
				s.DeployDir,
				s.DataDir,
			},
			StatusFn: func(_ *tls.Config, _ ...string) string {
				return "-"
			},
		}, c.Topology})
	}
	return ins
}

// MonitorInstance represent the monitor instance
type MonitorInstance struct {
	BaseInstance
	topo Topology
}

// InitConfig implement Instance interface
func (i *MonitorInstance) InitConfig(
	e executor.Executor,
	clusterName,
	clusterVersion,
	deployUser string,
	paths meta.DirPaths,
) error {
	gOpts := *i.topo.BaseTopo().GlobalOptions
	if err := i.BaseInstance.InitConfig(e, gOpts, deployUser, paths); err != nil {
		return err
	}

	enableTLS := gOpts.TLSEnabled
	// transfer run script
	spec := i.InstanceSpec.(PrometheusSpec)
	cfg := scripts.NewPrometheusScript(
		i.GetHost(),
		paths.Deploy,
		paths.Data[0],
		paths.Log,
	).WithPort(spec.Port).
		WithNumaNode(spec.NumaNode).
		WithRetention(spec.Retention)
	fp := filepath.Join(paths.Cache, fmt.Sprintf("run_prometheus_%s_%d.sh", i.GetHost(), i.GetPort()))
	if err := cfg.ConfigToFile(fp); err != nil {
		return err
	}

	dst := filepath.Join(paths.Deploy, "scripts", "run_prometheus.sh")
	if err := e.Transfer(fp, dst, false); err != nil {
		return err
	}

	if _, _, err := e.Execute("chmod +x "+dst, false); err != nil {
		return err
	}

	topoHasField := func(field string) (reflect.Value, bool) {
		return findSliceField(i.topo, field)
	}
	monitoredOptions := i.topo.GetMonitoredOptions()

	// transfer config
	fp = filepath.Join(paths.Cache, fmt.Sprintf("prometheus_%s_%d.yml", i.GetHost(), i.GetPort()))
	cfig := config.NewPrometheusConfig(clusterName, enableTLS)
	if monitoredOptions != nil {
		cfig.AddBlackbox(i.GetHost(), uint64(monitoredOptions.BlackboxExporterPort))
	}
	uniqueHosts := set.NewStringSet()

	if servers, found := topoHasField("PDServers"); found {
		for i := 0; i < servers.Len(); i++ {
			pd := servers.Index(i).Interface().(PDSpec)
			uniqueHosts.Insert(pd.Host)
			cfig.AddPD(pd.Host, uint64(pd.ClientPort))
		}
	}
	if servers, found := topoHasField("TiKVServers"); found {
		for i := 0; i < servers.Len(); i++ {
			kv := servers.Index(i).Interface().(TiKVSpec)
			uniqueHosts.Insert(kv.Host)
			cfig.AddTiKV(kv.Host, uint64(kv.StatusPort))
		}
	}
	if servers, found := topoHasField("TiDBServers"); found {
		for i := 0; i < servers.Len(); i++ {
			db := servers.Index(i).Interface().(TiDBSpec)
			uniqueHosts.Insert(db.Host)
			cfig.AddTiDB(db.Host, uint64(db.StatusPort))
		}
	}
	if servers, found := topoHasField("TiFlashServers"); found {
		for i := 0; i < servers.Len(); i++ {
			flash := servers.Index(i).Interface().(TiFlashSpec)
			cfig.AddTiFlashLearner(flash.Host, uint64(flash.FlashProxyStatusPort))
			cfig.AddTiFlash(flash.Host, uint64(flash.StatusPort))
		}
	}
	if servers, found := topoHasField("PumpServers"); found {
		for i := 0; i < servers.Len(); i++ {
			pump := servers.Index(i).Interface().(PumpSpec)
			uniqueHosts.Insert(pump.Host)
			cfig.AddPump(pump.Host, uint64(pump.Port))
		}
	}
	if servers, found := topoHasField("Drainers"); found {
		for i := 0; i < servers.Len(); i++ {
			drainer := servers.Index(i).Interface().(DrainerSpec)
			uniqueHosts.Insert(drainer.Host)
			cfig.AddDrainer(drainer.Host, uint64(drainer.Port))
		}
	}
	if servers, found := topoHasField("CDCServers"); found {
		for i := 0; i < servers.Len(); i++ {
			cdc := servers.Index(i).Interface().(CDCSpec)
			uniqueHosts.Insert(cdc.Host)
			cfig.AddCDC(cdc.Host, uint64(cdc.Port))
		}
	}
	if servers, found := topoHasField("Grafana"); found {
		for i := 0; i < servers.Len(); i++ {
			grafana := servers.Index(i).Interface().(GrafanaSpec)
			uniqueHosts.Insert(grafana.Host)
			cfig.AddGrafana(grafana.Host, uint64(grafana.Port))
		}
	}
	if servers, found := topoHasField("Alertmanager"); found {
		for i := 0; i < servers.Len(); i++ {
			alertmanager := servers.Index(i).Interface().(AlertmanagerSpec)
			uniqueHosts.Insert(alertmanager.Host)
			cfig.AddAlertmanager(alertmanager.Host, uint64(alertmanager.WebPort))
		}
	}
	if servers, found := topoHasField("Masters"); found {
		for i := 0; i < servers.Len(); i++ {
			master := servers.Index(i)
			host, port := master.FieldByName("Host").String(), master.FieldByName("Port").Int()
			cfig.AddDMMaster(host, uint64(port))
		}
	}

	if servers, found := topoHasField("Workers"); found {
		for i := 0; i < servers.Len(); i++ {
			master := servers.Index(i)
			host, port := master.FieldByName("Host").String(), master.FieldByName("Port").Int()
			cfig.AddDMWorker(host, uint64(port))
		}
	}
	if monitoredOptions != nil {
		for host := range uniqueHosts {
			cfig.AddNodeExpoertor(host, uint64(monitoredOptions.NodeExporterPort))
			cfig.AddBlackboxExporter(host, uint64(monitoredOptions.BlackboxExporterPort))
			cfig.AddMonitoredServer(host)
		}
	}

	if err := i.installRules(e, paths.Deploy, clusterVersion); err != nil {
		return errors.Annotate(err, "install rules")
	}

	if err := i.initRules(e, spec, paths); err != nil {
		return err
	}

	if err := cfig.ConfigToFile(fp); err != nil {
		return err
	}
	dst = filepath.Join(paths.Deploy, "conf", "prometheus.yml")
	return e.Transfer(fp, dst, false)
}

// We only really installRules for dm cluster because the rules(*.rules.yml) packed with the prometheus
// component is designed for tidb cluster (the dm cluster use the same prometheus component with tidb
// cluster), and the rules for dm cluster is packed in the dm-master component. So if deploying tidb
// cluster, the rules is correct, if deploying dm cluster, we should remove rules for tidb and install
// rules for dm.
func (i *MonitorInstance) installRules(e executor.Executor, deployDir, clusterVersion string) error {
	if i.topo.Type() != TopoTypeDM {
		return nil
	}

	tmp := filepath.Join(deployDir, "_tiup_tmp")
	_, stderr, err := e.Execute(fmt.Sprintf("mkdir -p %s", tmp), false)
	if err != nil {
		return errors.Annotatef(err, "stderr: %s", string(stderr))
	}

	srcPath := PackagePath(ComponentDMMaster, clusterVersion, i.OS(), i.Arch())
	dstPath := filepath.Join(tmp, filepath.Base(srcPath))

	err = e.Transfer(srcPath, dstPath, false)
	if err != nil {
		return err
	}

	cmd := fmt.Sprintf(`tar --no-same-owner -zxf %s -C %s && rm %s`, dstPath, tmp, dstPath)
	_, stderr, err = e.Execute(cmd, false)
	if err != nil {
		return errors.Annotatef(err, "stderr: %s", string(stderr))
	}

	// copy dm-master/conf/*.rules.yml
	targetDir := filepath.Join(deployDir, "bin", "prometheus")
	cmds := []string{
		"mkdir -p %[1]s",
		`find %[1]s -type f -name "*.rules.yml" -delete`,
		"cp %[2]s/dm-master/conf/*.rules.yml %[1]s",
		"rm -rf %[2]s",
	}
	_, stderr, err = e.Execute(fmt.Sprintf(strings.Join(cmds, " && "), targetDir, tmp), false)
	if err != nil {
		return errors.Annotatef(err, "stderr: %s", string(stderr))
	}

	return nil
}

func (i *MonitorInstance) initRules(e executor.Executor, spec PrometheusSpec, paths meta.DirPaths) error {
	if spec.RuleDir != "" {
		return i.TransferLocalConfigDir(e, spec.RuleDir, path.Join(paths.Deploy, "conf"), func(name string) bool {
			return strings.HasSuffix(name, ".rules.yml")
		})
	}

	// To make this step idempotent, we need cleanup old rules first
	cmds := []string{
		"mkdir -p %[1]s/conf",
		`find %[1]s/conf -type f -name "*.rules.yml" -delete`,
		`cp %[1]s/bin/prometheus/*.rules.yml %[1]s/conf/`,
	}
	_, stderr, err := e.Execute(fmt.Sprintf(strings.Join(cmds, " && "), paths.Deploy), false)
	if err != nil {
		return errors.Annotatef(err, "stderr: %s", string(stderr))
	}

	return nil
}

// ScaleConfig deploy temporary config on scaling
func (i *MonitorInstance) ScaleConfig(
	e executor.Executor,
	topo Topology,
	clusterName string,
	clusterVersion string,
	deployUser string,
	paths meta.DirPaths,
) error {
	s := i.topo
	defer func() { i.topo = s }()
	i.topo = topo
	return i.InitConfig(e, clusterName, clusterVersion, deployUser, paths)
}
