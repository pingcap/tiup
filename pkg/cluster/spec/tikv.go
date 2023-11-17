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
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	perrs "github.com/pingcap/errors"
	"github.com/pingcap/tiup/pkg/cluster/api"
	"github.com/pingcap/tiup/pkg/cluster/ctxt"
	"github.com/pingcap/tiup/pkg/cluster/template/scripts"
	logprinter "github.com/pingcap/tiup/pkg/logger/printer"
	"github.com/pingcap/tiup/pkg/meta"
	"github.com/pingcap/tiup/pkg/tidbver"
	"github.com/pingcap/tiup/pkg/utils"
	dto "github.com/prometheus/client_model/go"
	"github.com/prometheus/prom2json"
	"go.uber.org/zap"
)

const (
	metricNameRegionCount = "tikv_raftstore_region_count"
	labelNameLeaderCount  = "leader"
)

// TiKVSpec represents the TiKV topology specification in topology.yaml
type TiKVSpec struct {
	Host                string               `yaml:"host"`
	ManageHost          string               `yaml:"manage_host,omitempty" validate:"manage_host:editable"`
	ListenHost          string               `yaml:"listen_host,omitempty"`
	AdvertiseAddr       string               `yaml:"advertise_addr,omitempty"`
	SSHPort             int                  `yaml:"ssh_port,omitempty" validate:"ssh_port:editable"`
	Imported            bool                 `yaml:"imported,omitempty"`
	Patched             bool                 `yaml:"patched,omitempty"`
	IgnoreExporter      bool                 `yaml:"ignore_exporter,omitempty"`
	Port                int                  `yaml:"port" default:"20160"`
	StatusPort          int                  `yaml:"status_port" default:"20180"`
	AdvertiseStatusAddr string               `yaml:"advertise_status_addr,omitempty"`
	DeployDir           string               `yaml:"deploy_dir,omitempty"`
	DataDir             string               `yaml:"data_dir,omitempty"`
	LogDir              string               `yaml:"log_dir,omitempty"`
	Offline             bool                 `yaml:"offline,omitempty"`
	Source              string               `yaml:"source,omitempty" validate:"source:editable"`
	NumaNode            string               `yaml:"numa_node,omitempty" validate:"numa_node:editable"`
	NumaCores           string               `yaml:"numa_cores,omitempty" validate:"numa_cores:editable"`
	Config              map[string]any       `yaml:"config,omitempty" validate:"config:ignore"`
	ResourceControl     meta.ResourceControl `yaml:"resource_control,omitempty" validate:"resource_control:editable"`
	Arch                string               `yaml:"arch,omitempty"`
	OS                  string               `yaml:"os,omitempty"`
}

// checkStoreStatus checks the store status in current cluster
func checkStoreStatus(ctx context.Context, storeAddr string, tlsCfg *tls.Config, pdList ...string) string {
	if len(pdList) < 1 {
		return "N/A"
	}
	pdapi := api.NewPDClient(ctx, pdList, statusQueryTimeout, tlsCfg)
	store, err := pdapi.GetCurrentStore(storeAddr)
	if err != nil {
		if errors.Is(err, api.ErrNoStore) {
			return "N/A"
		}
		return "Down"
	}

	return store.Store.StateName
}

// Status queries current status of the instance
func (s *TiKVSpec) Status(ctx context.Context, timeout time.Duration, tlsCfg *tls.Config, pdList ...string) string {
	storeAddr := addr(s)
	state := checkStoreStatus(ctx, storeAddr, tlsCfg, pdList...)
	if s.Offline && strings.ToLower(state) == "offline" {
		state = "Pending Offline" // avoid misleading
	}
	return state
}

// Role returns the component role of the instance
func (s *TiKVSpec) Role() string {
	return ComponentTiKV
}

// SSH returns the host and SSH port of the instance
func (s *TiKVSpec) SSH() (string, int) {
	host := s.Host
	if s.ManageHost != "" {
		host = s.ManageHost
	}
	return host, s.SSHPort
}

// GetMainPort returns the main port of the instance
func (s *TiKVSpec) GetMainPort() int {
	return s.Port
}

// GetManageHost returns the manage host of the instance
func (s *TiKVSpec) GetManageHost() string {
	if s.ManageHost != "" {
		return s.ManageHost
	}
	return s.Host
}

// IsImported returns if the node is imported from TiDB-Ansible
func (s *TiKVSpec) IsImported() bool {
	return s.Imported
}

// IgnoreMonitorAgent returns if the node does not have monitor agents available
func (s *TiKVSpec) IgnoreMonitorAgent() bool {
	return s.IgnoreExporter
}

