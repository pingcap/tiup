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
	"strconv"
	"time"

	"github.com/pingcap/tiup/pkg/cluster/ctxt"
	"github.com/pingcap/tiup/pkg/cluster/template/scripts"
	"github.com/pingcap/tiup/pkg/meta"
)

// DrainerSpec represents the Drainer topology specification in topology.yaml
type DrainerSpec struct {
	Host            string                 `yaml:"host"`
	SSHPort         int                    `yaml:"ssh_port,omitempty" validate:"ssh_port:editable"`
	Imported        bool                   `yaml:"imported,omitempty"`
	Patched         bool                   `yaml:"patched,omitempty"`
	IgnoreExporter  bool                   `yaml:"ignore_exporter,omitempty"`
	Port            int                    `yaml:"port" default:"8249"`
	DeployDir       string                 `yaml:"deploy_dir,omitempty"`
	DataDir         string                 `yaml:"data_dir,omitempty"`
	LogDir          string                 `yaml:"log_dir,omitempty"`
	CommitTS        int64                  `yaml:"commit_ts" default:"-1" validate:"commit_ts:editable"`
	Offline         bool                   `yaml:"offline,omitempty"`
	NumaNode        string                 `yaml:"numa_node,omitempty" validate:"numa_node:editable"`
	Config          map[string]interface{} `yaml:"config,omitempty" validate:"config:ignore"`
	ResourceControl meta.ResourceControl   `yaml:"resource_control,omitempty" validate:"resource_control:editable"`
	Arch            string                 `yaml:"arch,omitempty"`
	OS              string                 `yaml:"os,omitempty"`
}

// Role returns the component role of the instance
func (s *DrainerSpec) Role() string {
	return ComponentDrainer
}

// SSH returns the host and SSH port of the instance
func (s *DrainerSpec) SSH() (string, int) {
	return s.Host, s.SSHPort
}

// GetMainPort returns the main port of the instance
func (s *DrainerSpec) GetMainPort() int {
	return s.Port
}

// IsImported returns if the node is imported from TiDB-Ansible
func (s *DrainerSpec) IsImported() bool {
	return s.Imported
}

// IgnoreMonitorAgent returns if the node does not have monitor agents available
func (s *DrainerSpec) IgnoreMonitorAgent() bool {
	return s.IgnoreExporter
}

// DrainerComponent represents Drainer component.
type DrainerComponent struct{ Topology *Specification }

// Name implements Component interface.
func (c *DrainerComponent) Name() string {
	return ComponentDrainer
}

// Role implements Component interface.
func (c *DrainerComponent) Role() string {
	return ComponentDrainer
}

// Instances implements Component interface.
func (c *DrainerComponent) Instances() []Instance {
	ins := make([]Instance, 0, len(c.Topology.Drainers))
	for _, s := range c.Topology.Drainers {
		s := s
		ins = append(ins, &DrainerInstance{BaseInstance{
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
			StatusFn: func(tlsCfg *tls.Config, _ ...string) string {
				return statusByHost(s.Host, s.Port, "/status", tlsCfg)
			},
			UptimeFn: func(tlsCfg *tls.Config) time.Duration {
				return UptimeByHost(s.Host, s.Port, tlsCfg)
			},
		}, c.Topology})
	}
	return ins
}

// DrainerInstance represent the Drainer instance.
type DrainerInstance struct {
	BaseInstance
	topo Topology
}

// ScaleConfig deploy temporary config on scaling
func (i *DrainerInstance) ScaleConfig(
	ctx context.Context,
	e ctxt.Executor,
	topo Topology,
	clusterName,
	clusterVersion,
	user string,
	paths meta.DirPaths,
) error {
	s := i.topo
	defer func() {
		i.topo = s
	}()
	i.topo = mustBeClusterTopo(topo)

	return i.InitConfig(ctx, e, clusterName, clusterVersion, user, paths)
}

// InitConfig implements Instance interface.
func (i *DrainerInstance) InitConfig(
	ctx context.Context,
	e ctxt.Executor,
	clusterName,
	clusterVersion,
	deployUser string,
	paths meta.DirPaths,
) error {
	topo := i.topo.(*Specification)
	if err := i.BaseInstance.InitConfig(ctx, e, topo.GlobalOptions, deployUser, paths); err != nil {
		return err
	}

	enableTLS := topo.GlobalOptions.TLSEnabled
	spec := i.InstanceSpec.(*DrainerSpec)
	nodeID := i.GetHost() + ":" + strconv.Itoa(i.GetPort())
	// keep origin node id if is imported
	if i.IsImported() {
		nodeID = ""
	}
	cfg := scripts.NewDrainerScript(
		nodeID,
		i.GetHost(),
		paths.Deploy,
		paths.Data[0],
		paths.Log,
	).WithPort(spec.Port).WithNumaNode(spec.NumaNode).AppendEndpoints(topo.Endpoints(deployUser)...)

	cfg.WithCommitTs(spec.CommitTS)

	fp := filepath.Join(paths.Cache, fmt.Sprintf("run_drainer_%s_%d.sh", i.GetHost(), i.GetPort()))

	if err := cfg.ConfigToFile(fp); err != nil {
		return err
	}
	dst := filepath.Join(paths.Deploy, "scripts", "run_drainer.sh")
	if err := e.Transfer(ctx, fp, dst, false, 0, false); err != nil {
		return err
	}

	if _, _, err := e.Execute(ctx, "chmod +x "+dst, false); err != nil {
		return err
	}

	globalConfig := topo.ServerConfigs.Drainer
	// merge config files for imported instance
	if i.IsImported() {
		configPath := ClusterPath(
			clusterName,
			AnsibleImportedConfigPath,
			fmt.Sprintf(
				"%s-%s-%d.toml",
				i.ComponentName(),
				i.GetHost(),
				i.GetPort(),
			),
		)
		importConfig, err := os.ReadFile(configPath)
		if err != nil {
			return err
		}
		globalConfig, err = mergeImported(importConfig, globalConfig)
		if err != nil {
			return err
		}
	}

	// set TLS configs
	if enableTLS {
		if spec.Config == nil {
			spec.Config = make(map[string]interface{})
		}
		spec.Config["security.ssl-ca"] = fmt.Sprintf(
			"%s/tls/%s",
			paths.Deploy,
			TLSCACert,
		)
		spec.Config["security.ssl-cert"] = fmt.Sprintf(
			"%s/tls/%s.crt",
			paths.Deploy,
			i.Role())
		spec.Config["security.ssl-key"] = fmt.Sprintf(
			"%s/tls/%s.pem",
			paths.Deploy,
			i.Role())
	}

	if err := i.MergeServerConfig(ctx, e, globalConfig, spec.Config, paths); err != nil {
		return err
	}

	return checkConfig(ctx, e, i.ComponentName(), clusterVersion, i.OS(), i.Arch(), i.ComponentName()+".toml", paths, nil)
}
