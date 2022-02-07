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
	"context"
	"crypto/tls"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"time"

	"github.com/pingcap/errors"
	"github.com/pingcap/tiup/pkg/cluster/ctxt"
	"github.com/pingcap/tiup/pkg/cluster/template/config"
	"github.com/pingcap/tiup/pkg/cluster/template/scripts"
	"github.com/pingcap/tiup/pkg/meta"
	"gopkg.in/ini.v1"
)

// GrafanaSpec represents the Grafana topology specification in topology.yaml
type GrafanaSpec struct {
	Host            string               `yaml:"host"`
	SSHPort         int                  `yaml:"ssh_port,omitempty" validate:"ssh_port:editable"`
	Imported        bool                 `yaml:"imported,omitempty"`
	Patched         bool                 `yaml:"patched,omitempty"`
	IgnoreExporter  bool                 `yaml:"ignore_exporter,omitempty"`
	Port            int                  `yaml:"port" default:"3000"`
	DeployDir       string               `yaml:"deploy_dir,omitempty"`
	Config          map[string]string    `yaml:"config,omitempty" validate:"config:ignore"`
	ResourceControl meta.ResourceControl `yaml:"resource_control,omitempty" validate:"resource_control:editable"`
	Arch            string               `yaml:"arch,omitempty"`
	OS              string               `yaml:"os,omitempty"`
	DashboardDir    string               `yaml:"dashboard_dir,omitempty" validate:"dashboard_dir:editable"`
	Username        string               `yaml:"username,omitempty" default:"admin" validate:"username:editable"`
	Password        string               `yaml:"password,omitempty" default:"admin" validate:"password:editable"`
	AnonymousEnable bool                 `yaml:"anonymous_enable" default:"false" validate:"anonymous_enable:editable"`
	RootURL         string               `yaml:"root_url" validate:"root_url:editable"`
	Domain          string               `yaml:"domain" validate:"domain:editable"`
	DefaultTheme    string               `yaml:"default_theme,omitempty" validate:"default_theme:editable"`
	OrgName         string               `yaml:"org_name,omitempty" validate:"org_name:editable"`
	OrgRole         string               `yaml:"org_role,omitempty" validate:"org_role:editable"`
}

// Role returns the component role of the instance
func (s *GrafanaSpec) Role() string {
	return ComponentGrafana
}

// SSH returns the host and SSH port of the instance
func (s *GrafanaSpec) SSH() (string, int) {
	return s.Host, s.SSHPort
}

// GetMainPort returns the main port of the instance
func (s *GrafanaSpec) GetMainPort() int {
	return s.Port
}

// IsImported returns if the node is imported from TiDB-Ansible
func (s *GrafanaSpec) IsImported() bool {
	return s.Imported
}

// IgnoreMonitorAgent returns if the node does not have monitor agents available
func (s *GrafanaSpec) IgnoreMonitorAgent() bool {
	return s.IgnoreExporter
}

// GrafanaComponent represents Grafana component.
type GrafanaComponent struct{ Topology }

// Name implements Component interface.
func (c *GrafanaComponent) Name() string {
	return ComponentGrafana
}

// Role implements Component interface.
func (c *GrafanaComponent) Role() string {
	return RoleMonitor
}

// Instances implements Component interface.
func (c *GrafanaComponent) Instances() []Instance {
	servers := c.BaseTopo().Grafanas
	ins := make([]Instance, 0, len(servers))

	for _, s := range servers {
		s := s
		ins = append(ins, &GrafanaInstance{
			BaseInstance: BaseInstance{
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
				StatusFn: func(_ context.Context, _ *tls.Config, _ ...string) string {
					return statusByHost(s.Host, s.Port, "", nil)
				},
				UptimeFn: func(_ context.Context, tlsCfg *tls.Config) time.Duration {
					return UptimeByHost(s.Host, s.Port, tlsCfg)
				},
			},
			topo: c.Topology,
		})
	}
	return ins
}

// GrafanaInstance represent the grafana instance
type GrafanaInstance struct {
	BaseInstance
	topo Topology
}

