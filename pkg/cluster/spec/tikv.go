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
	"crypto/tls"
	"fmt"
	"io/ioutil"
	"net/http"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/pingcap/errors"
	"github.com/pingcap/kvproto/pkg/metapb"
	"github.com/pingcap/tiup/pkg/cluster/api"
	"github.com/pingcap/tiup/pkg/cluster/executor"
	"github.com/pingcap/tiup/pkg/cluster/template/scripts"
	"github.com/pingcap/tiup/pkg/logger/log"
	"github.com/pingcap/tiup/pkg/meta"
	"github.com/pingcap/tiup/pkg/utils"
	dto "github.com/prometheus/client_model/go"
	"github.com/prometheus/prom2json"
	pdserverapi "github.com/tikv/pd/server/api"
)

const (
	metricNameRegionCount = "tikv_raftstore_region_count"
	labelNameLeaderCount  = "leader"
)

// TiKVSpec represents the TiKV topology specification in topology.yaml
type TiKVSpec struct {
	Host            string                 `yaml:"host"`
	ListenHost      string                 `yaml:"listen_host,omitempty"`
	SSHPort         int                    `yaml:"ssh_port,omitempty" validate:"ssh_port:editable"`
	Imported        bool                   `yaml:"imported,omitempty"`
	Port            int                    `yaml:"port" default:"20160"`
	StatusPort      int                    `yaml:"status_port" default:"20180"`
	DeployDir       string                 `yaml:"deploy_dir,omitempty"`
	DataDir         string                 `yaml:"data_dir,omitempty"`
	LogDir          string                 `yaml:"log_dir,omitempty"`
	Offline         bool                   `yaml:"offline,omitempty"`
	NumaNode        string                 `yaml:"numa_node,omitempty" validate:"numa_node:editable"`
	Config          map[string]interface{} `yaml:"config,omitempty" validate:"config:ignore"`
	ResourceControl meta.ResourceControl   `yaml:"resource_control,omitempty" validate:"resource_control:editable"`
	Arch            string                 `yaml:"arch,omitempty"`
	OS              string                 `yaml:"os,omitempty"`
}

// checkStoreStatus checks the store status in current cluster
func checkStoreStatus(storeAddr string, tlsCfg *tls.Config, pdList ...string) string {
	if len(pdList) < 1 {
		return "N/A"
	}
	pdapi := api.NewPDClient(pdList, statusQueryTimeout, tlsCfg)
	stores, err := pdapi.GetStores()
	if err != nil {
		return "Down"
	}

	// only get status of the latest store, it is the store with largest ID number
	// older stores might be legacy ones that already offlined
	var latestStore *pdserverapi.StoreInfo

	for _, store := range stores.Stores {
		if storeAddr == store.Store.Address {
			if latestStore == nil {
				latestStore = store
				continue
			}

			// If the PD leader has been switched multiple times, the store IDs
			// may be not monitonically assigned. To workaround this, we iterate
			// over the whole store list to see if any of the store's state is
			// not marked as "tombstone", then use that as the result.
			// See: https://github.com/tikv/pd/issues/3303
			//
			// It's logically not necessary to find the store with largest ID
			// number anymore in this process, but we're keeping the behavior
			// as the reasonable approach would still be using the state from
			// latest store, and this is only a workaround.
			if store.Store.State != metapb.StoreState_Tombstone {
				return store.Store.StateName
			}

			if store.Store.Id > latestStore.Store.Id {
				latestStore = store
			}
		}
	}
	if latestStore != nil {
		return latestStore.Store.StateName
	}
	return "N/A"
}

// Status queries current status of the instance
func (s TiKVSpec) Status(tlsCfg *tls.Config, pdList ...string) string {
	storeAddr := fmt.Sprintf("%s:%d", s.Host, s.Port)
	state := checkStoreStatus(storeAddr, tlsCfg, pdList...)
	if s.Offline && strings.ToLower(state) == "offline" {
		state = "Pending Offline" // avoid misleading
	}
	return state
}

