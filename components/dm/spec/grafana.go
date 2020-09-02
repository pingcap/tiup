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
	"github.com/pingcap/tiup/pkg/cluster/template/config"
	"github.com/pingcap/tiup/pkg/cluster/template/scripts"
	"github.com/pingcap/tiup/pkg/meta"
)

// GrafanaComponent represents Grafana component.
type GrafanaComponent struct{ *Topology }

// Name implements Component interface.
func (c *GrafanaComponent) Name() string {
	return ComponentGrafana
}

// Role implements Component interface.
func (c *GrafanaComponent) Role() string {
	return spec.RoleMonitor
}

// Instances implements Component interface.
func (c *GrafanaComponent) Instances() []Instance {
	ins := make([]Instance, 0, len(c.Grafana))
	for _, s := range c.Grafana {
		ins = append(ins, &GrafanaInstance{
			BaseInstance: spec.BaseInstance{
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
				},
				StatusFn: func(_ ...string) string {
					return "-"
				},
			},
			topo: c.Topology,
		})
	}
	return ins
}

// GrafanaInstance represent the grafana instance
type GrafanaInstance struct {
	spec.BaseInstance
	topo *Topology
}

// InitConfig implement Instance interface
func (i *GrafanaInstance) InitConfig(e executor.Executor, clusterName, clusterVersion, deployUser string, paths meta.DirPaths) error {
	if len(i.topo.Monitors) == 0 {
		return errors.New("no prometheus found in topology")
	}

	if err := i.BaseInstance.InitConfig(e, i.topo.GlobalOptions, deployUser, paths); err != nil {
		return err
	}

	// transfer run script
	tpl := filepath.Join("/templates", "scripts", "dm", "run_grafana.sh.tpl")
	cfg := scripts.NewGrafanaScript(clusterName, paths.Deploy).WithTPLFile(tpl)
	fp := filepath.Join(paths.Cache, fmt.Sprintf("run_grafana_%s_%d.sh", i.GetHost(), i.GetPort()))
	if err := cfg.ConfigToFile(fp); err != nil {
		return err
	}

	dst := filepath.Join(paths.Deploy, "scripts", "run_grafana.sh")
	if err := e.Transfer(fp, dst, false); err != nil {
		return err
	}

	if _, _, err := e.Execute("chmod +x "+dst, false); err != nil {
		return err
	}

	// transfer config
	fp = filepath.Join(paths.Cache, fmt.Sprintf("grafana_%s.ini", i.GetHost()))
	if err := config.NewGrafanaConfig(i.GetHost(), paths.Deploy).WithPort(uint64(i.GetPort())).ConfigToFile(fp); err != nil {
		return err
	}
	dst = filepath.Join(paths.Deploy, "conf", "grafana.ini")
	if err := e.Transfer(fp, dst, false); err != nil {
		return err
	}

	if err := i.initDashboards(e, i.InstanceSpec.(GrafanaSpec), paths); err != nil {
		return errors.Annotate(err, "initial dashboards")
	}

	var dirs []string

	// provisioningDir Must same as in grafana.ini.tpl
	provisioningDir := filepath.Join(paths.Deploy, "provisioning")
	dirs = append(dirs, provisioningDir)

	datasourceDir := filepath.Join(provisioningDir, "datasources")
	dirs = append(dirs, datasourceDir)

	dashboardDir := filepath.Join(provisioningDir, "dashboards")
	dirs = append(dirs, dashboardDir)

	cmd := fmt.Sprintf("mkdir -p %s", strings.Join(dirs, " "))
	if _, _, err := e.Execute(cmd, false); err != nil {
		return errors.AddStack(err)
	}

	// transfer dashboard.yml
	fp = filepath.Join(paths.Cache, fmt.Sprintf("dashboard_%s.yml", i.GetHost()))
	if err := config.NewDashboardConfig(clusterName, paths.Deploy).ConfigToFile(fp); err != nil {
		return err
	}
	dst = filepath.Join(dashboardDir, "dashboard.yml")
	if err := e.Transfer(fp, dst, false); err != nil {
		return err
	}

	// transfer datasource.yml
	fp = filepath.Join(paths.Cache, fmt.Sprintf("datasource_%s.yml", i.GetHost()))
	if err := config.NewDatasourceConfig(clusterName, i.topo.Monitors[0].Host).
		WithPort(uint64(i.topo.Monitors[0].Port)).
		ConfigToFile(fp); err != nil {
		return err
	}
	dst = filepath.Join(datasourceDir, "datasource.yml")
	return e.Transfer(fp, dst, false)
}

