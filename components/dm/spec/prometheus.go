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
	"path/filepath"

	"github.com/pingcap/tiup/pkg/cluster/executor"
	"github.com/pingcap/tiup/pkg/cluster/spec"
	"github.com/pingcap/tiup/pkg/cluster/template/config"
	"github.com/pingcap/tiup/pkg/cluster/template/scripts"
	"github.com/pingcap/tiup/pkg/meta"
)

// MonitorComponent represents Monitor component.
type MonitorComponent struct{ *Topology }

// Name implements Component interface.
func (c *MonitorComponent) Name() string {
	return ComponentPrometheus
}

// Instances implements Component interface.
func (c *MonitorComponent) Instances() []Instance {
	ins := make([]Instance, 0, len(c.Monitors))
	for _, s := range c.Monitors {
		ins = append(ins, &MonitorInstance{spec.BaseInstance{
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
			StatusFn: func(_ ...string) string {
				return "-"
			},
		}, c.Topology})
	}
	return ins
}

// MonitorInstance represent the monitor instance
type MonitorInstance struct {
	spec.BaseInstance
	topo *Topology
}

// InitConfig implement Instance interface
func (i *MonitorInstance) InitConfig(e executor.Executor, clusterName, clusterVersion, deployUser string, paths meta.DirPaths) error {
	if err := i.BaseInstance.InitConfig(e, i.topo.GlobalOptions, deployUser, paths); err != nil {
		return err
	}

	// transfer run script
	spec := i.InstanceSpec.(PrometheusSpec)
	cfg := scripts.NewPrometheusScript(
		i.GetHost(),
		paths.Deploy,
		paths.Data[0],
		paths.Log,
	).WithPort(spec.Port).
		WithNumaNode(spec.NumaNode)
	fp := filepath.Join(paths.Cache, fmt.Sprintf("run_prometheus_%s_%d.sh", i.GetHost(), i.GetPort()))
	if err := cfg.ConfigToFile(fp); err != nil {
		return err
	}

	dst := filepath.Join(paths.Deploy, "scripts", "run_prometheus.sh")
	if err := e.Transfer(fp, dst, false); err != nil {
		return err
	}

	if _, _, err := e.Execute("chmod +x "+dst, false); err != nil {
		return err
	}

	// topo := i.topo

	// transfer config
	fp = filepath.Join(paths.Cache, fmt.Sprintf("tikv_%s.yml", i.GetHost()))
	cfig := config.NewPrometheusConfig(clusterName)

	/*
		cfig.AddBlackbox(i.GetHost(), uint64(topo.MonitoredOptions.BlackboxExporterPort))
		uniqueHosts := set.NewStringSet()
		for _, pd := range topo.PDServers {
			uniqueHosts.Insert(pd.Host)
			cfig.AddPD(pd.Host, uint64(pd.ClientPort))
		}
		for _, kv := range topo.TiKVServers {
			uniqueHosts.Insert(kv.Host)
			cfig.AddTiKV(kv.Host, uint64(kv.StatusPort))
		}
		for _, db := range topo.TiDBServers {
			uniqueHosts.Insert(db.Host)
			cfig.AddTiDB(db.Host, uint64(db.StatusPort))
		}
		for _, flash := range topo.TiFlashServers {
			uniqueHosts.Insert(flash.Host)
			cfig.AddTiFlashLearner(flash.Host, uint64(flash.FlashProxyStatusPort))
			cfig.AddTiFlash(flash.Host, uint64(flash.StatusPort))
		}
		for _, pump := range topo.PumpServers {
			uniqueHosts.Insert(pump.Host)
			cfig.AddPump(pump.Host, uint64(pump.Port))
		}
		for _, drainer := range topo.Drainers {
			uniqueHosts.Insert(drainer.Host)
			cfig.AddDrainer(drainer.Host, uint64(drainer.Port))
		}
		for _, cdc := range topo.CDCServers {
			uniqueHosts.Insert(cdc.Host)
			cfig.AddCDC(cdc.Host, uint64(cdc.Port))
		}
		for _, grafana := range topo.Grafana {
			uniqueHosts.Insert(grafana.Host)
			cfig.AddGrafana(grafana.Host, uint64(grafana.Port))
		}
		for _, alertmanager := range topo.Alertmanager {
			uniqueHosts.Insert(alertmanager.Host)
			cfig.AddAlertmanager(alertmanager.Host, uint64(alertmanager.WebPort))
		}
		for host := range uniqueHosts {
			cfig.AddNodeExpoertor(host, uint64(topo.MonitoredOptions.NodeExporterPort))
			cfig.AddBlackboxExporter(host, uint64(topo.MonitoredOptions.BlackboxExporterPort))
			cfig.AddMonitoredServer(host)
		}
	*/

	if err := cfig.ConfigToFile(fp); err != nil {
		return err
	}
	dst = filepath.Join(paths.Deploy, "conf", "prometheus.yml")
	return e.Transfer(fp, dst, false)
}

// ScaleConfig deploy temporary config on scaling
func (i *MonitorInstance) ScaleConfig(e executor.Executor, topo spec.Topology,
	clusterName string, clusterVersion string, deployUser string, paths meta.DirPaths) error {
	s := i.topo
	defer func() { i.topo = s }()
	i.topo = topo.(*Topology)
	return i.InitConfig(e, clusterName, clusterVersion, deployUser, paths)
}
