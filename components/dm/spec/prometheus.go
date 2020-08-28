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
	"fmt"
	"path/filepath"
	"strings"

	"github.com/pingcap/errors"
	"github.com/pingcap/tiup/pkg/cluster"
	"github.com/pingcap/tiup/pkg/cluster/executor"
	"github.com/pingcap/tiup/pkg/cluster/spec"
	"github.com/pingcap/tiup/pkg/cluster/task"
	"github.com/pingcap/tiup/pkg/cluster/template/config/dm"
	"github.com/pingcap/tiup/pkg/cluster/template/scripts"
	"github.com/pingcap/tiup/pkg/meta"
)

// MonitorComponent represents Monitor component.
type MonitorComponent struct{ *Topology }

// Name implements Component interface.
func (c *MonitorComponent) Name() string {
	return ComponentPrometheus
}

// Role implements Component interface.
func (c *MonitorComponent) Role() string {
	return spec.RoleMonitor
}

// Instances implements Component interface.
func (c *MonitorComponent) Instances() []Instance {
	ins := make([]Instance, 0, len(c.Monitors))
	for _, s := range c.Monitors {
		ins = append(ins, &MonitorInstance{spec.BaseInstance{
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
			StatusFn: func(_ ...string) string {
				return "-"
			},
		}, c.Topology})
	}
	return ins
}

// MonitorInstance represent the monitor instance
type MonitorInstance struct {
	spec.BaseInstance
	topo *Topology
}

// InitConfig implement Instance interface
func (i *MonitorInstance) InitConfig(e executor.Executor, clusterName, clusterVersion, deployUser string, paths meta.DirPaths) error {
	if err := i.BaseInstance.InitConfig(e, i.topo.GlobalOptions, deployUser, paths); err != nil {
		return err
	}

	// transfer run script
	spec := i.InstanceSpec.(PrometheusSpec)
	cfg := scripts.NewPrometheusScript(
		i.GetHost(),
		paths.Deploy,
		paths.Data[0],
		paths.Log,
	).WithPort(spec.Port).
		WithNumaNode(spec.NumaNode).
		WithTPLFile(filepath.Join("/templates", "scripts", "dm", "run_prometheus.sh.tpl"))

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

	topo := i.topo

	// transfer config
	fp = filepath.Join(paths.Cache, fmt.Sprintf("prometheus_%s_%d.yml", i.GetHost(), i.GetPort()))
	cfig := dm.NewPrometheusConfig(clusterName)

	for _, master := range topo.Masters {
		cfig.AddMasterAddrs(master.Host, uint64(master.Port))
	}

	for _, worker := range topo.Workers {
		cfig.AddWorkerAddrs(worker.Host, uint64(worker.Port))
	}

	for _, alertmanager := range topo.Alertmanager {
		cfig.AddAlertmanager(alertmanager.Host, uint64(alertmanager.WebPort))
	}

	if err := i.initRules(e, spec, paths); err != nil {
		return errors.AddStack(err)
	}

	if err := cfig.ConfigToFile(fp); err != nil {
		return err
	}
	dst = filepath.Join(paths.Deploy, "conf", "prometheus.yml")
	return e.Transfer(fp, dst, false)
}

func (i *MonitorInstance) initRules(e executor.Executor, spec PrometheusSpec, paths meta.DirPaths) error {
	confDir := filepath.Join(paths.Deploy, "conf")
	// To make this step idempotent, we need cleanup old rules first
	if _, _, err := e.Execute(fmt.Sprintf("rm -f %s/*.rules.yml", confDir), false); err != nil {
		return err
	}

	// If the user specify a rule directory, we should use the rules specified
	if spec.RuleDir != "" {
		return i.TransferLocalConfigDir(e, spec.RuleDir, confDir, func(name string) bool {
			return strings.HasSuffix(name, ".rules.yml")
		})
	}

	// Use the default ones
	cmd := fmt.Sprintf("cp %[1]s/bin/prometheus/*.rules.yml %[1]s/conf/", paths.Deploy)
	if _, _, err := e.Execute(cmd, false); err != nil {
		return err
	}
	return nil
}

// ScaleConfig deploy temporary config on scaling
func (i *MonitorInstance) ScaleConfig(e executor.Executor, topo spec.Topology,
	clusterName string, clusterVersion string, deployUser string, paths meta.DirPaths) error {
	s := i.topo
	defer func() { i.topo = s }()
	i.topo = topo.(*Topology)
	return i.InitConfig(e, clusterName, clusterVersion, deployUser, paths)
}

var _ cluster.DeployerInstance = &MonitorInstance{}

// Deploy implements DeployerInstance interface.
func (i *MonitorInstance) Deploy(t *task.Builder, srcPath string, deployDir string, version string, _ string, clusterVersion string) {
	t.CopyComponent(
		i.ComponentName(),
		i.OS(),
		i.Arch(),
		version,
		srcPath,
		i.GetHost(),
		deployDir,
	).Shell( // rm the rules file which relate to tidb cluster and useless.
		i.GetHost(),
		fmt.Sprintf("rm %s/*.rules.yml", filepath.Join(deployDir, "bin", "prometheus")),
		false, /*sudo*/
	).Func("CopyRulesYML", func(ctx *task.Context) error {
		e := ctx.Get(i.GetHost())

		return i.installRules(e, deployDir, clusterVersion)
	})
}

func (i *MonitorInstance) installRules(e executor.Executor, deployDir, clusterVersion string) error {
	tmp := filepath.Join(deployDir, "_tiup_tmp")
	_, stderr, err := e.Execute(fmt.Sprintf("mkdir -p %s", tmp), false)
	if err != nil {
		return errors.Annotatef(err, "stderr: %s", string(stderr))
	}

	srcPath := task.PackagePath(ComponentDMMaster, clusterVersion, i.OS(), i.Arch())
	dstPath := filepath.Join(tmp, filepath.Base(srcPath))

	err = e.Transfer(srcPath, dstPath, false)
	if err != nil {
		return errors.AddStack(err)
	}

	cmd := fmt.Sprintf(`tar --no-same-owner -zxf %s -C %s && rm %s`, dstPath, tmp, dstPath)
	_, stderr, err = e.Execute(cmd, false)
	if err != nil {
		return errors.Annotatef(err, "stderr: %s", string(stderr))
	}

	// copy dm-master/conf/*.rules.yml
	targetDir := filepath.Join(deployDir, "conf")
	cmd = fmt.Sprintf("cp %s/dm-master/conf/*.rules.yml %s", tmp, targetDir)
	_, stderr, err = e.Execute(cmd, false)
	if err != nil {
		return errors.Annotatef(err, "stderr: %s", string(stderr))
	}

	cmd = fmt.Sprintf("rm -rf %s", tmp)
	_, stderr, err = e.Execute(cmd, false)
	if err != nil {
		return errors.Annotatef(err, "stderr: %s", string(stderr))
	}

	// backup *.rules.yml for later reload (in case that the user change rule_dir)
	cmd = fmt.Sprintf("cp %s/*.rules.yml %s", targetDir, filepath.Join(deployDir, "bin", "prometheus"))
	_, stderr, err = e.Execute(cmd, false)
	if err != nil {
		return errors.Annotatef(err, "stderr: %s", string(stderr))
	}

	return nil
}
