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
	"time"

	"github.com/pingcap/errors"
	"github.com/pingcap/tiup/pkg/cluster/api"
	"github.com/pingcap/tiup/pkg/cluster/ctxt"
	"github.com/pingcap/tiup/pkg/cluster/template/scripts"
	"github.com/pingcap/tiup/pkg/logger/log"
	"github.com/pingcap/tiup/pkg/meta"
	"github.com/pingcap/tiup/pkg/utils"
)

// PDSpec represents the PD topology specification in topology.yaml
type PDSpec struct {
	Host                string `yaml:"host"`
	ListenHost          string `yaml:"listen_host,omitempty"`
	AdvertiseClientAddr string `yaml:"advertise_client_addr,omitempty"`
	AdvertisePeerAddr   string `yaml:"advertise_peer_addr,omitempty"`
	SSHPort             int    `yaml:"ssh_port,omitempty" validate:"ssh_port:editable"`
	Imported            bool   `yaml:"imported,omitempty"`
	Patched             bool   `yaml:"patched,omitempty"`
	IgnoreExporter      bool   `yaml:"ignore_exporter,omitempty"`
	// Use Name to get the name with a default value if it's empty.
	Name            string                 `yaml:"name"`
	ClientPort      int                    `yaml:"client_port" default:"2379"`
	PeerPort        int                    `yaml:"peer_port" default:"2380"`
	DeployDir       string                 `yaml:"deploy_dir,omitempty"`
	DataDir         string                 `yaml:"data_dir,omitempty"`
	LogDir          string                 `yaml:"log_dir,omitempty"`
	NumaNode        string                 `yaml:"numa_node,omitempty" validate:"numa_node:editable"`
	Config          map[string]interface{} `yaml:"config,omitempty" validate:"config:ignore"`
	ResourceControl meta.ResourceControl   `yaml:"resource_control,omitempty" validate:"resource_control:editable"`
	Arch            string                 `yaml:"arch,omitempty"`
	OS              string                 `yaml:"os,omitempty"`
}

// Status queries current status of the instance
func (s *PDSpec) Status(tlsCfg *tls.Config, _ ...string) string {
	addr := fmt.Sprintf("%s:%d", s.Host, s.ClientPort)
	pc := api.NewPDClient([]string{addr}, statusQueryTimeout, tlsCfg)

	// check health
	err := pc.CheckHealth()
	if err != nil {
		return "Down"
	}

	// find leader node
	leader, err := pc.GetLeader()
	if err != nil {
		return "ERR"
	}
	res := "Up"
	if s.Name == leader.Name {
		res += "|L"
	}
	return res
}

// Role returns the component role of the instance
func (s *PDSpec) Role() string {
	return ComponentPD
}

// SSH returns the host and SSH port of the instance
func (s *PDSpec) SSH() (string, int) {
	return s.Host, s.SSHPort
}

// GetMainPort returns the main port of the instance
func (s *PDSpec) GetMainPort() int {
	return s.ClientPort
}

// IsImported returns if the node is imported from TiDB-Ansible
func (s *PDSpec) IsImported() bool {
	return s.Imported
}

// IgnoreMonitorAgent returns if the node does not have monitor agents available
func (s *PDSpec) IgnoreMonitorAgent() bool {
	return s.IgnoreExporter
}

// PDComponent represents PD component.
type PDComponent struct{ Topology *Specification }

// Name implements Component interface.
func (c *PDComponent) Name() string {
	return ComponentPD
}

// Role implements Component interface.
func (c *PDComponent) Role() string {
	return ComponentPD
}

// Instances implements Component interface.
func (c *PDComponent) Instances() []Instance {
	ins := make([]Instance, 0, len(c.Topology.PDServers))
	for _, s := range c.Topology.PDServers {
		s := s
		ins = append(ins, &PDInstance{
			Name: s.Name,
			BaseInstance: BaseInstance{
				InstanceSpec: s,
				Name:         c.Name(),
				Host:         s.Host,
				ListenHost:   s.ListenHost,
				Port:         s.ClientPort,
				SSHP:         s.SSHPort,

				Ports: []int{
					s.ClientPort,
					s.PeerPort,
				},
				Dirs: []string{
					s.DeployDir,
					s.DataDir,
				},
				StatusFn: s.Status,
				UptimeFn: func(tlsCfg *tls.Config) time.Duration {
					return UptimeByHost(s.Host, s.ClientPort, tlsCfg)
				},
			},
			topo: c.Topology,
		})
	}
	return ins
}

