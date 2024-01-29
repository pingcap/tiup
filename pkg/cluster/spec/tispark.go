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
	"path/filepath"
	"reflect"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/pingcap/errors"
	"github.com/pingcap/tiup/pkg/checkpoint"
	"github.com/pingcap/tiup/pkg/cluster/ctxt"
	"github.com/pingcap/tiup/pkg/cluster/template/config"
	"github.com/pingcap/tiup/pkg/cluster/template/scripts"
	system "github.com/pingcap/tiup/pkg/cluster/template/systemd"
	"github.com/pingcap/tiup/pkg/meta"
	"github.com/pingcap/tiup/pkg/utils"
	"go.uber.org/zap"
)

// TiSparkMasterSpec is the topology specification for TiSpark master node
type TiSparkMasterSpec struct {
	Host           string            `yaml:"host"`
	ManageHost     string            `yaml:"manage_host,omitempty" validate:"manage_host:editable"`
	ListenHost     string            `yaml:"listen_host,omitempty"`
	SSHPort        int               `yaml:"ssh_port,omitempty" validate:"ssh_port:editable"`
	Imported       bool              `yaml:"imported,omitempty"`
	Patched        bool              `yaml:"patched,omitempty"`
	IgnoreExporter bool              `yaml:"ignore_exporter,omitempty"`
	Port           int               `yaml:"port" default:"7077"`
	WebPort        int               `yaml:"web_port" default:"8080"`
	DeployDir      string            `yaml:"deploy_dir,omitempty"`
	JavaHome       string            `yaml:"java_home,omitempty" validate:"java_home:editable"`
	SparkConfigs   map[string]any    `yaml:"spark_config,omitempty" validate:"spark_config:ignore"`
	SparkEnvs      map[string]string `yaml:"spark_env,omitempty" validate:"spark_env:ignore"`
	Arch           string            `yaml:"arch,omitempty"`
	OS             string            `yaml:"os,omitempty"`
}

// Role returns the component role of the instance
func (s *TiSparkMasterSpec) Role() string {
	return RoleTiSparkMaster
}

// SSH returns the host and SSH port of the instance
func (s *TiSparkMasterSpec) SSH() (string, int) {
	host := s.Host
	if s.ManageHost != "" {
		host = s.ManageHost
	}
	return host, s.SSHPort
}

// GetMainPort returns the main port of the instance
func (s *TiSparkMasterSpec) GetMainPort() int {
	return s.Port
}

// IsImported returns if the node is imported from TiDB-Ansible
func (s *TiSparkMasterSpec) IsImported() bool {
	return s.Imported
}

// IgnoreMonitorAgent returns if the node does not have monitor agents available
func (s *TiSparkMasterSpec) IgnoreMonitorAgent() bool {
	return s.IgnoreExporter
}

// TiSparkWorkerSpec is the topology specification for TiSpark slave nodes
type TiSparkWorkerSpec struct {
	Host           string `yaml:"host"`
	ManageHost     string `yaml:"manage_host,omitempty"`
	ListenHost     string `yaml:"listen_host,omitempty"`
	SSHPort        int    `yaml:"ssh_port,omitempty" validate:"ssh_port:editable"`
	Imported       bool   `yaml:"imported,omitempty"`
	Patched        bool   `yaml:"patched,omitempty"`
	IgnoreExporter bool   `yaml:"ignore_exporter,omitempty"`
	Port           int    `yaml:"port" default:"7078"`
	WebPort        int    `yaml:"web_port" default:"8081"`
	DeployDir      string `yaml:"deploy_dir,omitempty"`
	JavaHome       string `yaml:"java_home,omitempty" validate:"java_home:editable"`
	Arch           string `yaml:"arch,omitempty"`
	OS             string `yaml:"os,omitempty"`
}

// Role returns the component role of the instance
func (s *TiSparkWorkerSpec) Role() string {
	return RoleTiSparkWorker
}

// SSH returns the host and SSH port of the instance
func (s *TiSparkWorkerSpec) SSH() (string, int) {
	host := s.Host
	if s.ManageHost != "" {
		host = s.ManageHost
	}
	return host, s.SSHPort
}

