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
	"io/ioutil"
	"path/filepath"
	"reflect"
	"strings"

	"github.com/google/uuid"
	"github.com/pingcap/errors"
	"github.com/pingcap/tiup/pkg/cluster/executor"
	"github.com/pingcap/tiup/pkg/cluster/template/config"
	"github.com/pingcap/tiup/pkg/cluster/template/scripts"
	system "github.com/pingcap/tiup/pkg/cluster/template/systemd"
	"github.com/pingcap/tiup/pkg/meta"
)

// TiSparkMasterSpec is the topology specification for TiSpark master node
type TiSparkMasterSpec struct {
	Host         string                 `yaml:"host"`
	ListenHost   string                 `yaml:"listen_host,omitempty"`
	SSHPort      int                    `yaml:"ssh_port,omitempty" validate:"ssh_port:editable"`
	Imported     bool                   `yaml:"imported,omitempty"`
	Port         int                    `yaml:"port" default:"7077"`
	WebPort      int                    `yaml:"web_port" default:"8080"`
	DeployDir    string                 `yaml:"deploy_dir,omitempty"`
	JavaHome     string                 `yaml:"java_home,omitempty" validate:"java_home:editable"`
	SparkConfigs map[string]interface{} `yaml:"spark_config,omitempty" validate:"spark_config:editable"`
	SparkEnvs    map[string]string      `yaml:"spark_env,omitempty" validate:"spark_env:editable"`
	Arch         string                 `yaml:"arch,omitempty"`
	OS           string                 `yaml:"os,omitempty"`
}

// Role returns the component role of the instance
func (s TiSparkMasterSpec) Role() string {
	return RoleTiSparkMaster
}

// SSH returns the host and SSH port of the instance
func (s TiSparkMasterSpec) SSH() (string, int) {
	return s.Host, s.SSHPort
}

// GetMainPort returns the main port of the instance
func (s TiSparkMasterSpec) GetMainPort() int {
	return s.Port
}

// IsImported returns if the node is imported from TiDB-Ansible
func (s TiSparkMasterSpec) IsImported() bool {
	return s.Imported
}

// Status queries current status of the instance
func (s TiSparkMasterSpec) Status(pdList ...string) string {
	url := fmt.Sprintf("http://%s:%d/", s.Host, s.WebPort)
	return statusByURL(url)
}

// TiSparkWorkerSpec is the topology specification for TiSpark slave nodes
type TiSparkWorkerSpec struct {
	Host       string `yaml:"host"`
	ListenHost string `yaml:"listen_host,omitempty"`
	SSHPort    int    `yaml:"ssh_port,omitempty" validate:"ssh_port:editable"`
	Imported   bool   `yaml:"imported,omitempty"`
	Port       int    `yaml:"port" default:"7078"`
	WebPort    int    `yaml:"web_port" default:"8081"`
	DeployDir  string `yaml:"deploy_dir,omitempty"`
	JavaHome   string `yaml:"java_home,omitempty" validate:"java_home:editable"`
	Arch       string `yaml:"arch,omitempty"`
	OS         string `yaml:"os,omitempty"`
}

// Role returns the component role of the instance
func (s TiSparkWorkerSpec) Role() string {
	return RoleTiSparkWorker
}

// SSH returns the host and SSH port of the instance
func (s TiSparkWorkerSpec) SSH() (string, int) {
	return s.Host, s.SSHPort
}

// GetMainPort returns the main port of the instance
func (s TiSparkWorkerSpec) GetMainPort() int {
	return s.Port
}

// IsImported returns if the node is imported from TiDB-Ansible
func (s TiSparkWorkerSpec) IsImported() bool {
	return s.Imported
}

// Status queries current status of the instance
func (s TiSparkWorkerSpec) Status(pdList ...string) string {
	url := fmt.Sprintf("http://%s:%d/", s.Host, s.WebPort)
	return statusByURL(url)
}

