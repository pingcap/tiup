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
	"context"
	"fmt"

	"github.com/pingcap/errors"
	"github.com/pingcap/tiup/pkg/cluster/ctxt"
	"github.com/pingcap/tiup/pkg/cluster/module"
)

// SystemCtl run systemctl command on host
type SystemCtl struct {
	host         string
	unit         string
	action       string
	daemonReload bool
	checkactive  bool
}

// Execute implements the Task interface
func (c *SystemCtl) Execute(ctx context.Context) error {
	e, ok := ctxt.GetInner(ctx).GetExecutor(c.host)
	if !ok {
		return ErrNoExecutor
	}

	cfg := module.SystemdModuleConfig{
		Unit:         c.unit,
		Action:       c.action,
		ReloadDaemon: c.daemonReload,
		CheckActive:  c.checkactive,
	}
	systemd := module.NewSystemdModule(cfg)
	stdout, stderr, err := systemd.Execute(ctx, e)
	ctxt.GetInner(ctx).SetOutputs(c.host, stdout, stderr)
	if err != nil {
		return errors.Annotatef(err, "stdout: %s, stderr:%s", string(stdout), string(stderr))
	}

	return nil
}

// Rollback implements the Task interface
func (c *SystemCtl) Rollback(ctx context.Context) error {
	return ErrUnsupportedRollback
}

// String implements the fmt.Stringer interface
func (c *SystemCtl) String() string {
	return fmt.Sprintf("SystemCtl: host=%s action=%s %s", c.host, c.action, c.unit)
}