// GetMainPort returns the main port of the instance
func (s *TiSparkWorkerSpec) GetMainPort() int {
	return s.Port
}

// IsImported returns if the node is imported from TiDB-Ansible
func (s *TiSparkWorkerSpec) IsImported() bool {
	return s.Imported
}

// IgnoreMonitorAgent returns if the node does not have monitor agents available
func (s *TiSparkWorkerSpec) IgnoreMonitorAgent() bool {
	return s.IgnoreExporter
}

// TiSparkMasterComponent represents TiSpark master component.
type TiSparkMasterComponent struct{ Topology *Specification }

// Name implements Component interface.
func (c *TiSparkMasterComponent) Name() string {
	return ComponentTiSpark
}

// Role implements Component interface.
func (c *TiSparkMasterComponent) Role() string {
	return RoleTiSparkMaster
}

// Source implements Component interface.
func (c *TiSparkMasterComponent) Source() string {
	return ComponentTiSpark
}

// CalculateVersion implements the Component interface
func (c *TiSparkMasterComponent) CalculateVersion(clusterVersion string) string {
	return ""
}

// SetVersion implements Component interface.
func (c *TiSparkMasterComponent) SetVersion(version string) {
	// should never be calles
}

// Instances implements Component interface.
func (c *TiSparkMasterComponent) Instances() []Instance {
	ins := make([]Instance, 0, len(c.Topology.TiSparkMasters))
	for _, s := range c.Topology.TiSparkMasters {
		s := s
		ins = append(ins, &TiSparkMasterInstance{
			BaseInstance: BaseInstance{
				InstanceSpec: s,
				Name:         c.Name(),
				ManageHost:   s.ManageHost,
				Host:         s.Host,
				Port:         s.Port,
				SSHP:         s.SSHPort,
				NumaNode:     "",
				NumaCores:    "",

				Ports: []int{
					s.Port,
					s.WebPort,
				},
				Dirs: []string{
					s.DeployDir,
				},
				StatusFn: func(_ context.Context, timeout time.Duration, tlsCfg *tls.Config, _ ...string) string {
					return statusByHost(s.Host, s.WebPort, "", timeout, tlsCfg)
				},
				UptimeFn: func(_ context.Context, timeout time.Duration, tlsCfg *tls.Config) time.Duration {
					return 0
				},
				Component: c,
			},
			topo: c.Topology,
		})
	}
	return ins
}

// TiSparkMasterInstance represent the TiSpark master instance
type TiSparkMasterInstance struct {
	BaseInstance
	topo Topology
}

// GetCustomFields get custom spark configs of the instance
func (i *TiSparkMasterInstance) GetCustomFields() map[string]any {
	v := reflect.Indirect(reflect.ValueOf(i.InstanceSpec)).FieldByName("SparkConfigs")
	if !v.IsValid() {
		return nil
	}
	return v.Interface().(map[string]any)
}

// GetCustomEnvs get custom spark envionment variables of the instance
func (i *TiSparkMasterInstance) GetCustomEnvs() map[string]string {
	v := reflect.Indirect(reflect.ValueOf(i.InstanceSpec)).FieldByName("SparkEnvs")
	if !v.IsValid() {
		return nil
	}
	return v.Interface().(map[string]string)
}

// GetJavaHome returns the java_home value in spec
func (i *TiSparkMasterInstance) GetJavaHome() string {
	return reflect.Indirect(reflect.ValueOf(i.InstanceSpec)).FieldByName("JavaHome").String()
}

