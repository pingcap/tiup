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

	"github.com/pingcap/tiup/pkg/cluster/executor"
	"github.com/pingcap/tiup/pkg/cluster/template/config"
	"github.com/pingcap/tiup/pkg/cluster/template/scripts"
	"github.com/pingcap/tiup/pkg/meta"
)

// TiSparkMasterSpec is the topology specification for TiSpark master node
type TiSparkMasterSpec struct {
	Host         string                 `yaml:"host"`
	SSHPort      int                    `yaml:"ssh_port,omitempty"`
	Imported     bool                   `yaml:"imported,omitempty"`
	Port         int                    `yaml:"port" default:"7077"`
	WebPort      int                    `yaml:"web_port" default:"8080"`
	DeployDir    string                 `yaml:"deploy_dir,omitempty"`
	SparkConfigs map[string]interface{} `yaml:"spark_config",omitempty`
	SparkEnvs    map[string]string      `yaml:"spark_env",omitempty`
	Arch         string                 `yaml:"arch,omitempty"`
	OS           string                 `yaml:"os,omitempty"`
}

// Role returns the component role of the instance
func (s TiSparkMasterSpec) Role() string {
	return ComponentTiSpark
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

// TiSparkSlaveSpec is the topology specification for TiSpark slave nodes
type TiSparkSlaveSpec struct {
	Host         string                 `yaml:"host"`
	SSHPort      int                    `yaml:"ssh_port,omitempty"`
	Imported     bool                   `yaml:"imported,omitempty"`
	Port         int                    `yaml:"port" default:"7078"`
	WebPort      int                    `yaml:"web_port" default:"8081"`
	DeployDir    string                 `yaml:"deploy_dir,omitempty"`
	SparkConfigs map[string]interface{} `yaml:"spark_config",omitempty`
	SparkEnvs    map[string]string      `yaml:"spark_env",omitempty`
	Arch         string                 `yaml:"arch,omitempty"`
	OS           string                 `yaml:"os,omitempty"`
}

// Role returns the component role of the instance
func (s TiSparkSlaveSpec) Role() string {
	return ComponentTiSpark
}

// SSH returns the host and SSH port of the instance
func (s TiSparkSlaveSpec) SSH() (string, int) {
	return s.Host, s.SSHPort
}

// GetMainPort returns the main port of the instance
func (s TiSparkSlaveSpec) GetMainPort() int {
	return s.Port
}

// IsImported returns if the node is imported from TiDB-Ansible
func (s TiSparkSlaveSpec) IsImported() bool {
	return s.Imported
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
				statusFn: func(_ ...string) string {
					return "-"
				},
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

// InitConfig implement Instance interface
func (i *TiSparkMasterInstance) InitConfig(e executor.Executor, clusterName, clusterVersion, deployUser string, paths meta.DirPaths) error {
	if err := i.instance.InitConfig(e, clusterName, clusterVersion, deployUser, paths); err != nil {
		return err
	}

	// transfer default config
	pdList := make([]string, 0)
	for _, pd := range i.instance.topo.Endpoints(deployUser) {
		pdList = append(pdList, fmt.Sprintf("%s:%d", pd.IP, pd.ClientPort))
	}
	masterList := make([]string, 0)
	for _, m := range i.topo.TiSparkMasters {
		masterList = append(masterList, fmt.Sprintf("%s:%d", m.Host, m.Port))
	}

	cfg := config.NewTiSparkConfig(pdList).WithMasters(masterList).
		WithCustomFields(i.GetCustomFields())
	// transfer spark-defaults.conf
	fp := filepath.Join(paths.Cache, fmt.Sprintf("spark-defaults-%s-%d.conf", i.GetHost(), i.GetPort()))
	if err := cfg.ConfigToFile(fp); err != nil {
		return err
	}
	// tispark files are all in a "spark" sub-directory of deploy dir
	dst := filepath.Join(paths.Deploy, "spark", "conf", "spark-defaults.conf")
	if err := e.Transfer(fp, dst, false); err != nil {
		return err
	}

	env := scripts.NewTiSparkEnv(masterList).WithCustomEnv(i.GetCustomEnvs())
	// transfer spark-env.sh file
	fp = filepath.Join(paths.Cache, fmt.Sprintf("spark-env-%s-%d.sh", i.GetHost(), i.GetPort()))
	if err := env.ScriptToFile(fp); err != nil {
		return err
	}
	// tispark files are all in a "spark" sub-directory of deploy dir
	dst = filepath.Join(paths.Deploy, "spark", "conf", "spark-env.sh")
	if err := e.Transfer(fp, dst, false); err != nil {
		return err
	}

	// transfer log4j config (it's not a template but a static file)
	fp = filepath.Join(paths.Cache, fmt.Sprintf("spark-log4j-%s-%d.properties", i.GetHost(), i.GetPort()))
	log4jFile, err := config.GetConfig("spark-log4j.properties.tpl")
	if err != nil {
		return err
	}
	if err := ioutil.WriteFile(fp, log4jFile, 0644); err != nil {
		return err
	}
	dst = filepath.Join(paths.Deploy, "spark", "conf", "log4j.properties")
	if err := e.Transfer(fp, dst, false); err != nil {
		return err
	}

	return nil
}

// ScaleConfig deploy temporary config on scaling
func (i *TiSparkMasterInstance) ScaleConfig(e executor.Executor, cluster *Specification,
	clusterName, clusterVersion, deployUser string, paths meta.DirPaths) error {
	s := i.instance.topo
	defer func() { i.instance.topo = s }()
	i.instance.topo = cluster.Merge(i.instance.topo)
	return i.InitConfig(e, clusterName, clusterVersion, deployUser, paths)
}

// TiSparkSlaveComponent represents TiSpark slave component.
type TiSparkSlaveComponent struct{ *Specification }

// Name implements Component interface.
func (c *TiSparkSlaveComponent) Name() string {
	return ComponentTiSpark
}

// Instances implements Component interface.
func (c *TiSparkSlaveComponent) Instances() []Instance {
	ins := make([]Instance, 0, len(c.TiSparkSlaves))
	for _, s := range c.TiSparkSlaves {
		ins = append(ins, &TiSparkSlaveInstance{
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
				statusFn: func(_ ...string) string {
					return "-"
				},
			},
		})
	}
	return ins
}

// TiSparkSlaveInstance represent the TiSpark slave instance
type TiSparkSlaveInstance struct {
	instance
}

// GetCustomFields get custom spark configs of the instance
func (i *TiSparkSlaveInstance) GetCustomFields() map[string]interface{} {
	v := reflect.ValueOf(i.InstanceSpec).FieldByName("SparkConfigs")
	if !v.IsValid() {
		return nil
	}
	return v.Interface().(map[string]interface{})
}

// GetCustomEnvs get custom spark envionment variables of the instance
func (i *TiSparkSlaveInstance) GetCustomEnvs() map[string]string {
	v := reflect.ValueOf(i.InstanceSpec).FieldByName("SparkEnvs")
	if !v.IsValid() {
		return nil
	}
	return v.Interface().(map[string]string)
}

// InitConfig implement Instance interface
func (i *TiSparkSlaveInstance) InitConfig(e executor.Executor, clusterName, clusterVersion, deployUser string, paths meta.DirPaths) error {
	if err := i.instance.InitConfig(e, clusterName, clusterVersion, deployUser, paths); err != nil {
		return err
	}

	// transfer default config
	pdList := make([]string, 0)
	for _, pd := range i.instance.topo.Endpoints(deployUser) {
		pdList = append(pdList, fmt.Sprintf("%s:%d", pd.IP, pd.ClientPort))
	}
	masterList := make([]string, 0)
	for _, m := range i.topo.TiSparkMasters {
		masterList = append(masterList, fmt.Sprintf("%s:%d", m.Host, m.Port))
	}

	cfg := config.NewTiSparkConfig(pdList).WithMasters(masterList).
		WithCustomFields(i.GetCustomFields())
	// transfer spark-defaults.conf
	fp := filepath.Join(paths.Cache, fmt.Sprintf("spark-defaults-%s-%d.conf", i.GetHost(), i.GetPort()))
	if err := cfg.ConfigToFile(fp); err != nil {
		return err
	}
	// tispark files are all in a "spark" sub-directory of deploy dir
	dst := filepath.Join(paths.Deploy, "spark", "conf", "spark-defaults.conf")
	if err := e.Transfer(fp, dst, false); err != nil {
		return err
	}

	env := scripts.NewTiSparkEnv(masterList).WithCustomEnv(i.GetCustomEnvs())
	// transfer spark-env.sh file
	fp = filepath.Join(paths.Cache, fmt.Sprintf("spark-env-%s-%d.sh", i.GetHost(), i.GetPort()))
	if err := env.ScriptToFile(fp); err != nil {
		return err
	}
	// tispark files are all in a "spark" sub-directory of deploy dir
	dst = filepath.Join(paths.Deploy, "spark", "conf", "spark-env.sh")
	if err := e.Transfer(fp, dst, false); err != nil {
		return err
	}

	// transfer start-slave.sh
	fp = filepath.Join(paths.Cache, fmt.Sprintf("start-tispark-slave-%s-%d.sh", i.GetHost(), i.GetPort()))
	slaveSh, err := env.SlaveScriptWithTemplate()
	if err != nil {
		return err
	}
	if err := ioutil.WriteFile(fp, slaveSh, 0755); err != nil {
		return err
	}
	dst = filepath.Join(paths.Deploy, "spark", "sbin", "start-slave.sh")
	if err := e.Transfer(fp, dst, false); err != nil {
		return err
	}

	// transfer log4j config (it's not a template but a static file)
	fp = filepath.Join(paths.Cache, fmt.Sprintf("spark-log4j-%s-%d.properties", i.GetHost(), i.GetPort()))
	log4jFile, err := config.GetConfig("spark-log4j.properties.tpl")
	if err != nil {
		return err
	}
	if err := ioutil.WriteFile(fp, log4jFile, 0644); err != nil {
		return err
	}
	dst = filepath.Join(paths.Deploy, "spark", "conf", "log4j.properties")
	if err := e.Transfer(fp, dst, false); err != nil {
		return err
	}

	return nil
}

// ScaleConfig deploy temporary config on scaling
func (i *TiSparkSlaveInstance) ScaleConfig(e executor.Executor, cluster *Specification,
	clusterName, clusterVersion, deployUser string, paths meta.DirPaths) error {
	s := i.instance.topo
	defer func() { i.instance.topo = s }()
	i.instance.topo = cluster.Merge(i.instance.topo)
	return i.InitConfig(e, clusterName, clusterVersion, deployUser, paths)
}