// Labels returns the labels of TiKV
func (s *TiKVSpec) Labels() (map[string]string, error) {
	lbs := make(map[string]string)

	if serverLabels := GetValueFromPath(s.Config, "server.labels"); serverLabels != nil {
		m := map[any]any{}
		if sm, ok := serverLabels.(map[string]any); ok {
			for k, v := range sm {
				m[k] = v
			}
		} else if im, ok := serverLabels.(map[any]any); ok {
			m = im
		}
		for k, v := range m {
			key, ok := k.(string)
			if !ok {
				return nil, perrs.Errorf("TiKV label name %v is not a string, check the instance: %s:%d", k, s.Host, s.GetMainPort())
			}
			value, ok := v.(string)
			if !ok {
				return nil, perrs.Errorf("TiKV label value %v is not a string, check the instance: %s:%d", v, s.Host, s.GetMainPort())
			}

			lbs[key] = value
		}
	}

	return lbs, nil
}

// TiKVComponent represents TiKV component.
type TiKVComponent struct{ Topology *Specification }

// Name implements Component interface.
func (c *TiKVComponent) Name() string {
	return ComponentTiKV
}

// Role implements Component interface.
func (c *TiKVComponent) Role() string {
	return ComponentTiKV
}

// Source implements Component interface.
func (c *TiKVComponent) Source() string {
	source := c.Topology.ComponentSources.TiKV
	if source != "" {
		return source
	}
	return ComponentTiKV
}

// CalculateVersion implements the Component interface
func (c *TiKVComponent) CalculateVersion(clusterVersion string) string {
	version := c.Topology.ComponentVersions.TiKV
	if version == "" {
		version = clusterVersion
	}
	return version
}

// SetVersion implements Component interface.
func (c *TiKVComponent) SetVersion(version string) {
	c.Topology.ComponentVersions.TiKV = version
}

// Instances implements Component interface.
func (c *TiKVComponent) Instances() []Instance {
	ins := make([]Instance, 0, len(c.Topology.TiKVServers))
	for _, s := range c.Topology.TiKVServers {
		s := s
		ins = append(ins, &TiKVInstance{BaseInstance{
			InstanceSpec: s,
			Name:         c.Name(),
			Host:         s.Host,
			ManageHost:   s.ManageHost,
			ListenHost:   utils.Ternary(s.ListenHost != "", s.ListenHost, c.Topology.BaseTopo().GlobalOptions.ListenHost).(string),
			Port:         s.Port,
			SSHP:         s.SSHPort,
			Source:       s.Source,
			NumaNode:     s.NumaNode,
			NumaCores:    s.NumaCores,

			Ports: []int{
				s.Port,
				s.StatusPort,
			},
			Dirs: []string{
				s.DeployDir,
				s.DataDir,
			},
			StatusFn: s.Status,
			UptimeFn: func(_ context.Context, timeout time.Duration, tlsCfg *tls.Config) time.Duration {
				return UptimeByHost(s.GetManageHost(), s.StatusPort, timeout, tlsCfg)
			},
			Component: c,
		}, c.Topology, 0})
	}
	return ins
}

// TiKVInstance represent the TiDB instance
type TiKVInstance struct {
	BaseInstance
	topo                     Topology
	leaderCountBeforeRestart int
}

// InitConfig implement Instance interface
func (i *TiKVInstance) InitConfig(
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
	spec := i.InstanceSpec.(*TiKVSpec)

	pds := []string{}
	for _, pdspec := range topo.PDServers {
		pds = append(pds, utils.JoinHostPort(pdspec.Host, pdspec.ClientPort))
	}
	cfg := &scripts.TiKVScript{
		Addr:                       utils.JoinHostPort(i.GetListenHost(), spec.Port),
		AdvertiseAddr:              utils.Ternary(spec.AdvertiseAddr != "", spec.AdvertiseAddr, utils.JoinHostPort(spec.Host, spec.Port)).(string),
		StatusAddr:                 utils.JoinHostPort(i.GetListenHost(), spec.StatusPort),
		SupportAdvertiseStatusAddr: tidbver.TiKVSupportAdvertiseStatusAddr(clusterVersion),
		AdvertiseStatusAddr:        utils.Ternary(spec.AdvertiseStatusAddr != "", spec.AdvertiseStatusAddr, utils.JoinHostPort(spec.Host, spec.StatusPort)).(string),
		PD:                         strings.Join(pds, ","),

		DeployDir: paths.Deploy,
		DataDir:   paths.Data[0],
		LogDir:    paths.Log,

		NumaNode:  spec.NumaNode,
		NumaCores: spec.NumaCores,
	}

	fp := filepath.Join(paths.Cache, fmt.Sprintf("run_tikv_%s_%d.sh", i.GetHost(), i.GetPort()))
	if err := cfg.ConfigToFile(fp); err != nil {
		return err
	}
	dst := filepath.Join(paths.Deploy, "scripts", "run_tikv.sh")

	if err := e.Transfer(ctx, fp, dst, false, 0, false); err != nil {
		return err
	}

	_, _, err := e.Execute(ctx, "chmod +x "+dst, false)
	if err != nil {
		return err
	}

	globalConfig := topo.ServerConfigs.TiKV
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

	return checkConfig(ctx, e, i.ComponentName(), i.ComponentSource(), clusterVersion, i.OS(), i.Arch(), i.ComponentName()+".toml", paths)
}