func (i *GrafanaInstance) initDashboards(e executor.Executor, spec GrafanaSpec, paths meta.DirPaths) error {
	dashboardsDir := filepath.Join(paths.Deploy, "dashboards")
	// To make this step idempotent, we need cleanup old dashboards first
	if _, _, err := e.Execute(fmt.Sprintf("rm -f %s/*.json", dashboardsDir), false); err != nil {
		return err
	}

	if spec.DashboardDir != "" {
		return i.TransferLocalConfigDir(e, spec.DashboardDir, dashboardsDir, func(name string) bool {
			return strings.HasSuffix(name, ".json")
		})
	}

	// Use the default ones
	cmd := fmt.Sprintf("cp %[1]s/bin/*.json %[1]s/dashboards/", paths.Deploy)
	if _, _, err := e.Execute(cmd, false); err != nil {
		return err
	}
	return nil
}

// ScaleConfig deploy temporary config on scaling
func (i *GrafanaInstance) ScaleConfig(e executor.Executor, topo spec.Topology,
	clusterName string, clusterVersion string, deployUser string, paths meta.DirPaths) error {
	s := i.topo
	defer func() { i.topo = s }()
	dmtopo := topo.(*Topology)
	i.topo = dmtopo.Merge(i.topo)
	return i.InitConfig(e, clusterName, clusterVersion, deployUser, paths)
}

var _ cluster.DeployerInstance = &GrafanaInstance{}

// Deploy implements DeployerInstance interface.
func (i *GrafanaInstance) Deploy(t *task.Builder, srcPath string, deployDir string, version string, clusterName string, clusterVersion string) {
	t.CopyComponent(
		i.ComponentName(),
		i.OS(),
		i.Arch(),
		version,
		srcPath,
		i.GetHost(),
		deployDir,
	).Shell( // rm the json file which relate to tidb cluster and useless.
		i.GetHost(),
		fmt.Sprintf("rm %s/*.json", filepath.Join(deployDir, "bin")),
		false, /*sudo*/
	).Func("Dashboards", func(ctx *task.Context) error {
		e := ctx.Get(i.GetHost())

		return i.installDashboards(e, deployDir, clusterName, clusterVersion)
	})
}

func (i *GrafanaInstance) installDashboards(e executor.Executor, deployDir, clusterName, clusterVersion string) error {
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

	// copy dm-master/scripts/*.json
	targetDir := filepath.Join(deployDir, "dashboards")
	_, stderr, err = e.Execute(fmt.Sprintf("mkdir -p %s", targetDir), false)
	if err != nil {
		return errors.Annotatef(err, "stderr: %s", string(stderr))
	}

	cmd = fmt.Sprintf("cp %s/dm-master/scripts/*.json %s", tmp, targetDir)
	_, stderr, err = e.Execute(cmd, false)
	if err != nil {
		return errors.Annotatef(err, "stderr: %s", string(stderr))
	}

	for _, cmd := range []string{
		`find %s -type f -exec sed -i "s/\${DS_.*-CLUSTER}/%s/g" {} \;`,
		`find %s -type f -exec sed -i "s/DS_.*-CLUSTER/%s/g" {} \;`,
		`find %s -type f -exec sed -i "s/test-cluster/%s/g" {} \;`,
		`find %s -type f -exec sed -i "s/Test-Cluster/%s/g" {} \;`,
	} {
		cmd := fmt.Sprintf(cmd, targetDir, clusterName)
		_, stderr, err = e.Execute(cmd, false)
		if err != nil {
			return errors.Annotatef(err, "stderr: %s", string(stderr))
		}
	}

	cmd = fmt.Sprintf("rm -rf %s", tmp)
	_, stderr, err = e.Execute(cmd, false)
	if err != nil {
		return errors.Annotatef(err, "stderr: %s", string(stderr))
	}

	// backup *.json for later reload (in case that the user change dashboard_dir)
	cmd = fmt.Sprintf("cp %s/*.json %s", targetDir, filepath.Join(deployDir, "bin"))
	_, stderr, err = e.Execute(cmd, false)
	if err != nil {
		return errors.Annotatef(err, "stderr: %s", string(stderr))
	}

	return nil
}
