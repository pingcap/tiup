// Copyright 2023 PingCAP, Inc.
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
	"bytes"
	"context"
	"crypto/tls"
	"fmt"
	"path/filepath"
	"strings"
	"time"

	"github.com/pingcap/tiup/pkg/cluster/ctxt"
	"github.com/pingcap/tiup/pkg/cluster/template/scripts"
	"github.com/pingcap/tiup/pkg/meta"
	"github.com/pingcap/tiup/pkg/utils"
	"github.com/prometheus/common/expfmt"
)

func proxyUptimeByHost(host string, port int, timeout time.Duration, tlsCfg *tls.Config) time.Duration {
	if timeout < time.Second {
		timeout = statusQueryTimeout
	}

	scheme := "http"
	if tlsCfg != nil {
		scheme = "https"
	}
	url := fmt.Sprintf("%s://%s/api/metrics", scheme, utils.JoinHostPort(host, port))

	client := utils.NewHTTPClient(timeout, tlsCfg)

	body, err := client.Get(context.TODO(), url)
	if err != nil || body == nil {
		return 0
	}

	var parser expfmt.TextParser
	reader := bytes.NewReader(body)
	mf, err := parser.TextToMetricFamilies(reader)
	if err != nil {
		return 0
	}

	now := time.Now()
	for k, v := range mf {
		if k == promMetricStartTimeSeconds {
			ms := v.GetMetric()
			if len(ms) >= 1 {
				startTime := ms[0].Gauge.GetValue()
				return now.Sub(time.Unix(int64(startTime), 0))
			}
			return 0
		}
	}

	return 0
}

// TiProxySpec represents the TiProxy topology specification in topology.yaml
type TiProxySpec struct {
	Host       string         `yaml:"host"`
	ManageHost string         `yaml:"manage_host,omitempty" validate:"manage_host:editable"`
	SSHPort    int            `yaml:"ssh_port,omitempty" validate:"ssh_port:editable"`
	Port       int            `yaml:"port" default:"6000"`
	StatusPort int            `yaml:"status_port" default:"3080"`
	DeployDir  string         `yaml:"deploy_dir,omitempty"`
	NumaNode   string         `yaml:"numa_node,omitempty" validate:"numa_node:editable"`
	Config     map[string]any `yaml:"config,omitempty" validate:"config:ignore"`
	Arch       string         `yaml:"arch,omitempty"`
	OS         string         `yaml:"os,omitempty"`
}

// Role returns the component role of the instance
func (s *TiProxySpec) Role() string {
	return ComponentTiProxy
}

// SSH returns the host and SSH port of the instance
func (s *TiProxySpec) SSH() (string, int) {
	return s.Host, s.SSHPort
}

// GetMainPort returns the main port of the instance
func (s *TiProxySpec) GetMainPort() int {
	return s.Port
}

// GetManageHost returns the manage host of the instance
func (s *TiProxySpec) GetManageHost() string {
	if s.ManageHost != "" {
		return s.ManageHost
	}
	return s.Host
}

// IsImported returns if the node is imported from TiDB-Ansible
func (s *TiProxySpec) IsImported() bool {
	return false
}

// IgnoreMonitorAgent returns if the node does not have monitor agents available
func (s *TiProxySpec) IgnoreMonitorAgent() bool {
	return false
}

// TiProxyComponent represents TiProxy component.
type TiProxyComponent struct{ Topology *Specification }

// Name implements Component interface.
func (c *TiProxyComponent) Name() string {
	return ComponentTiProxy
}

// Role implements Component interface.
func (c *TiProxyComponent) Role() string {
	return ComponentTiProxy
}

// Source implements Component interface.
func (c *TiProxyComponent) Source() string {
	return ComponentTiProxy
}

// CalculateVersion implements the Component interface
func (c *TiProxyComponent) CalculateVersion(clusterVersion string) string {
	version := c.Topology.ComponentVersions.TiProxy
	if version == "" {
		// always not follow global version
		// because tiproxy version is different from clusterVersion
		// but "nightly" is effective
		if clusterVersion == "nightly" {
			version = clusterVersion
		}
	}
	return version
}

// SetVersion implements Component interface.
func (c *TiProxyComponent) SetVersion(version string) {
	c.Topology.ComponentVersions.TiProxy = version
}

// Instances implements Component interface.
func (c *TiProxyComponent) Instances() []Instance {
	ins := make([]Instance, 0, len(c.Topology.TiProxyServers))
	for _, s := range c.Topology.TiProxyServers {
		s := s
		instance := &TiProxyInstance{BaseInstance{
			InstanceSpec: s,
			Name:         c.Name(),
			Host:         s.Host,
			ManageHost:   s.ManageHost,
			ListenHost:   c.Topology.BaseTopo().GlobalOptions.ListenHost,
			Port:         s.Port,
			SSHP:         s.SSHPort,
			NumaNode:     s.NumaNode,
			NumaCores:    "",
			Ports: []int{
				s.Port,
				s.StatusPort,
			},
			Dirs: []string{
				s.DeployDir,
			},
			StatusFn: func(_ context.Context, timeout time.Duration, tlsCfg *tls.Config, _ ...string) string {
				return statusByHost(s.Host, s.StatusPort, "/api/debug/health", timeout, tlsCfg)
			},
			UptimeFn: func(_ context.Context, timeout time.Duration, tlsCfg *tls.Config) time.Duration {
				return proxyUptimeByHost(s.Host, s.StatusPort, timeout, tlsCfg)
			},
			Component: c,
		}, c.Topology}

		ins = append(ins, instance)
	}
	return ins
}