// PDInstance represent the PD instance
type PDInstance struct {
	Name string
	BaseInstance
	topo Topology
}

// InitConfig implement Instance interface
func (i *PDInstance) InitConfig(
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
	spec := i.InstanceSpec.(*PDSpec)
	cfg := scripts.
		NewPDScript(spec.Name, i.GetHost(), paths.Deploy, paths.Data[0], paths.Log).
		WithNumaNode(spec.NumaNode).
		WithClientPort(spec.ClientPort).
		WithPeerPort(spec.PeerPort).
		AppendEndpoints(topo.Endpoints(deployUser)...).
		WithListenHost(i.GetListenHost())

	if enableTLS {
		cfg = cfg.WithScheme("https")
	}
	cfg = cfg.WithAdvertiseClientAddr(spec.AdvertiseClientAddr).
		WithAdvertisePeerAddr(spec.AdvertisePeerAddr)

	fp := filepath.Join(paths.Cache, fmt.Sprintf("run_pd_%s_%d.sh", i.GetHost(), i.GetPort()))
	if err := cfg.ConfigToFile(fp); err != nil {
		return err
	}
	dst := filepath.Join(paths.Deploy, "scripts", "run_pd.sh")
	if err := e.Transfer(ctx, fp, dst, false, 0, false); err != nil {
		return err
	}
	_, _, err := e.Execute(ctx, "chmod +x "+dst, false)
	if err != nil {
		return err
	}

	globalConfig := topo.ServerConfigs.PD
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
	spec.Config, err = i.setTLSConfig(ctx, enableTLS, spec.Config, paths)
	if err != nil {
		return err
	}

	if err := i.MergeServerConfig(ctx, e, globalConfig, spec.Config, paths); err != nil {
		return err
	}

	return checkConfig(ctx, e, i.ComponentName(), clusterVersion, i.OS(), i.Arch(), i.ComponentName()+".toml", paths, nil)
}

// setTLSConfig set TLS Config to support enable/disable TLS
func (i *PDInstance) setTLSConfig(ctx context.Context, enableTLS bool, configs map[string]interface{}, paths meta.DirPaths) (map[string]interface{}, error) {
	// set TLS configs
	if enableTLS {
		if configs == nil {
			configs = make(map[string]interface{})
		}
		configs["security.cacert-path"] = fmt.Sprintf(
			"%s/tls/%s",
			paths.Deploy,
			TLSCACert,
		)
		configs["security.cert-path"] = fmt.Sprintf(
			"%s/tls/%s.crt",
			paths.Deploy,
			i.Role())
		configs["security.key-path"] = fmt.Sprintf(
			"%s/tls/%s.pem",
			paths.Deploy,
			i.Role())
	} else {
		// drainer tls config list
		tlsConfigs := []string{
			"security.cacert-path",
			"security.cert-path",
			"security.key-path",
		}
		// delete TLS configs
		if configs != nil {
			for _, config := range tlsConfigs {
				delete(configs, config)
			}
		}
	}

	return configs, nil
}

