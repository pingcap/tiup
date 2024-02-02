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
	"strings"

	"github.com/pingcap/errors"
	"github.com/pingcap/tiup/pkg/cluster/ctxt"
)

var (
	sysctlFilePath = "/etc/sysctl.d/99-sysctl.conf"
)

// Sysctl set a kernel param on host
type Sysctl struct {
	host string
	key  string
	val  string
	sudo bool
}

// Execute implements the Task interface
func (s *Sysctl) Execute(ctx context.Context) error {
	e, ok := ctxt.GetInner(ctx).GetExecutor(s.host)
	if !ok {
		return ErrNoExecutor
	}

	cmd := strings.Join([]string{
		fmt.Sprintf("cp %s{,.bak} 2>/dev/null", sysctlFilePath),
		fmt.Sprintf("sed -i '/%s/d' %s 2>/dev/null", s.key, sysctlFilePath),
		fmt.Sprintf("echo '%s=%s' >> %s", s.key, s.val, sysctlFilePath),
		fmt.Sprintf("sysctl -p %s", sysctlFilePath),
	}, " && ")
	stdout, stderr, err := e.Execute(ctx, cmd, s.sudo)
	ctxt.GetInner(ctx).SetOutputs(s.host, stdout, stderr)
	if err != nil {
		return errors.Trace(err)
	}

	return nil
}

// Rollback implements the Task interface
func (s *Sysctl) Rollback(ctx context.Context) error {
	return ErrUnsupportedRollback
}

// String implements the fmt.Stringer interface
func (s *Sysctl) String() string {
	return fmt.Sprintf("Sysctl: host=%s %s = %s", s.host, s.key, s.val)
}
