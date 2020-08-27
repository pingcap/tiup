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

	"github.com/pingcap/errors"
	"github.com/pingcap/tiup/pkg/cluster"
	"github.com/pingcap/tiup/pkg/cluster/executor"
	"github.com/pingcap/tiup/pkg/cluster/spec"
	cspec "github.com/pingcap/tiup/pkg/cluster/spec"
	"github.com/pingcap/tiup/pkg/cluster/task"
	"github.com/pingcap/tiup/pkg/cluster/template/config"
	"github.com/pingcap/tiup/pkg/cluster/template/scripts"
	"github.com/pingcap/tiup/pkg/meta"
)

// AlertManagerComponent represents Alertmanager component.
type AlertManagerComponent struct{ *Topology }

// Name implements Component interface.
func (c *AlertManagerComponent) Name() string {
	return ComponentAlertManager
}

// Role implements Component interface.
func (c *AlertManagerComponent) Role() string {
	return cspec.RoleMonitor
}

// Instances implements Component interface.
func (c *AlertManagerComponent) Instances() []Instance {
	ins := make([]Instance, 0, len(c.Alertmanager))
	for _, s := range c.Alertmanager {
		ins = append(ins, &AlertManagerInstance{
			BaseInstance: cspec.BaseInstance{
				InstanceSpec: s,
				Name:         c.Name(),
				Host:         s.Host,
				Port:         s.WebPort,
				SSHP:         s.SSHPort,

				Ports: []int{
					s.WebPort,
					s.ClusterPort,
				},
				Dirs: []string{
					s.DeployDir,
					s.DataDir,
				},
				StatusFn: func(_ ...string) string {
					return "-"
				},
			},
			topo: c.Topology,
		})
	}
	return ins
}

// AlertManagerInstance represent the alert manager instance
type AlertManagerInstance struct {
	cspec.BaseInstance
	topo *Topology
}

var _ cluster.DeployerInstance = &AlertManagerInstance{}

// Deploy implements DeployerInstance interface.
func (i *AlertManagerInstance) Deploy(t *task.Builder, srcPath string, deployDir string, version string, clusterName string, clusterVersion string) {
	t.CopyComponent(
		i.ComponentName(),
		i.OS(),
		i.Arch(),
		version,
		srcPath,
		i.GetHost(),
		deployDir,
	).Func("CopyConfig", func(ctx *task.Context) error {
		tempDir, err := ioutil.TempDir("", "tiup-*")
		if err != nil {
			return errors.AddStack(err)
		}
		// transfer config
		e := ctx.Get(i.GetHost())
		fp := filepath.Join(tempDir, fmt.Sprintf("alertmanager_%s.yml", i.GetHost()))
		if err := config.NewAlertManagerConfig().ConfigToFile(fp); err != nil {
			return err
		}
		dst := filepath.Join(deployDir, "conf", "alertmanager.yml")
		err = e.Transfer(fp, dst, false)
		if err != nil {
			return errors.Annotatef(err, "failed to transfer %s to %s@%s", fp, i.GetHost(), dst)
		}
		return nil
	})

}

// InitConfig implement Instance interface
func (i *AlertManagerInstance) InitConfig(e executor.Executor, clusterName, clusterVersion, deployUser string, paths meta.DirPaths) error {
	if err := i.BaseInstance.InitConfig(e, i.topo.GlobalOptions, deployUser, paths); err != nil {
		return err
	}

	// Transfer start script
	spec := i.InstanceSpec.(AlertManagerSpec)
	cfg := scripts.NewAlertManagerScript(spec.Host, paths.Deploy, paths.Data[0], paths.Log).
		WithWebPort(spec.WebPort).WithClusterPort(spec.ClusterPort).WithNumaNode(spec.NumaNode).
		AppendEndpoints(cspec.AlertManagerEndpoints(i.topo.Alertmanager, deployUser))

	fp := filepath.Join(paths.Cache, fmt.Sprintf("run_alertmanager_%s_%d.sh", i.GetHost(), i.GetPort()))
	if err := cfg.ConfigToFile(fp); err != nil {
		return err
	}

	dst := filepath.Join(paths.Deploy, "scripts", "run_alertmanager.sh")
	if err := e.Transfer(fp, dst, false); err != nil {
		return err
	}
	if _, _, err := e.Execute("chmod +x "+dst, false); err != nil {
		return err
	}

	// If the user specific a local config file, we should overwrite the default one with it
	if spec.ConfigFilePath != "" {
		name := filepath.Base(spec.ConfigFilePath)
		dst := filepath.Join(paths.Deploy, "conf", name)
		if err := i.TransferLocalConfigFile(e, spec.ConfigFilePath, dst); err != nil {
			return errors.Annotate(err, "transfer alertmanager config failed")
		}
	}

	return nil
}

// ScaleConfig deploy temporary config on scaling
func (i *AlertManagerInstance) ScaleConfig(e executor.Executor, topo spec.Topology,
	clusterName string, clusterVersion string, deployUser string, paths meta.DirPaths) error {
	s := i.topo
	defer func() { i.topo = s }()
	i.topo = topo.(*Topology)
	return i.InitConfig(e, clusterName, clusterVersion, deployUser, paths)
}