// TiSparkMasterComponent represents TiSpark master component.
type TiSparkMasterComponent struct{ *Specification }

// Name implements Component interface.
func (c *TiSparkMasterComponent) Name() string {
	return ComponentTiSpark
}

// Instances implements Component interface.
func (c *TiSparkMasterComponent) Instances() []Instance {
	ins := make([]Instance, 0, len(c.TiSparkMasters))
	for _, s := range c.TiSparkMasters {
		ins = append(ins, &TiSparkMasterInstance{
			instance: instance{
				InstanceSpec: s,
				name:         c.Name(),
				host:         s.Host,
				port:         s.Port,
				sshp:         s.SSHPort,
				topo:         c.Specification,

				usedPorts: []int{
					s.Port,
					s.WebPort,
				},
				usedDirs: []string{
					s.DeployDir,
				},
				statusFn: s.Status,
			},
		})
	}
	return ins
}

// TiSparkMasterInstance represent the TiSpark master instance
type TiSparkMasterInstance struct {
	instance
}

// GetCustomFields get custom spark configs of the instance
func (i *TiSparkMasterInstance) GetCustomFields() map[string]interface{} {
	v := reflect.ValueOf(i.InstanceSpec).FieldByName("SparkConfigs")
	if !v.IsValid() {
		return nil
	}
	return v.Interface().(map[string]interface{})
}

// GetCustomEnvs get custom spark envionment variables of the instance
func (i *TiSparkMasterInstance) GetCustomEnvs() map[string]string {
	v := reflect.ValueOf(i.InstanceSpec).FieldByName("SparkEnvs")
	if !v.IsValid() {
		return nil
	}
	return v.Interface().(map[string]string)
}

// GetJavaHome returns the java_home value in spec
func (i *TiSparkMasterInstance) GetJavaHome() string {
	return reflect.ValueOf(i.InstanceSpec).FieldByName("JavaHome").String()
}

// InitConfig implement Instance interface
func (i *TiSparkMasterInstance) InitConfig(e executor.Executor, clusterName, clusterVersion, deployUser string, paths meta.DirPaths) error {
	// generate systemd service to invoke spark's start/stop scripts
	comp := i.Role()
	host := i.GetHost()
	port := i.GetPort()
	sysCfg := filepath.Join(paths.Cache, fmt.Sprintf("%s-%s-%d.service", comp, host, port))

	systemCfg := system.NewTiSparkConfig(comp, deployUser, paths.Deploy, i.GetJavaHome())

	if err := systemCfg.ConfigToFile(sysCfg); err != nil {
		return errors.Trace(err)
	}
	tgt := filepath.Join("/tmp", comp+"_"+uuid.New().String()+".service")
	if err := e.Transfer(sysCfg, tgt, false); err != nil {
		return errors.Annotatef(err, "transfer from %s to %s failed", sysCfg, tgt)
	}
	cmd := fmt.Sprintf("mv %s /etc/systemd/system/%s-%d.service", tgt, comp, port)
	if _, _, err := e.Execute(cmd, true); err != nil {
		return errors.Annotatef(err, "execute: %s", cmd)
	}

	// transfer default config
	pdList := make([]string, 0)
	for _, pd := range i.instance.topo.Endpoints(deployUser) {
		pdList = append(pdList, fmt.Sprintf("%s:%d", pd.IP, pd.ClientPort))
	}
	masterList := make([]string, 0)
	for _, master := range i.instance.topo.TiSparkMasters {
		masterList = append(masterList, fmt.Sprintf("%s:%d", master.Host, master.Port))
	}

	cfg := config.NewTiSparkConfig(pdList).WithMasters(strings.Join(masterList, ",")).
		WithCustomFields(i.GetCustomFields())
	// transfer spark-defaults.conf
	fp := filepath.Join(paths.Cache, fmt.Sprintf("spark-defaults-%s-%d.conf", host, port))
	if err := cfg.ConfigToFile(fp); err != nil {
		return err
	}
	dst := filepath.Join(paths.Deploy, "conf", "spark-defaults.conf")
	if err := e.Transfer(fp, dst, false); err != nil {
		return err
	}

	env := scripts.NewTiSparkEnv(host).
		WithLocalIP(i.GetListenHost()).
		WithMasterPorts(i.usedPorts[0], i.usedPorts[1]).
		WithCustomEnv(i.GetCustomEnvs())
	// transfer spark-env.sh file
	fp = filepath.Join(paths.Cache, fmt.Sprintf("spark-env-%s-%d.sh", host, port))
	if err := env.ScriptToFile(fp); err != nil {
		return err
	}
	// tispark files are all in a "spark" sub-directory of deploy dir
	dst = filepath.Join(paths.Deploy, "conf", "spark-env.sh")
	if err := e.Transfer(fp, dst, false); err != nil {
		return err
	}

	// transfer log4j config (it's not a template but a static file)
	fp = filepath.Join(paths.Cache, fmt.Sprintf("spark-log4j-%s-%d.properties", host, port))
	log4jFile, err := config.GetConfig("spark-log4j.properties.tpl")
	if err != nil {
		return err
	}
	if err := ioutil.WriteFile(fp, log4jFile, 0644); err != nil {
		return err
	}
	dst = filepath.Join(paths.Deploy, "conf", "log4j.properties")
	return e.Transfer(fp, dst, false)
}