// InitConfig implement Instance interface
func (i *TiSparkMasterInstance) InitConfig(
	ctx context.Context,
	e ctxt.Executor,
	clusterName,
	clusterVersion,
	deployUser string,
	paths meta.DirPaths,
) (err error) {
	// generate systemd service to invoke spark's start/stop scripts
	comp := i.Role()
	host := i.GetHost()
	port := i.GetPort()
	topo := i.topo.(*Specification)
	sysCfg := filepath.Join(paths.Cache, fmt.Sprintf("%s-%s-%d.service", comp, host, port))

	// insert checkpoint
	point := checkpoint.Acquire(ctx, CopyConfigFile, map[string]any{"config-file": sysCfg})
	defer func() {
		point.Release(err, zap.String("config-file", sysCfg))
	}()
	if point.Hit() != nil {
		return nil
	}

	systemCfg := system.NewTiSparkConfig(comp, deployUser, paths.Deploy, i.GetJavaHome())

	if err := systemCfg.ConfigToFile(sysCfg); err != nil {
		return errors.Trace(err)
	}
	tgt := filepath.Join("/tmp", comp+"_"+uuid.New().String()+".service")
	if err := e.Transfer(ctx, sysCfg, tgt, false, 0, false); err != nil {
		return errors.Annotatef(err, "transfer from %s to %s failed", sysCfg, tgt)
	}
	systemdDir := "/etc/systemd/system/"
	sudo := true
	if i.topo.BaseTopo().GlobalOptions.SystemdMode == UserMode {
		systemdDir = "~/.config/systemd/user/"
		sudo = false
	}
	cmd := fmt.Sprintf("mv %s %s%s-%d.service", tgt, systemdDir, comp, port)
	if _, _, err := e.Execute(ctx, cmd, sudo); err != nil {
		return errors.Annotatef(err, "execute: %s", cmd)
	}

	// transfer default config
	pdList := topo.GetPDList()
	masterList := make([]string, 0)
	for _, master := range topo.TiSparkMasters {
		masterList = append(masterList, utils.JoinHostPort(master.Host, master.Port))
	}

	cfg := config.NewTiSparkConfig(pdList).WithMasters(strings.Join(masterList, ",")).
		WithCustomFields(i.GetCustomFields())
	// transfer spark-defaults.conf
	fp := filepath.Join(paths.Cache, fmt.Sprintf("spark-defaults-%s-%d.conf", host, port))
	if err := cfg.ConfigToFile(fp); err != nil {
		return err
	}
	dst := filepath.Join(paths.Deploy, "conf", "spark-defaults.conf")
	if err := e.Transfer(ctx, fp, dst, false, 0, false); err != nil {
		return err
	}

	env := scripts.NewTiSparkEnv(host).
		WithLocalIP(i.GetListenHost()).
		WithMaster(host).
		WithMasterPorts(i.Ports[0], i.Ports[1]).
		WithCustomEnv(i.GetCustomEnvs())
	// transfer spark-env.sh file
	fp = filepath.Join(paths.Cache, fmt.Sprintf("spark-env-%s-%d.sh", host, port))
	if err := env.ScriptToFile(fp); err != nil {
		return err
	}
	// tispark files are all in a "spark" sub-directory of deploy dir
	dst = filepath.Join(paths.Deploy, "conf", "spark-env.sh")
	if err := e.Transfer(ctx, fp, dst, false, 0, false); err != nil {
		return err
	}

	// transfer log4j config (it's not a template but a static file)
	fp = filepath.Join(paths.Cache, fmt.Sprintf("spark-log4j-%s-%d.properties", host, port))
	log4jFile, err := config.GetConfig("spark-log4j.properties.tpl")
	if err != nil {
		return err
	}
	if err := utils.WriteFile(fp, log4jFile, 0644); err != nil {
		return err
	}
	dst = filepath.Join(paths.Deploy, "conf", "log4j.properties")
	return e.Transfer(ctx, fp, dst, false, 0, false)
}

// ScaleConfig deploy temporary config on scaling
func (i *TiSparkMasterInstance) ScaleConfig(
	ctx context.Context,
	e ctxt.Executor,
	topo Topology,
	clusterName,
	clusterVersion,
	deployUser string,
	paths meta.DirPaths,
) error {
	s := i.topo
	defer func() { i.topo = s }()
	cluster := mustBeClusterTopo(topo)
	i.topo = cluster.Merge(i.topo)
	return i.InitConfig(ctx, e, clusterName, clusterVersion, deployUser, paths)
}

