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
	limitsFilePath = "/etc/security/limits.conf"
)

// Limit set a system limit on host
type Limit struct {
	host   string
	domain string // user or group
	limit  string // limit type
	item   string
	value  string
	sudo   bool
}

// Execute implements the Task interface
func (l *Limit) Execute(ctx context.Context) error {
	e, ok := ctxt.GetInner(ctx).GetExecutor(l.host)
	if !ok {
		return ErrNoExecutor
	}

	cmd := strings.Join([]string{
		fmt.Sprintf("cp %s{,.bak} 2>/dev/null", limitsFilePath),
		fmt.Sprintf("sed -i '/%s\\s*%s\\s*%s/d' %s 2>/dev/null",
			l.domain, l.limit, l.item, limitsFilePath),
		fmt.Sprintf("echo '%s    %s    %s    %s' >> %s",
			l.domain, l.limit, l.item, l.value, limitsFilePath),
	}, " && ")

	stdout, stderr, err := e.Execute(ctx, cmd, l.sudo)
	ctxt.GetInner(ctx).SetOutputs(l.host, stdout, stderr)
	if err != nil {
		return errors.Trace(err)
	}

	return nil
}

// Rollback implements the Task interface
func (l *Limit) Rollback(ctx context.Context) error {
	return ErrUnsupportedRollback
}

// String implements the fmt.Stringer interface
func (l *Limit) String() string {
	return fmt.Sprintf("Limit: host=%s %s %s %s %s", l.host, l.domain, l.limit, l.item, l.value)
}