// setTLSConfig set TLS Config to support enable/disable TLS
func (i *TiKVInstance) setTLSConfig(ctx context.Context, enableTLS bool, configs map[string]any, paths meta.DirPaths) (map[string]any, error) {
	if enableTLS {
		if configs == nil {
			configs = make(map[string]any)
		}
		configs["security.ca-path"] = fmt.Sprintf(
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
			"security.ca-path",
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
func (i *TiKVInstance) ScaleConfig(
	ctx context.Context,
	e ctxt.Executor,
	topo Topology,
	clusterName,
	clusterVersion,
	deployUser string,
	paths meta.DirPaths,
) error {
	s := i.topo
	defer func() {
		i.topo = s
	}()
	i.topo = mustBeClusterTopo(topo)
	return i.InitConfig(ctx, e, clusterName, clusterVersion, deployUser, paths)
}

var _ RollingUpdateInstance = &TiKVInstance{}

// PreRestart implements RollingUpdateInstance interface.
func (i *TiKVInstance) PreRestart(ctx context.Context, topo Topology, apiTimeoutSeconds int, tlsCfg *tls.Config) error {
	timeoutOpt := &utils.RetryOption{
		Timeout: time.Second * time.Duration(apiTimeoutSeconds),
		Delay:   time.Second * 2,
	}

	tidbTopo, ok := topo.(*Specification)
	if !ok {
		panic("should be type of tidb topology")
	}

	if len(tidbTopo.TiKVServers) <= 1 {
		return nil
	}

	pdClient := api.NewPDClient(ctx, tidbTopo.GetPDListWithManageHost(), 5*time.Second, tlsCfg)

	// Make sure there's leader of PD.
	// Although we evict pd leader when restart pd,
	// But when there's only one PD instance the pd might not serve request right away after restart.
	err := pdClient.WaitLeader(timeoutOpt)
	if err != nil {
		return err
	}

	// Get and record the leader count before evict leader.
	leaderCount, err := genLeaderCounter(tidbTopo, tlsCfg)(i.ID())
	if err != nil {
		return perrs.Annotatef(err, "failed to get leader count %s", i.GetHost())
	}
	i.leaderCountBeforeRestart = leaderCount

	if err := pdClient.EvictStoreLeader(addr(i.InstanceSpec.(*TiKVSpec)), timeoutOpt, genLeaderCounter(tidbTopo, tlsCfg)); err != nil {
		if !utils.IsTimeoutOrMaxRetry(err) {
			return perrs.Annotatef(err, "failed to evict store leader %s", i.GetHost())
		}
		ctx.Value(logprinter.ContextKeyLogger).(*logprinter.Logger).
			Warnf("Ignore evicting store leader from %s, %v", i.ID(), err)
	}
	return nil
}

// PostRestart implements RollingUpdateInstance interface.
func (i *TiKVInstance) PostRestart(ctx context.Context, topo Topology, tlsCfg *tls.Config) error {
	tidbTopo, ok := topo.(*Specification)
	if !ok {
		panic("should be type of tidb topology")
	}

	if len(tidbTopo.TiKVServers) <= 1 {
		return nil
	}

	pdClient := api.NewPDClient(ctx, tidbTopo.GetPDListWithManageHost(), 5*time.Second, tlsCfg)

	// remove store leader evict scheduler after restart
	if err := pdClient.RemoveStoreEvict(addr(i.InstanceSpec.(*TiKVSpec))); err != nil {
		return perrs.Annotatef(err, "failed to remove evict store scheduler for %s", i.GetHost())
	}

	if i.leaderCountBeforeRestart > 0 {
		if err := pdClient.RecoverStoreLeader(addr(i.InstanceSpec.(*TiKVSpec)), i.leaderCountBeforeRestart, nil, genLeaderCounter(tidbTopo, tlsCfg)); err != nil {
			if !utils.IsTimeoutOrMaxRetry(err) {
				return perrs.Annotatef(err, "failed to recover store leader %s", i.GetHost())
			}
			ctx.Value(logprinter.ContextKeyLogger).(*logprinter.Logger).
				Warnf("Ignore recovering store leader from %s, %v", i.ID(), err)
		}
	}

	return nil
}

func addr(spec *TiKVSpec) string {
	if spec.AdvertiseAddr != "" {
		return spec.AdvertiseAddr
	}

	if spec.Port == 0 || spec.Port == 80 {
		panic(fmt.Sprintf("invalid TiKV port %d", spec.Port))
	}
	return utils.JoinHostPort(spec.Host, spec.Port)
}

func genLeaderCounter(topo *Specification, tlsCfg *tls.Config) func(string) (int, error) {
	return func(id string) (int, error) {
		statusAddress := ""
		foundIds := []string{}
		for _, kv := range topo.TiKVServers {
			kvid := utils.JoinHostPort(kv.Host, kv.Port)
			if id == kvid {
				statusAddress = utils.JoinHostPort(kv.GetManageHost(), kv.StatusPort)
				break
			}
			foundIds = append(foundIds, kvid)
		}
		if statusAddress == "" {
			return 0, fmt.Errorf("TiKV instance with ID %s not found, found %s", id, strings.Join(foundIds, ","))
		}

		transport := makeTransport(tlsCfg)

		mfChan := make(chan *dto.MetricFamily, 1024)
		go func() {
			addr := fmt.Sprintf("http://%s/metrics", statusAddress)
			// XXX: https://github.com/tikv/tikv/issues/5340
			//		Some TiKV versions don't handle https correctly
			//      So we check if it's in that case first
			if tlsCfg != nil && checkHTTPS(fmt.Sprintf("https://%s/metrics", statusAddress), tlsCfg) == nil {
				addr = fmt.Sprintf("https://%s/metrics", statusAddress)
			}

			if err := prom2json.FetchMetricFamilies(addr, mfChan, transport); err != nil {
				zap.L().Error("failed counting leader",
					zap.String("host", id),
					zap.String("status addr", addr),
					zap.Error(err),
				)
			}
		}()

		fms := []*prom2json.Family{}
		for mf := range mfChan {
			fm := prom2json.NewFamily(mf)
			fms = append(fms, fm)
		}
		for _, fm := range fms {
			if fm.Name != metricNameRegionCount {
				continue
			}
			for _, m := range fm.Metrics {
				if m, ok := m.(prom2json.Metric); ok && m.Labels["type"] == labelNameLeaderCount {
					return strconv.Atoi(m.Value)
				}
			}
		}

		return 0, perrs.Errorf("metric %s{type=\"%s\"} not found", metricNameRegionCount, labelNameLeaderCount)
	}
}

func makeTransport(tlsCfg *tls.Config) *http.Transport {
	// Start with the DefaultTransport for sane defaults.
	transport := http.DefaultTransport.(*http.Transport).Clone()
	// Conservatively disable HTTP keep-alives as this program will only
	// ever need a single HTTP request.
	transport.DisableKeepAlives = true
	// Timeout early if the server doesn't even return the headers.
	transport.ResponseHeaderTimeout = time.Minute
	// We should clone a tlsCfg because we use it across goroutine
	if tlsCfg != nil {
		transport.TLSClientConfig = tlsCfg.Clone()
	}

	// prefer to use the inner http proxy
	httpProxy := os.Getenv("TIUP_INNER_HTTP_PROXY")
	if len(httpProxy) == 0 {
		httpProxy = os.Getenv("HTTP_PROXY")
	}
	if len(httpProxy) > 0 {
		if proxyURL, err := url.Parse(httpProxy); err == nil {
			transport.Proxy = http.ProxyURL(proxyURL)
		}
	}
	return transport
}

// Check if the url works with tlsCfg
func checkHTTPS(url string, tlsCfg *tls.Config) error {
	transport := makeTransport(tlsCfg)

	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return perrs.Annotatef(err, "creating GET request for URL %q failed", url)
	}

	client := http.Client{Transport: transport}
	resp, err := client.Do(req)
	if err != nil {
		return perrs.Annotatef(err, "executing GET request for URL %q failed", url)
	}
	resp.Body.Close()
	return nil
}