// ScaleConfig deploy temporary config on scaling
func (i *TiSparkMasterInstance) ScaleConfig(e executor.Executor, topo Topology,
	clusterName, clusterVersion, deployUser string, paths meta.DirPaths) error {
	s := i.instance.topo
	defer func() { i.instance.topo = s }()
	cluster := mustBeClusterTopo(topo)
	i.instance.topo = cluster.Merge(i.instance.topo)
	return i.InitConfig(e, clusterName, clusterVersion, deployUser, paths)
}

// TiSparkWorkerComponent represents TiSpark slave component.
type TiSparkWorkerComponent struct{ *Specification }

// Name implements Component interface.
func (c *TiSparkWorkerComponent) Name() string {
	return ComponentTiSpark
}

// Instances implements Component interface.
func (c *TiSparkWorkerComponent) Instances() []Instance {
	ins := make([]Instance, 0, len(c.TiSparkWorkers))
	for _, s := range c.TiSparkWorkers {
		ins = append(ins, &TiSparkWorkerInstance{
			instance: instance{
				InstanceSpec: s,
				name:         c.Name(),
				host:         s.Host,
				port:         s.Port,
				sshp:         s.SSHPort,
				topo:         c.Specification,

				usedPorts: []int{
					s.Port,
					s.WebPort,
				},
				usedDirs: []string{
					s.DeployDir,
				},
				statusFn: s.Status,
			},
		})
	}
	return ins
}

// TiSparkWorkerInstance represent the TiSpark slave instance
type TiSparkWorkerInstance struct {
	instance
}

// GetJavaHome returns the java_home value in spec
func (i *TiSparkWorkerInstance) GetJavaHome() string {
	return reflect.ValueOf(i.InstanceSpec).FieldByName("JavaHome").String()
}

