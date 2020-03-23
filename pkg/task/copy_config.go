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

package task

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/pingcap-incubator/tiops/pkg/executor"
	"github.com/pingcap-incubator/tiops/pkg/meta"
	"github.com/pingcap-incubator/tiops/pkg/template/scripts"
)

// CopyConfig is used to copy all configurations to the target directory of path
type CopyConfig struct {
	name      string
	topology  *meta.TopologySpecification
	component string
	host      string
	dstDir    string
}

// Execute implements the Task interface
func (c *CopyConfig) Execute(ctx *Context) error {
	// Copy to remote server
	exec, found := ctx.GetExecutor(c.host)
	if !found {
		return ErrNoExecutor
	}

	/*
		sysCfg := filepath.Join(cacheConfigDir, c.component+".service")
		if err := system.NewSystemConfig(c.component, "tidb", c.dstDir).ConfigToFile(sysCfg); err != nil {
			return err
		}
		if err := exec.Transfer(sysCfg, filepath.Join("/etc/systemd/system", c.component+"./service")); err != nil {
			return err
		}
	*/
	cacheConfigDir := meta.ClusterPath(c.name, "config")
	if err := os.MkdirAll(cacheConfigDir, 0755); err != nil {
		return err
	}
	switch c.component {
	case "pd":
		return c.transferPDConfig(exec, cacheConfigDir)
	case "tidb":
		return c.transferTiDBConfig(exec, cacheConfigDir)
	case "tikv":
		return c.transferTiKVConfig(exec, cacheConfigDir)
	default:
		return nil //fmt.Errorf("unknow component: %s", c.component)
	}
}

func (c *CopyConfig) endpoints() []*scripts.PDScript {
	ends := []*scripts.PDScript{}
	for _, spec := range c.topology.PDServers {
		ends = append(ends, scripts.NewPDScript(
			"pd"+spec.Host,
			spec.Host,
			spec.DeployDir,
			spec.DataDir,
		))
	}
	return ends
}

func (c *CopyConfig) transferPDConfig(exec executor.TiOpsExecutor, cacheConfigDir string) error {
	name := "pd-" + c.host
	cfg := scripts.NewPDScript(name, c.host, c.dstDir, filepath.Join(c.dstDir, "data")).AppendEndpoints(c.endpoints()...)
	fp := filepath.Join(cacheConfigDir, fmt.Sprintf("run_pd_%s.sh", c.host))
	if err := cfg.ConfigToFile(fp); err != nil {
		return err
	}
	return exec.Transfer(fp, filepath.Join(c.dstDir, "scripts", "run_pd.sh"))
}

func (c *CopyConfig) transferTiDBConfig(exec executor.TiOpsExecutor, cacheConfigDir string) error {
	cfg := scripts.NewTiDBScript(c.host, c.dstDir).AppendEndpoints(c.endpoints()...)
	fp := filepath.Join(cacheConfigDir, fmt.Sprintf("run_tidb_%s.sh", c.host))
	if err := cfg.ConfigToFile(fp); err != nil {
		return err
	}
	return exec.Transfer(fp, filepath.Join(c.dstDir, "scripts", "run_tidb.sh"))
}

func (c *CopyConfig) transferTiKVConfig(exec executor.TiOpsExecutor, cacheConfigDir string) error {
	cfg := scripts.NewTiKVScript(c.host, c.dstDir, filepath.Join(c.dstDir, "data")).AppendEndpoints(c.endpoints()...)
	fp := filepath.Join(cacheConfigDir, fmt.Sprintf("run_tikv_%s.sh", c.host))
	if err := cfg.ConfigToFile(fp); err != nil {
		return err
	}
	return exec.Transfer(fp, filepath.Join(c.dstDir, "scripts", "run_tikv.sh"))
}

// Rollback implements the Task interface
func (c *CopyConfig) Rollback(ctx *Context) error {
	return ErrUnsupportRollback
}