// TiProxyInstance represent the TiProxy instance.
type TiProxyInstance struct {
	BaseInstance
	topo Topology
}

// ScaleConfig deploy temporary config on scaling
func (i *TiProxyInstance) ScaleConfig(
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

func (i *TiProxyInstance) checkConfig(
	cfg map[string]any,
	paths meta.DirPaths,
) map[string]any {
	topo := i.topo.(*Specification)
	spec := i.InstanceSpec.(*TiProxySpec)

	if cfg == nil {
		cfg = make(map[string]any)
	}

	pds := []string{}
	for _, pdspec := range topo.PDServers {
		pds = append(pds, utils.JoinHostPort(pdspec.Host, pdspec.ClientPort))
	}
	cfg["proxy.pd-addrs"] = strings.Join(pds, ",")
	cfg["proxy.addr"] = utils.JoinHostPort(i.GetListenHost(), i.GetPort())
	cfg["proxy.advertise-addr"] = spec.Host
	cfg["api.addr"] = utils.JoinHostPort(i.GetListenHost(), spec.StatusPort)
	cfg["log.log-file.filename"] = filepath.Join(paths.Log, "tiproxy.log")

	return cfg
}

// InitConfig implements Instance interface.
func (i *TiProxyInstance) InitConfig(
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
	spec := i.InstanceSpec.(*TiProxySpec)
	globalConfig := topo.ServerConfigs.TiProxy
	instanceConfig := i.checkConfig(spec.Config, paths)

	cfg := &scripts.TiProxyScript{
		DeployDir: paths.Deploy,
		NumaNode:  spec.NumaNode,
	}

	fp := filepath.Join(paths.Cache, fmt.Sprintf("run_tiproxy_%s_%d.sh", i.GetHost(), i.GetPort()))

	if err := cfg.ConfigToFile(fp); err != nil {
		return err
	}
	dst := filepath.Join(paths.Deploy, "scripts", "run_tiproxy.sh")
	if err := e.Transfer(ctx, fp, dst, false, 0, false); err != nil {
		return err
	}

	if _, _, err := e.Execute(ctx, "chmod +x "+dst, false); err != nil {
		return err
	}

	var err error
	instanceConfig, err = i.setTLSConfig(ctx, topo.GlobalOptions.TLSEnabled, instanceConfig, paths)
	if err != nil {
		return err
	}

	return i.MergeServerConfig(ctx, e, globalConfig, instanceConfig, paths)
}

// setTLSConfig set TLS Config to support enable/disable TLS
func (i *TiProxyInstance) setTLSConfig(ctx context.Context, enableTLS bool, configs map[string]any, paths meta.DirPaths) (map[string]any, error) {
	if configs == nil {
		configs = make(map[string]any)
	}
	if enableTLS {
		configs["security.cluster-tls.ca"] = fmt.Sprintf("%s/tls/%s", paths.Deploy, TLSCACert)
		configs["security.cluster-tls.cert"] = fmt.Sprintf("%s/tls/%s.crt", paths.Deploy, i.Role())
		configs["security.cluster-tls.key"] = fmt.Sprintf("%s/tls/%s.pem", paths.Deploy, i.Role())

		configs["security.server-http-tls.ca"] = fmt.Sprintf("%s/tls/%s", paths.Deploy, TLSCACert)
		configs["security.server-http-tls.cert"] = fmt.Sprintf("%s/tls/%s.crt", paths.Deploy, i.Role())
		configs["security.server-http-tls.key"] = fmt.Sprintf("%s/tls/%s.pem", paths.Deploy, i.Role())
		configs["security.server-http-tls.skip-ca"] = true

		configs["security.sql-tls.ca"] = fmt.Sprintf("%s/tls/%s", paths.Deploy, TLSCACert)
		configs["security.sql-tls.cert"] = fmt.Sprintf("%s/tls/%s.crt", paths.Deploy, i.Role())
		configs["security.sql-tls.key"] = fmt.Sprintf("%s/tls/%s.pem", paths.Deploy, i.Role())
	} else {
		// drainer tls config list
		tlsConfigs := []string{
			"security.cluster-tls.ca",
			"security.cluster-tls.cert",
			"security.cluster-tls.key",
			"security.server-tls.ca",
			"security.server-tls.cert",
			"security.server-tls.key",
			"security.server-tls.skip-ca",
			"security.server-http-tls.ca",
			"security.server-http-tls.cert",
			"security.server-http-tls.key",
			"security.server-http-tls.skip-ca",
			"security.sql-tls.ca",
			"security.sql-tls.cert",
			"security.sql-tls.key",
		}
		// delete TLS configs
		for _, config := range tlsConfigs {
			delete(configs, config)
		}
	}

	return configs, nil
}

var _ RollingUpdateInstance = &TiProxyInstance{}

// GetAddr return the address of this TiProxy instance
func (i *TiProxyInstance) GetAddr() string {
	return utils.JoinHostPort(i.GetHost(), i.GetPort())
}

// PreRestart implements RollingUpdateInstance interface.
func (i *TiProxyInstance) PreRestart(ctx context.Context, topo Topology, apiTimeoutSeconds int, tlsCfg *tls.Config) error {
	return nil
}

// PostRestart implements RollingUpdateInstance interface.
func (i *TiProxyInstance) PostRestart(ctx context.Context, topo Topology, tlsCfg *tls.Config) error {
	return nil
}