// InitConfig implement Instance interface
func (i *TiSparkWorkerInstance) InitConfig(e executor.Executor, clusterName, clusterVersion, deployUser string, paths meta.DirPaths) error {
	// generate systemd service to invoke spark's start/stop scripts
	comp := i.Role()
	host := i.GetHost()
	port := i.GetPort()
	sysCfg := filepath.Join(paths.Cache, fmt.Sprintf("%s-%s-%d.service", comp, host, port))

	systemCfg := system.NewTiSparkConfig(comp, deployUser, paths.Deploy, i.GetJavaHome())

	if err := systemCfg.ConfigToFile(sysCfg); err != nil {
		return errors.Trace(err)
	}
	tgt := filepath.Join("/tmp", comp+"_"+uuid.New().String()+".service")
	if err := e.Transfer(sysCfg, tgt, false); err != nil {
		return errors.Annotatef(err, "transfer from %s to %s failed", sysCfg, tgt)
	}
	cmd := fmt.Sprintf("mv %s /etc/systemd/system/%s-%d.service", tgt, comp, port)
	if _, _, err := e.Execute(cmd, true); err != nil {
		return errors.Annotatef(err, "execute: %s", cmd)
	}

	// transfer default config
	pdList := make([]string, 0)
	for _, pd := range i.instance.topo.Endpoints(deployUser) {
		pdList = append(pdList, fmt.Sprintf("%s:%d", pd.IP, pd.ClientPort))
	}
	masterList := make([]string, 0)
	for _, master := range i.instance.topo.TiSparkMasters {
		masterList = append(masterList, fmt.Sprintf("%s:%d", master.Host, master.Port))
	}

	cfg := config.NewTiSparkConfig(pdList).WithMasters(strings.Join(masterList, ",")).
		WithCustomFields(i.instance.topo.TiSparkMasters[0].SparkConfigs)
	// transfer spark-defaults.conf
	fp := filepath.Join(paths.Cache, fmt.Sprintf("spark-defaults-%s-%d.conf", host, port))
	if err := cfg.ConfigToFile(fp); err != nil {
		return err
	}
	dst := filepath.Join(paths.Deploy, "conf", "spark-defaults.conf")
	if err := e.Transfer(fp, dst, false); err != nil {
		return err
	}

	env := scripts.NewTiSparkEnv(i.topo.TiSparkMasters[0].Host).
		WithLocalIP(i.GetListenHost()).
		WithMasterPorts(i.topo.TiSparkMasters[0].Port, i.topo.TiSparkMasters[0].WebPort).
		WithWorkerPorts(i.usedPorts[0], i.usedPorts[1]).
		WithCustomEnv(i.instance.topo.TiSparkMasters[0].SparkEnvs)
	// transfer spark-env.sh file
	fp = filepath.Join(paths.Cache, fmt.Sprintf("spark-env-%s-%d.sh", host, port))
	if err := env.ScriptToFile(fp); err != nil {
		return err
	}
	// tispark files are all in a "spark" sub-directory of deploy dir
	dst = filepath.Join(paths.Deploy, "conf", "spark-env.sh")
	if err := e.Transfer(fp, dst, false); err != nil {
		return err
	}

	// transfer start-slave.sh
	fp = filepath.Join(paths.Cache, fmt.Sprintf("start-tispark-slave-%s-%d.sh", host, port))
	slaveSh, err := env.SlaveScriptWithTemplate()
	if err != nil {
		return err
	}
	if err := ioutil.WriteFile(fp, slaveSh, 0755); err != nil {
		return err
	}
	dst = filepath.Join(paths.Deploy, "sbin", "start-slave.sh")
	if err := e.Transfer(fp, dst, false); err != nil {
		return err
	}

	// transfer log4j config (it's not a template but a static file)
	fp = filepath.Join(paths.Cache, fmt.Sprintf("spark-log4j-%s-%d.properties", host, port))
	log4jFile, err := config.GetConfig("spark-log4j.properties.tpl")
	if err != nil {
		return err
	}
	if err := ioutil.WriteFile(fp, log4jFile, 0644); err != nil {
		return err
	}
	dst = filepath.Join(paths.Deploy, "conf", "log4j.properties")
	return e.Transfer(fp, dst, false)
}

// ScaleConfig deploy temporary config on scaling
func (i *TiSparkWorkerInstance) ScaleConfig(e executor.Executor, topo Topology,
	clusterName, clusterVersion, deployUser string, paths meta.DirPaths) error {
	s := i.instance.topo
	defer func() { i.instance.topo = s }()
	cluster := mustBeClusterTopo(topo)
	i.instance.topo = cluster.Merge(i.instance.topo)
	return i.InitConfig(e, clusterName, clusterVersion, deployUser, paths)
}
