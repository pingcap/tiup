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

	"github.com/pingcap/errors"
	operator "github.com/pingcap/tiup/pkg/cluster/operation"
	"github.com/pingcap/tiup/pkg/cluster/spec"
)

// ClusterOperate represents the cluster operation task.
type ClusterOperate struct {
	spec    *spec.Specification
	op      operator.Operation
	options operator.Options
}

// Execute implements the Task interface
func (c *ClusterOperate) Execute(ctx *Context) error {
	switch c.op {
	case operator.StartOperation:
		err := operator.Start(ctx, c.spec, c.options)
		if err != nil {
			return errors.Annotate(err, "failed to start")
		}
		operator.PrintClusterStatus(ctx, c.spec)
	case operator.StopOperation:
		err := operator.Stop(ctx, c.spec, c.options)
		if err != nil {
			return errors.Annotate(err, "failed to stop")
		}
		operator.PrintClusterStatus(ctx, c.spec)
	case operator.RestartOperation:
		err := operator.Restart(ctx, c.spec, c.options)
		if err != nil {
			return errors.Annotate(err, "failed to restart")
		}
		operator.PrintClusterStatus(ctx, c.spec)
	case operator.UpgradeOperation:
		err := operator.Upgrade(ctx, c.spec, c.options)
		if err != nil {
			return errors.Annotate(err, "failed to upgrade")
		}
		operator.PrintClusterStatus(ctx, c.spec)
	case operator.DestroyOperation:
		err := operator.Destroy(ctx, c.spec, c.options)
		if err != nil {
			return errors.Annotate(err, "failed to destroy")
		}
	case operator.DestroyTombstoneOperation:
		_, err := operator.DestroyTombstone(ctx, c.spec, false, c.options)
		if err != nil {
			return errors.Annotate(err, "failed to destroy")
		}
	// print nothing
	case operator.ScaleInOperation:
		err := operator.ScaleIn(ctx, c.spec, c.options)
		if err != nil {
			return errors.Annotate(err, "failed to scale in")
		}
	default:
		return errors.Errorf("nonsupport %s", c.op)
	}

	return nil
}

// Rollback implements the Task interface
func (c *ClusterOperate) Rollback(ctx *Context) error {
	return ErrUnsupportedRollback
}

// String implements the fmt.Stringer interface
func (c *ClusterOperate) String() string {
	return fmt.Sprintf("ClusterOperate: operation=%s, options=%+v", c.op, c.options)
}
