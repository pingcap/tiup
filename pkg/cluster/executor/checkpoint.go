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

package executor

import (
	"context"
	"fmt"
	"reflect"
	"time"

	"github.com/pingcap/tiup/pkg/checkpoint"
	"github.com/pingcap/tiup/pkg/cluster/ctxt"
	"go.uber.org/zap"
)

var (
	// register checkpoint for ssh command
	sshPoint = checkpoint.Register(
		checkpoint.Field("host", reflect.DeepEqual),
		checkpoint.Field("port", func(a, b any) bool {
			return fmt.Sprintf("%v", a) == fmt.Sprintf("%v", b)
		}),
		checkpoint.Field("user", reflect.DeepEqual),
		checkpoint.Field("sudo", reflect.DeepEqual),
		checkpoint.Field("cmd", reflect.DeepEqual),
	)

	// register checkpoint for scp command
	scpPoint = checkpoint.Register(
		checkpoint.Field("host", reflect.DeepEqual),
		checkpoint.Field("port", func(a, b any) bool {
			return fmt.Sprintf("%v", a) == fmt.Sprintf("%v", b)
		}),
		checkpoint.Field("user", reflect.DeepEqual),
		checkpoint.Field("src", reflect.DeepEqual),
		checkpoint.Field("dst", reflect.DeepEqual),
		checkpoint.Field("download", reflect.DeepEqual),
	)
)

// CheckPointExecutor wraps Executor and inject checkpoints
//
//	ATTENTION please: the result of CheckPointExecutor shouldn't be used to impact
//	                  external system like PD, otherwise, the external system may
//	                  take wrong action.
type CheckPointExecutor struct {
	ctxt.Executor
	config *SSHConfig
}

// Execute implements Executor interface.
func (c *CheckPointExecutor) Execute(ctx context.Context, cmd string, sudo bool, timeout ...time.Duration) (stdout []byte, stderr []byte, err error) {
	point := checkpoint.Acquire(ctx, sshPoint, map[string]any{
		"host": c.config.Host,
		"port": c.config.Port,
		"user": c.config.User,
		"sudo": sudo,
		"cmd":  cmd,
	})
	defer func() {
		point.Release(err,
			zap.String("host", c.config.Host),
			zap.Int("port", c.config.Port),
			zap.String("user", c.config.User),
			zap.Bool("sudo", sudo),
			zap.String("cmd", cmd),
			zap.String("stdout", string(stdout)),
			zap.String("stderr", string(stderr)),
		)
	}()
	if point.Hit() != nil {
		return []byte(point.Hit()["stdout"].(string)), []byte(point.Hit()["stderr"].(string)), nil
	}

	return c.Executor.Execute(ctx, cmd, sudo, timeout...)
}

// Transfer implements Executer interface.
func (c *CheckPointExecutor) Transfer(ctx context.Context, src, dst string, download bool, limit int, compress bool) (err error) {
	point := checkpoint.Acquire(ctx, scpPoint, map[string]any{
		"host":     c.config.Host,
		"port":     c.config.Port,
		"user":     c.config.User,
		"src":      src,
		"dst":      dst,
		"download": download,
		"limit":    limit,
		"compress": compress,
	})
	defer func() {
		point.Release(err,
			zap.String("host", c.config.Host),
			zap.Int("port", c.config.Port),
			zap.String("user", c.config.User),
			zap.String("src", src),
			zap.String("dst", dst),
			zap.Bool("download", download))
	}()
	if point.Hit() != nil {
		return nil
	}

	return c.Executor.Transfer(ctx, src, dst, download, limit, compress)
}