// InitConfig implement Instance interface
func (i *GrafanaInstance) InitConfig(
	ctx context.Context,
	e ctxt.Executor,
	clusterName,
	clusterVersion,
	deployUser string,
	paths meta.DirPaths,
) error {
	gOpts := *i.topo.BaseTopo().GlobalOptions
	if err := i.BaseInstance.InitConfig(ctx, e, gOpts, deployUser, paths); err != nil {
		return err
	}

	// transfer run script
	cfg := scripts.NewGrafanaScript(clusterName, paths.Deploy)
	fp := filepath.Join(paths.Cache, fmt.Sprintf("run_grafana_%s_%d.sh", i.GetHost(), i.GetPort()))
	if err := cfg.ConfigToFile(fp); err != nil {
		return err
	}

	dst := filepath.Join(paths.Deploy, "scripts", "run_grafana.sh")
	if err := e.Transfer(ctx, fp, dst, false, 0, false); err != nil {
		return err
	}

	if _, _, err := e.Execute(ctx, "chmod +x "+dst, false); err != nil {
		return err
	}

	// transfer config
	spec := i.InstanceSpec.(*GrafanaSpec)
	fp = filepath.Join(paths.Cache, fmt.Sprintf("grafana_%s.ini", i.GetHost()))
	if err := config.NewGrafanaConfig(i.GetHost(), paths.Deploy).
		WithPort(uint64(i.GetPort())).
		WithUsername(spec.Username).
		WithPassword(spec.Password).
		WithAnonymousenable(spec.AnonymousEnable).
		WithRootURL(spec.RootURL).
		WithDomain(spec.Domain).
		WithDefaultTheme(spec.DefaultTheme).
		WithOrgName(spec.OrgName).
		WithOrgRole(spec.OrgRole).
		ConfigToFile(fp); err != nil {
		return err
	}

	// doesn't work
	if _, err := i.setTLSConfig(ctx, false, nil, paths); err != nil {
		return err
	}

	userConfig := i.topo.GetGrafanaConfig()
	if userConfig == nil {
		userConfig = make(map[string]string)
	}
	for k, v := range spec.Config {
		userConfig[k] = v
	}
	err := mergeAdditionalGrafanaConf(fp, userConfig)
	if err != nil {
		return err
	}

	dst = filepath.Join(paths.Deploy, "conf", "grafana.ini")
	if err := e.Transfer(ctx, fp, dst, false, 0, false); err != nil {
		return err
	}
	if err := i.installDashboards(ctx, e, paths.Deploy, clusterName, clusterVersion); err != nil {
		return errors.Annotate(err, "install dashboards")
	}

	// initial dashboards/*.json
	if err := i.initDashboards(ctx, e, i.InstanceSpec.(*GrafanaSpec), paths, clusterName); err != nil {
		return errors.Annotate(err, "initial dashboards")
	}

	// transfer dashboard.yml
	fp = filepath.Join(paths.Cache, fmt.Sprintf("dashboard_%s.yml", i.GetHost()))
	if err := config.NewDashboardConfig(clusterName, paths.Deploy).ConfigToFile(fp); err != nil {
		return err
	}
	dst = filepath.Join(paths.Deploy, "provisioning", "dashboards", "dashboard.yml")
	if err := i.TransferLocalConfigFile(ctx, e, fp, dst); err != nil {
		return err
	}

	topo := reflect.ValueOf(i.topo)
	if topo.Kind() == reflect.Ptr {
		topo = topo.Elem()
	}
	val := topo.FieldByName("Monitors")
	if (val == reflect.Value{}) {
		return errors.Errorf("field Monitors not found in topology: %v", topo)
	}
	monitors := val.Interface().([]*PrometheusSpec)
	// transfer datasource.yml
	if len(monitors) == 0 {
		return errors.New("no prometheus found in topology")
	}
	fp = filepath.Join(paths.Cache, fmt.Sprintf("datasource_%s.yml", i.GetHost()))
	if err := config.NewDatasourceConfig(clusterName, monitors[0].Host).
		WithPort(uint64(monitors[0].Port)).
		ConfigToFile(fp); err != nil {
		return err
	}
	dst = filepath.Join(paths.Deploy, "provisioning", "datasources", "datasource.yml")
	return i.TransferLocalConfigFile(ctx, e, fp, dst)
}

// setTLSConfig set TLS Config to support enable/disable TLS
func (i *GrafanaInstance) setTLSConfig(ctx context.Context, enableTLS bool, configs map[string]interface{}, paths meta.DirPaths) (map[string]interface{}, error) {
	return nil, nil
}