// Role returns the component role of the instance
func (s TiKVSpec) Role() string {
	return ComponentTiKV
}

// SSH returns the host and SSH port of the instance
func (s TiKVSpec) SSH() (string, int) {
	return s.Host, s.SSHPort
}

// GetMainPort returns the main port of the instance
func (s TiKVSpec) GetMainPort() int {
	return s.Port
}

// IsImported returns if the node is imported from TiDB-Ansible
func (s TiKVSpec) IsImported() bool {
	return s.Imported
}

// Labels returns the labels of TiKV
func (s TiKVSpec) Labels() (map[string]string, error) {
	lbs := make(map[string]string)

	if serverLabels := GetValueFromPath(s.Config, "server.labels"); serverLabels != nil {
		m := map[interface{}]interface{}{}
		if sm, ok := serverLabels.(map[string]interface{}); ok {
			for k, v := range sm {
				m[k] = v
			}
		} else if im, ok := serverLabels.(map[interface{}]interface{}); ok {
			m = im
		}
		for k, v := range m {
			key, ok := k.(string)
			if !ok {
				return nil, errors.Errorf("TiKV label name %v is not a string, check the instance: %s:%d", k, s.Host, s.GetMainPort())
			}
			value, ok := v.(string)
			if !ok {
				return nil, errors.Errorf("TiKV label value %v is not a string, check the instance: %s:%d", v, s.Host, s.GetMainPort())
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

// Instances implements Component interface.
func (c *TiKVComponent) Instances() []Instance {
	ins := make([]Instance, 0, len(c.Topology.TiKVServers))
	for _, s := range c.Topology.TiKVServers {
		s := s
		ins = append(ins, &TiKVInstance{BaseInstance{
			InstanceSpec: s,
			Name:         c.Name(),
			Host:         s.Host,
			ListenHost:   s.ListenHost,
			Port:         s.Port,
			SSHP:         s.SSHPort,

			Ports: []int{
				s.Port,
				s.StatusPort,
			},
			Dirs: []string{
				s.DeployDir,
				s.DataDir,
			},
			StatusFn: s.Status,
		}, c.Topology})
	}
	return ins
}

// TiKVInstance represent the TiDB instance
type TiKVInstance struct {
	BaseInstance
	topo Topology
}

// InitConfig implement Instance interface
func (i *TiKVInstance) InitConfig(
	e executor.Executor,
	clusterName,
	clusterVersion,
	deployUser string,
	paths meta.DirPaths,
) error {
	topo := i.topo.(*Specification)
	if err := i.BaseInstance.InitConfig(e, topo.GlobalOptions, deployUser, paths); err != nil {
		return err
	}

	enableTLS := topo.GlobalOptions.TLSEnabled
	spec := i.InstanceSpec.(TiKVSpec)
	cfg := scripts.NewTiKVScript(
		clusterVersion,
		i.GetHost(),
		paths.Deploy,
		paths.Data[0],
		paths.Log,
	).WithPort(spec.Port).
		WithNumaNode(spec.NumaNode).
		WithStatusPort(spec.StatusPort).
		AppendEndpoints(topo.Endpoints(deployUser)...).
		WithListenHost(i.GetListenHost())
	fp := filepath.Join(paths.Cache, fmt.Sprintf("run_tikv_%s_%d.sh", i.GetHost(), i.GetPort()))
	if err := cfg.ConfigToFile(fp); err != nil {
		return err
	}
	dst := filepath.Join(paths.Deploy, "scripts", "run_tikv.sh")

	if err := e.Transfer(fp, dst, false); err != nil {
		return err
	}

	if _, _, err := e.Execute("chmod +x "+dst, false); err != nil {
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
		importConfig, err := ioutil.ReadFile(configPath)
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
		spec.Config["security.ca-path"] = fmt.Sprintf(
			"%s/tls/%s",
			paths.Deploy,
			TLSCACert,
		)
		spec.Config["security.cert-path"] = fmt.Sprintf(
			"%s/tls/%s.crt",
			paths.Deploy,
			i.Role())
		spec.Config["security.key-path"] = fmt.Sprintf(
			"%s/tls/%s.pem",
			paths.Deploy,
			i.Role())
	}

	if err := i.MergeServerConfig(e, globalConfig, spec.Config, paths); err != nil {
		return err
	}

	return checkConfig(e, i.ComponentName(), clusterVersion, i.OS(), i.Arch(), i.ComponentName()+".toml", paths, nil)
}

// ScaleConfig deploy temporary config on scaling
func (i *TiKVInstance) ScaleConfig(
	e executor.Executor,
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
	return i.InitConfig(e, clusterName, clusterVersion, deployUser, paths)
}

var _ RollingUpdateInstance = &TiKVInstance{}

// PreRestart implements RollingUpdateInstance interface.
func (i *TiKVInstance) PreRestart(topo Topology, apiTimeoutSeconds int, tlsCfg *tls.Config) error {
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

	pdClient := api.NewPDClient(tidbTopo.GetPDList(), 5*time.Second, tlsCfg)

	// Make sure there's leader of PD.
	// Although we evict pd leader when restart pd,
	// But when there's only one PD instance the pd might not serve request right away after restart.
	err := pdClient.WaitLeader(timeoutOpt)
	if err != nil {
		return err
	}

	if err := pdClient.EvictStoreLeader(addr(i), timeoutOpt, genLeaderCounter(tidbTopo, tlsCfg)); err != nil {
		if utils.IsTimeoutOrMaxRetry(err) {
			log.Warnf("Ignore evicting store leader from %s, %v", i.ID(), err)
		} else {
			return errors.Annotatef(err, "failed to evict store leader %s", i.GetHost())
		}
	}
	return nil
}

// PostRestart implements RollingUpdateInstance interface.
func (i *TiKVInstance) PostRestart(topo Topology, tlsCfg *tls.Config) error {
	tidbTopo, ok := topo.(*Specification)
	if !ok {
		panic("should be type of tidb topology")
	}

	if len(tidbTopo.TiKVServers) <= 1 {
		return nil
	}

	pdClient := api.NewPDClient(tidbTopo.GetPDList(), 5*time.Second, tlsCfg)

	// remove store leader evict scheduler after restart
	if err := pdClient.RemoveStoreEvict(addr(i)); err != nil {
		return errors.Annotatef(err, "failed to remove evict store scheduler for %s", i.GetHost())
	}

	return nil
}

func addr(ins Instance) string {
	if ins.GetPort() == 0 || ins.GetPort() == 80 {
		panic(ins)
	}
	return ins.GetHost() + ":" + strconv.Itoa(ins.GetPort())
}

func genLeaderCounter(topo *Specification, tlsCfg *tls.Config) func(string) (int, error) {
	return func(id string) (int, error) {
		statusAddress := ""
		foundIds := []string{}
		for _, kv := range topo.TiKVServers {
			kvid := fmt.Sprintf("%s:%d", kv.Host, kv.Port)
			if id == kvid {
				statusAddress = fmt.Sprintf("%s:%d", kv.Host, kv.StatusPort)
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
				log.Errorf("failed counting leader on %s (status addr %s), %v", id, addr, err)
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

		return 0, errors.Errorf("metric %s{type=\"%s\"} not found", metricNameRegionCount, labelNameLeaderCount)
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
	return transport
}

// Check if the url works with tlsCfg
func checkHTTPS(url string, tlsCfg *tls.Config) error {
	transport := makeTransport(tlsCfg)

	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return errors.Annotatef(err, "creating GET request for URL %q failed", url)
	}

	client := http.Client{Transport: transport}
	resp, err := client.Do(req)
	if err != nil {
		return errors.Annotatef(err, "executing GET request for URL %q failed", url)
	}
	resp.Body.Close()
	return nil
}