// TiSparkWorkerComponent represents TiSpark slave component.
type TiSparkWorkerComponent struct{ Topology *Specification }

// Name implements Component interface.
func (c *TiSparkWorkerComponent) Name() string {
	return ComponentTiSpark
}

// Role implements Component interface.
func (c *TiSparkWorkerComponent) Role() string {
	return RoleTiSparkWorker
}

// Source implements Component interface.
func (c *TiSparkWorkerComponent) Source() string {
	return ComponentTiSpark
}

// CalculateVersion implements the Component interface
func (c *TiSparkWorkerComponent) CalculateVersion(clusterVersion string) string {
	return ""
}

// SetVersion implements Component interface.
func (c *TiSparkWorkerComponent) SetVersion(version string) {
	// should never be called
}

// Instances implements Component interface.
func (c *TiSparkWorkerComponent) Instances() []Instance {
	ins := make([]Instance, 0, len(c.Topology.TiSparkWorkers))
	for _, s := range c.Topology.TiSparkWorkers {
		ins = append(ins, &TiSparkWorkerInstance{
			BaseInstance: BaseInstance{
				InstanceSpec: s,
				Name:         c.Name(),
				ManageHost:   s.ManageHost,
				Host:         s.Host,
				Port:         s.Port,
				SSHP:         s.SSHPort,
				NumaNode:     "",
				NumaCores:    "",

				Ports: []int{
					s.Port,
					s.WebPort,
				},
				Dirs: []string{
					s.DeployDir,
				},
				StatusFn: func(_ context.Context, timeout time.Duration, tlsCfg *tls.Config, _ ...string) string {
					return statusByHost(s.Host, s.WebPort, "", timeout, tlsCfg)
				},
				UptimeFn: func(_ context.Context, timeout time.Duration, tlsCfg *tls.Config) time.Duration {
					return 0
				},
				Component: c,
			},
			topo: c.Topology,
		})
	}
	return ins
}

// TiSparkWorkerInstance represent the TiSpark slave instance
type TiSparkWorkerInstance struct {
	BaseInstance
	topo Topology
}

// GetJavaHome returns the java_home value in spec
func (i *TiSparkWorkerInstance) GetJavaHome() string {
	return reflect.Indirect(reflect.ValueOf(i.InstanceSpec)).FieldByName("JavaHome").String()
}