func (i *GrafanaInstance) initDashboards(ctx context.Context, e ctxt.Executor, spec *GrafanaSpec, paths meta.DirPaths, clusterName string) error {
	dashboardsDir := filepath.Join(paths.Deploy, "dashboards")
	if spec.DashboardDir != "" {
		return i.TransferLocalConfigDir(ctx, e, spec.DashboardDir, dashboardsDir, func(name string) bool {
			return strings.HasSuffix(name, ".json")
		})
	}

	cmds := []string{
		"mkdir -p %[1]s",
		`find %[1]s -maxdepth 1 -type f -name "*.json" -delete`,
		`find %[2]s/bin -maxdepth 1 -type f -name "*.json" -exec cp {} %[1]s \;`,
	}
	_, stderr, err := e.Execute(ctx, fmt.Sprintf(strings.Join(cmds, " && "), dashboardsDir, paths.Deploy), false)
	if err != nil {
		return errors.Annotatef(err, "stderr: %s", string(stderr))
	}

	// Deal with the cluster name
	for _, cmd := range []string{
		`find %s -type f -exec sed -i 's/\${DS_.*-CLUSTER}/%s/g' {} \;`,
		`find %s -type f -exec sed -i 's/DS_.*-CLUSTER/%s/g' {} \;`,
		`find %s -type f -exec sed -i 's/\${DS_LIGHTNING}/%s/g' {} \;`,
		`find %s -type f -exec sed -i 's/DS_LIGHTNING/%s/g' {} \;`,
		`find %s -type f -exec sed -i 's/test-cluster/%s/g' {} \;`,
		`find %s -type f -exec sed -i 's/Test-Cluster/%s/g' {} \;`,
	} {
		cmd := fmt.Sprintf(cmd, dashboardsDir, clusterName)
		_, stderr, err := e.Execute(ctx, cmd, false)
		if err != nil {
			return errors.Annotatef(err, "stderr: %s", string(stderr))
		}
	}

	return nil
}

// We only really installDashboards for dm cluster because the dashboards(*.json) packed with
// the grafana component is designed for tidb cluster (the dm cluster use the same cluster
// component with tidb cluster), and the dashboards for dm cluster is packed in the dm-master
// component. So if deploying tidb cluster, the dashboards is correct, if deploying dm cluster,
// we should remove dashboards for tidb and install dashboards for dm.
func (i *GrafanaInstance) installDashboards(ctx context.Context, e ctxt.Executor, deployDir, clusterName, clusterVersion string) error {
	if i.topo.Type() != TopoTypeDM {
		return nil
	}

	tmp := filepath.Join(deployDir, "_tiup_tmp")
	_, stderr, err := e.Execute(ctx, fmt.Sprintf("mkdir -p %s", tmp), false)
	if err != nil {
		return errors.Annotatef(err, "stderr: %s", string(stderr))
	}

	srcPath := PackagePath(ComponentDMMaster, clusterVersion, i.OS(), i.Arch())
	dstPath := filepath.Join(tmp, filepath.Base(srcPath))
	err = e.Transfer(ctx, srcPath, dstPath, false, 0, false)
	if err != nil {
		return err
	}

	cmd := fmt.Sprintf(`tar --no-same-owner -zxf %s -C %s && rm %s`, dstPath, tmp, dstPath)
	_, stderr, err = e.Execute(ctx, cmd, false)
	if err != nil {
		return errors.Annotatef(err, "stderr: %s", string(stderr))
	}

	// copy dm-master/scripts/*.json
	targetDir := filepath.Join(deployDir, "bin")
	cmds := []string{
		"mkdir -p %[1]s",
		`find %[1]s -maxdepth 1 -type f -name "*.json" -delete`,
		`find %[2]s/dm-master/scripts -type f -name "*.json" -exec cp {} %[1]s \;`,
		"rm -rf %[2]s",
	}
	_, stderr, err = e.Execute(ctx, fmt.Sprintf(strings.Join(cmds, " && "), targetDir, tmp), false)
	if err != nil {
		return errors.Annotatef(err, "stderr: %s", string(stderr))
	}

	return nil
}

// ScaleConfig deploy temporary config on scaling
func (i *GrafanaInstance) ScaleConfig(
	ctx context.Context,
	e ctxt.Executor,
	topo Topology,
	clusterName string,
	clusterVersion string,
	deployUser string,
	paths meta.DirPaths,
) error {
	s := i.topo
	defer func() { i.topo = s }()
	i.topo = topo.Merge(i.topo)
	return i.InitConfig(ctx, e, clusterName, clusterVersion, deployUser, paths)
}

func mergeAdditionalGrafanaConf(source string, addition map[string]string) error {
	bytes, err := os.ReadFile(source)
	if err != nil {
		return err
	}
	result, err := ini.Load(bytes)
	if err != nil {
		return err
	}
	for k, v := range addition {
		// convert "log.file.level to [log.file] level"
		for i := len(k) - 1; i >= 0; i-- {
			if k[i] == '.' {
				result.Section(k[:i]).Key(k[i+1:]).SetValue(v)
				break
			}
		}
	}
	return result.SaveTo(source)
}
