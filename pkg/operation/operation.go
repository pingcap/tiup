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

package operator

import (
	"fmt"
	"time"

	"github.com/pingcap-incubator/tiops/pkg/executor"
	"github.com/pingcap-incubator/tiops/pkg/meta"
)

// Options represents the operation options
type Options struct {
	Role         string
	Node         string
	Force        bool // Option for upgrade subcommand
	DeletedNodes []string
}

// Operation represents the type of cluster operation
type Operation byte

// Operation represents the kind of cluster operation
const (
	StartOperation Operation = iota
	StopOperation
	RestartOperation
	DestroyOperation
	UpgradeOperation
	ScaleInOperation
	ScaleOutOperation
)

var defaultTimeoutForReady = time.Second * 60

var opStringify = [...]string{
	"StartOperation",
	"StopOperation",
	"RestartOperation",
	"DestroyOperation",
	"UpgradeOperation",
	"ScaleInOperation",
	"ScaleOutOperation",
}

func (op Operation) String() string {
	if op <= ScaleOutOperation {
		return opStringify[op]
	}
	return fmt.Sprintf("unknonw-op(%d)", op)
}

func filterComponent(comps []meta.Component, component string) (res []meta.Component) {
	if component == "" {
		res = comps
		return
	}

	for _, c := range comps {
		if c.Name() != component {
			continue
		}

		res = append(res, c)
	}

	return
}

func filterInstance(instances []meta.Instance, node string) (res []meta.Instance) {
	if node == "" {
		res = instances
		return
	}

	for _, c := range instances {
		if c.ID() != node {
			continue
		}
		res = append(res, c)
	}

	return
}

// ExecutorGetter get the executor by host.
type ExecutorGetter interface {
	Get(host string) (e executor.TiOpsExecutor)
}
