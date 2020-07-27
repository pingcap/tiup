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
	"strconv"
	"strings"
	"time"

	"github.com/pingcap/errors"
	pdserverapi "github.com/pingcap/pd/v4/server/api"
	"github.com/pingcap/tiup/pkg/cluster/api"
	"github.com/pingcap/tiup/pkg/cluster/clusterutil"
	"github.com/pingcap/tiup/pkg/cluster/executor"
	"github.com/pingcap/tiup/pkg/cluster/template/scripts"
	"github.com/pingcap/tiup/pkg/logger/log"
	"github.com/pingcap/tiup/pkg/meta"
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
func checkStoreStatus(storeAddr string, pdList ...string) string {
	if len(pdList) < 1 {
		return "N/A"
	}
	pdapi := api.NewPDClient(pdList, statusQueryTimeout, nil)
	stores, err := pdapi.GetStores()
	if err != nil {
		return "Down"
	}

	// only get status of the latest store, it is the store with lagest ID number
	// older stores might be legacy ones that already offlined
	var latestStore *pdserverapi.StoreInfo
	for _, store := range stores.Stores {
		if storeAddr == store.Store.Address {
			if latestStore == nil {
				latestStore = store
				continue
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
func (s TiKVSpec) Status(pdList ...string) string {
	storeAddr := fmt.Sprintf("%s:%d", s.Host, s.Port)
	state := checkStoreStatus(storeAddr, pdList...)
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

// TiKVComponent represents TiKV component.
type TiKVComponent struct {
	*Specification
}

// Name implements Component interface.
func (c *TiKVComponent) Name() string {
	return ComponentTiKV
}

// Instances implements Component interface.
func (c *TiKVComponent) Instances() []Instance {
	ins := make([]Instance, 0, len(c.TiKVServers))
	for _, s := range c.TiKVServers {
		s := s
		ins = append(ins, &TiKVInstance{instance{
			InstanceSpec: s,
			name:         c.Name(),
			host:         s.Host,
			listenHost:   s.ListenHost,
			port:         s.Port,
			sshp:         s.SSHPort,
			topo:         c.Specification,

			usedPorts: []int{
				s.Port,
				s.StatusPort,
			},
			usedDirs: []string{
				s.DeployDir,
				s.DataDir,
			},
			statusFn: s.Status,
		}})
	}
	return ins
}

// TiKVInstance represent the TiDB instance
type TiKVInstance struct {
	instance
}

// InitConfig implement Instance interface
func (i *TiKVInstance) InitConfig(e executor.Executor, clusterName, clusterVersion, deployUser string, paths meta.DirPaths) error {
	if err := i.instance.InitConfig(e, clusterName, clusterVersion, deployUser, paths); err != nil {
		return err
	}

	spec := i.InstanceSpec.(TiKVSpec)
	cfg := scripts.NewTiKVScript(
		i.GetHost(),
		paths.Deploy,
		paths.Data[0],
		paths.Log,
	).WithPort(spec.Port).
		WithNumaNode(spec.NumaNode).
		WithStatusPort(spec.StatusPort).
		AppendEndpoints(i.instance.topo.Endpoints(deployUser)...).
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

	globalConfig := i.instance.topo.ServerConfigs.TiKV
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

	if err := i.mergeServerConfig(e, globalConfig, spec.Config, paths); err != nil {
		return err
	}

	return checkConfig(e, i.ComponentName(), clusterVersion, i.OS(), i.Arch(), i.ComponentName()+".toml", paths)
}

// ScaleConfig deploy temporary config on scaling
func (i *TiKVInstance) ScaleConfig(e executor.Executor, topo Topology, clusterName, clusterVersion, deployUser string, paths meta.DirPaths) error {
	s := i.instance.topo
	defer func() {
		i.instance.topo = s
	}()
	i.instance.topo = mustBeClusterTopo(topo)
	return i.InitConfig(e, clusterName, clusterVersion, deployUser, paths)
}

var _ RollingUpdateInstance = &TiKVInstance{}

// PreRestart implements RollingUpdateInstance interface.
func (i *TiKVInstance) PreRestart(topo Topology, apiTimeoutSeconds int) error {
	timeoutOpt := &clusterutil.RetryOption{
		Timeout: time.Second * time.Duration(apiTimeoutSeconds),
		Delay:   time.Second * 2,
	}

	tidbTopo, ok := topo.(*Specification)
	if !ok {
		panic("should be type of tidb topology")
	}

	pdClient := api.NewPDClient(tidbTopo.GetPDList(), 5*time.Second, nil)

	// Make sure there's leader of PD.
	// Although we evict pd leader when restart pd,
	// But when there's only one PD instance the pd might not serve request right away after restart.
	err := pdClient.WaitLeader(timeoutOpt)
	if err != nil {
		return errors.AddStack(err)
	}

	if err := pdClient.EvictStoreLeader(addr(i), timeoutOpt); err != nil {
		if clusterutil.IsTimeoutOrMaxRetry(err) {
			log.Warnf("Ignore evicting store leader from %s, %v", i.ID(), err)
		} else {
			return errors.Annotatef(err, "failed to evict store leader %s", i.GetHost())
		}
	}
	return nil
}

// PostRestart implements RollingUpdateInstance interface.
func (i *TiKVInstance) PostRestart(topo Topology) error {
	tidbTopo, ok := topo.(*Specification)
	if !ok {
		panic("should be type of tidb topology")
	}

	pdClient := api.NewPDClient(tidbTopo.GetPDList(), 5*time.Second, nil)

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
