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
	"os"

	"github.com/pingcap-incubator/tiops/pkg/meta"
)

// ScaleConfig is used to copy all configurations to the target directory of path
type ScaleConfig struct {
	name       string
	instance   meta.Instance
	base       *meta.TopologySpecification
	deployUser string
	deployDir  string
}

// Execute implements the Task interface
func (c *ScaleConfig) Execute(ctx *Context) error {
	// Copy to remote server
	exec, found := ctx.GetExecutor(c.instance.GetHost())
	if !found {
		return ErrNoExecutor
	}

	cacheConfigDir := meta.ClusterPath(c.name, "config")
	if err := os.MkdirAll(cacheConfigDir, 0755); err != nil {
		return err
	}

	return c.instance.ScaleConfig(exec, c.base, c.deployDir, cacheConfigDir, c.deployDir)
}

// Rollback implements the Task interface
func (c *ScaleConfig) Rollback(ctx *Context) error {
	return ErrUnsupportRollback
}