// InitConfig implement Instance interface
func (i *TiSparkWorkerInstance) InitConfig(
	ctx context.Context,
	e ctxt.Executor,
	clusterName,
	clusterVersion,
	deployUser string,
	paths meta.DirPaths,
) (err error) {
	// generate systemd service to invoke spark's start/stop scripts
	comp := i.Role()
	host := i.GetHost()
	port := i.GetPort()
	topo := i.topo.(*Specification)
	sysCfg := filepath.Join(paths.Cache, fmt.Sprintf("%s-%s-%d.service", comp, host, port))

	// insert checkpoint
	point := checkpoint.Acquire(ctx, CopyConfigFile, map[string]any{"config-file": sysCfg})
	defer func() {
		point.Release(err, zap.String("config-file", sysCfg))
	}()
	if point.Hit() != nil {
		return nil
	}

	systemCfg := system.NewTiSparkConfig(comp, deployUser, paths.Deploy, i.GetJavaHome())

	if err := systemCfg.ConfigToFile(sysCfg); err != nil {
		return errors.Trace(err)
	}
	tgt := filepath.Join("/tmp", comp+"_"+uuid.New().String()+".service")
	if err := e.Transfer(ctx, sysCfg, tgt, false, 0, false); err != nil {
		return errors.Annotatef(err, "transfer from %s to %s failed", sysCfg, tgt)
	}
	systemdDir := "/etc/systemd/system/"
	sudo := true
	if i.topo.BaseTopo().GlobalOptions.SystemdMode == UserMode {
		systemdDir = "~/.config/systemd/user/"
		sudo = false
	}
	cmd := fmt.Sprintf("mv %s %s%s-%d.service", tgt, systemdDir, comp, port)
	if _, _, err := e.Execute(ctx, cmd, sudo); err != nil {
		return errors.Annotatef(err, "execute: %s", cmd)
	}

	// transfer default config
	pdList := topo.GetPDList()
	masterList := make([]string, 0)
	for _, master := range topo.TiSparkMasters {
		masterList = append(masterList, utils.JoinHostPort(master.Host, master.Port))
	}

	cfg := config.NewTiSparkConfig(pdList).WithMasters(strings.Join(masterList, ",")).
		WithCustomFields(topo.TiSparkMasters[0].SparkConfigs)

	// doesn't work
	if _, err := i.setTLSConfig(ctx, false, nil, paths); err != nil {
		return err
	}

	// transfer spark-defaults.conf
	fp := filepath.Join(paths.Cache, fmt.Sprintf("spark-defaults-%s-%d.conf", host, port))
	if err := cfg.ConfigToFile(fp); err != nil {
		return err
	}
	dst := filepath.Join(paths.Deploy, "conf", "spark-defaults.conf")
	if err := e.Transfer(ctx, fp, dst, false, 0, false); err != nil {
		return err
	}

	env := scripts.NewTiSparkEnv(host).
		WithLocalIP(i.GetListenHost()).
		WithMaster(topo.TiSparkMasters[0].Host).
		WithMasterPorts(topo.TiSparkMasters[0].Port, topo.TiSparkMasters[0].WebPort).
		WithWorkerPorts(i.Ports[0], i.Ports[1]).
		WithCustomEnv(topo.TiSparkMasters[0].SparkEnvs)
	// transfer spark-env.sh file
	fp = filepath.Join(paths.Cache, fmt.Sprintf("spark-env-%s-%d.sh", host, port))
	if err := env.ScriptToFile(fp); err != nil {
		return err
	}
	// tispark files are all in a "spark" sub-directory of deploy dir
	dst = filepath.Join(paths.Deploy, "conf", "spark-env.sh")
	if err := e.Transfer(ctx, fp, dst, false, 0, false); err != nil {
		return err
	}

	// transfer start-slave.sh
	fp = filepath.Join(paths.Cache, fmt.Sprintf("start-tispark-slave-%s-%d.sh", host, port))
	slaveSh, err := env.SlaveScriptWithTemplate()
	if err != nil {
		return err
	}
	if err := utils.WriteFile(fp, slaveSh, 0755); err != nil {
		return err
	}
	dst = filepath.Join(paths.Deploy, "sbin", "start-slave.sh")
	if err := e.Transfer(ctx, fp, dst, false, 0, false); err != nil {
		return err
	}

	// transfer log4j config (it's not a template but a static file)
	fp = filepath.Join(paths.Cache, fmt.Sprintf("spark-log4j-%s-%d.properties", host, port))
	log4jFile, err := config.GetConfig("spark-log4j.properties.tpl")
	if err != nil {
		return err
	}
	if err := utils.WriteFile(fp, log4jFile, 0644); err != nil {
		return err
	}
	dst = filepath.Join(paths.Deploy, "conf", "log4j.properties")
	return e.Transfer(ctx, fp, dst, false, 0, false)
}

// setTLSConfig set TLS Config to support enable/disable TLS
// TiSparkWorkerInstance no need to configure TLS
func (i *TiSparkWorkerInstance) setTLSConfig(ctx context.Context, enableTLS bool, configs map[string]any, paths meta.DirPaths) (map[string]any, error) {
	return nil, nil
}

// ScaleConfig deploy temporary config on scaling
func (i *TiSparkWorkerInstance) ScaleConfig(
	ctx context.Context,
	e ctxt.Executor,
	topo Topology,
	clusterName,
	clusterVersion,
	deployUser string,
	paths meta.DirPaths,
) error {
	s := i.topo
	defer func() { i.topo = s }()
	i.topo = topo.Merge(i.topo)
	return i.InitConfig(ctx, e, clusterName, clusterVersion, deployUser, paths)
}
