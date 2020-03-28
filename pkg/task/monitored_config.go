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

	"github.com/pingcap-incubator/tiops/pkg/meta"
)

// MonitoredConfig is used to generate the monitor node configuration
type MonitoredConfig struct {
	name       string
	options    meta.MonitoredOptions
	deployUser string
	deployDir  string
}

// Execute implements the Task interface
func (m *MonitoredConfig) Execute(ctx *Context) error {
	ctx.Warnf("MonitoredConfig not implement")
	return nil
}

// Rollback implements the Task interface
func (m *MonitoredConfig) Rollback(ctx *Context) error {
	return ErrUnsupportRollback
}

// String implements the fmt.Stringer interface
func (m *MonitoredConfig) String() string {
	return fmt.Sprintf("MonitoredConfig: cluster=%s, user=%s, dir=%s, node_exporter_port=%d, blackbox_exporter_port=%d",
		m.name, m.deployUser, m.deployDir, m.options.NodeExporterPort, m.options.BlackboxExporterPort)
}
