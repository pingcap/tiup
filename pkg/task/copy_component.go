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
	"github.com/pingcap-incubator/tiup/pkg/repository"
	"github.com/pingcap/errors"
)

const cacheTarballDir = "/tmp/tiops/tarball"
const cacheConfigDir = "/tmp/tiops/config"

// CopyComponent is used to copy all files related the specific version a component
// to the target directory of path
type CopyComponent struct {
	topology  meta.TopologySpecification
	component string
	version   repository.Version
	host      string
	dstDir    string
}

// Execute implements the Task interface
func (c *CopyComponent) Execute(ctx *Context) error {

	// Copy to remote server
	exec, found := ctx.GetExecutor(c.host)
	if !found {
		return ErrNoExecutor
	}

	resName := fmt.Sprintf("%s-%s", c.component, c.version)
	fileName := fmt.Sprintf("%s-linux-amd64.tar.gz", resName)
	srcPath := filepath.Join(cacheTarballDir, fileName)
	dstDir := filepath.Join(c.dstDir, "bin")
	dstPath := filepath.Join(dstDir, fileName)

	err := exec.Transfer(srcPath, dstPath)
	if err != nil {
		return errors.Trace(err)
	}

	cmd := fmt.Sprintf(`tar -xzf %s -C %s && rm %s`, dstPath, dstDir, dstPath)

	stdout, stderr, err := exec.Execute(cmd, false)
	if err != nil {
		return errors.Trace(err)
	}

	fmt.Println("Decompress tarball stdout: ", string(stdout))
	fmt.Println("Decompress tarball stderr: ", string(stderr))

	return c.TransferConfig(exec)
}

// TransferConfig transfer config file to server
func (c *CopyComponent) TransferConfig(exec executor.TiOpsExecutor) error {
	/*
		sysCfg := filepath.Join(cacheConfigDir, c.component+".service")
		if err := system.NewSystemConfig(c.component, "tidb", c.dstDir).ConfigToFile(sysCfg); err != nil {
			return err
		}
		if err := exec.Transfer(sysCfg, filepath.Join("/etc/systemd/system", c.component+"./service")); err != nil {
			return err
		}
	*/
	if err := os.MkdirAll(cacheConfigDir, 0755); err != nil {
		return err
	}
	switch c.component {
	case "pd":
		return c.transferPDConfig(exec)
	case "tidb":
		return c.transferTiDBConfig(exec)
	case "tikv":
		return c.transferTiKVConfig(exec)
	default:
		return nil //fmt.Errorf("unknow component: %s", c.component)
	}
}

func (c *CopyComponent) endpoints() []*scripts.PDScript {
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

func (c *CopyComponent) transferPDConfig(exec executor.TiOpsExecutor) error {
	name := "pd-" + c.host
	cfg := scripts.NewPDScript(name, c.host, c.dstDir, filepath.Join(c.dstDir, "data")).AppendEndpoints(c.endpoints()...)
	fp := filepath.Join(cacheConfigDir, fmt.Sprintf("run_pd_%s.sh", c.host))
	if err := cfg.ConfigToFile(fp); err != nil {
		return err
	}
	return exec.Transfer(fp, filepath.Join(c.dstDir, "scripts", "run_pd.sh"))
}

func (c *CopyComponent) transferTiDBConfig(exec executor.TiOpsExecutor) error {
	cfg := scripts.NewTiDBScript(c.host, c.dstDir).AppendEndpoints(c.endpoints()...)
	fp := filepath.Join(cacheConfigDir, fmt.Sprintf("run_tidb_%s.sh", c.host))
	if err := cfg.ConfigToFile(fp); err != nil {
		return err
	}
	return exec.Transfer(fp, filepath.Join(c.dstDir, "scripts", "run_tidb.sh"))
}

func (c *CopyComponent) transferTiKVConfig(exec executor.TiOpsExecutor) error {
	cfg := scripts.NewTiKVScript(c.host, c.dstDir, filepath.Join(c.dstDir, "data")).AppendEndpoints(c.endpoints()...)
	fp := filepath.Join(cacheConfigDir, fmt.Sprintf("run_tikv_%s.sh", c.host))
	if err := cfg.ConfigToFile(fp); err != nil {
		return err
	}
	return exec.Transfer(fp, filepath.Join(c.dstDir, "scripts", "run_tikv.sh"))
}

// Rollback implements the Task interface
func (c *CopyComponent) Rollback(ctx *Context) error {
	return ErrUnsupportRollback
}
