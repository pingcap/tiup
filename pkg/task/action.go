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
	"io"

	"github.com/pingcap-incubator/tiops/pkg/meta"
	operator "github.com/pingcap-incubator/tiops/pkg/operation"
	"github.com/pingcap/errors"
)

// ClusterOperate represents the cluster operation task.
type ClusterOperate struct {
	spec *meta.Specification
	// valid op:
	// start
	// stop
	// restart
	// destroy
	op     string
	role   string
	nodeID string
	w      io.Writer
}

// Execute implements the Task interface
func (c *ClusterOperate) Execute(ctx *Context) error {
	switch c.op {
	case "start":
		err := operator.Start(ctx, c.w, c.spec, c.role, c.nodeID)
		if err != nil {
			return errors.Annotate(err, "failed to start")
		}
		operator.PrintClusterStatus(ctx, c.w, c.spec)
	case "stop":
		err := operator.Stop(ctx, c.w, c.spec, c.role, c.nodeID)
		if err != nil {
			return errors.Annotate(err, "failed to stop")
		}
		operator.PrintClusterStatus(ctx, c.w, c.spec)
	case "restart":
		err := operator.Restart(ctx, c.w, c.spec, c.role, c.nodeID)
		if err != nil {
			return errors.Annotate(err, "failed to restart")
		}
		operator.PrintClusterStatus(ctx, c.w, c.spec)
	case "destroy":
		fallthrough
	default:
		return errors.Errorf("nonsupport %s", c.op)
	}

	return nil
}

// Rollback implements the Task interface
func (c *ClusterOperate) Rollback(ctx *Context) error {
	return ErrUnsupportRollback
}