// ScaleConfig deploy temporary config on scaling
func (i *PDInstance) ScaleConfig(
	ctx context.Context,
	e ctxt.Executor,
	topo Topology,
	clusterName,
	clusterVersion,
	deployUser string,
	paths meta.DirPaths,
) error {
	// We need pd.toml here, but we don't need to check it
	if err := i.InitConfig(ctx, e, clusterName, clusterVersion, deployUser, paths); err != nil &&
		errors.Cause(err) != ErrorCheckConfig {
		return err
	}

	cluster := mustBeClusterTopo(topo)
	spec := i.InstanceSpec.(*PDSpec)
	cfg0 := scripts.NewPDScript(
		i.Name,
		i.GetHost(),
		paths.Deploy,
		paths.Data[0],
		paths.Log,
	).WithPeerPort(spec.PeerPort).
		WithNumaNode(spec.NumaNode).
		WithClientPort(spec.ClientPort).
		AppendEndpoints(cluster.Endpoints(deployUser)...).
		WithListenHost(i.GetListenHost())
	if topo.BaseTopo().GlobalOptions.TLSEnabled {
		cfg0 = cfg0.WithScheme("https")
	}
	cfg0 = cfg0.WithAdvertiseClientAddr(spec.AdvertiseClientAddr).
		WithAdvertisePeerAddr(spec.AdvertisePeerAddr)
	cfg := scripts.NewPDScaleScript(cfg0)

	fp := filepath.Join(paths.Cache, fmt.Sprintf("run_pd_%s_%d.sh", i.GetHost(), i.GetPort()))
	log.Infof("script path: %s", fp)
	if err := cfg.ConfigToFile(fp); err != nil {
		return err
	}

	dst := filepath.Join(paths.Deploy, "scripts", "run_pd.sh")
	if err := e.Transfer(ctx, fp, dst, false, 0, false); err != nil {
		return err
	}
	if _, _, err := e.Execute(ctx, "chmod +x "+dst, false); err != nil {
		return err
	}
	return nil
}

var _ RollingUpdateInstance = &PDInstance{}

// IsLeader checks if the instance is PD leader
func (i *PDInstance) IsLeader(topo Topology, apiTimeoutSeconds int, tlsCfg *tls.Config) (bool, error) {
	tidbTopo, ok := topo.(*Specification)
	if !ok {
		panic("topo should be type of tidb topology")
	}
	pdClient := api.NewPDClient(tidbTopo.GetPDList(), time.Second*5, tlsCfg)

	return i.isLeader(pdClient)
}

func (i *PDInstance) isLeader(pdClient *api.PDClient) (bool, error) {
	leader, err := pdClient.GetLeader()
	if err != nil {
		return false, errors.Annotatef(err, "failed to get PD leader %s", i.GetHost())
	}

	return leader.Name == i.Name, nil
}

// PreRestart implements RollingUpdateInstance interface.
func (i *PDInstance) PreRestart(topo Topology, apiTimeoutSeconds int, tlsCfg *tls.Config) error {
	timeoutOpt := &utils.RetryOption{
		Timeout: time.Second * time.Duration(apiTimeoutSeconds),
		Delay:   time.Second * 2,
	}

	tidbTopo, ok := topo.(*Specification)
	if !ok {
		panic("topo should be type of tidb topology")
	}
	pdClient := api.NewPDClient(tidbTopo.GetPDList(), time.Second*5, tlsCfg)

	isLeader, err := i.isLeader(pdClient)
	if err != nil {
		return err
	}
	if len(tidbTopo.PDServers) > 1 && isLeader {
		if err := pdClient.EvictPDLeader(timeoutOpt); err != nil {
			return errors.Annotatef(err, "failed to evict PD leader %s", i.GetHost())
		}
	}

	return nil
}

// PostRestart implements RollingUpdateInstance interface.
func (i *PDInstance) PostRestart(topo Topology, tlsCfg *tls.Config) error {
	// When restarting the next PD, if the PD has not been fully started and has become the target of
	// the transfer leader, this may cause the PD service to be unavailable for about 10 seconds.

	timeoutOpt := utils.RetryOption{
		Attempts: 100,
		Delay:    time.Second,
		Timeout:  120 * time.Second,
	}
	currentPDAddrs := []string{fmt.Sprintf("%s:%d", i.Host, i.Port)}
	pdClient := api.NewPDClient(currentPDAddrs, 5*time.Second, tlsCfg)

	if err := utils.Retry(pdClient.CheckHealth, timeoutOpt); err != nil {
		return errors.Annotatef(err, "failed to start PD peer %s", i.GetHost())
	}

	return nil
}
