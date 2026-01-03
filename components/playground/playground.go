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

package main

import (
	"context"
	"io"
	"sync"

	"github.com/pingcap/tiup/components/playground/proc"
	tuiv2output "github.com/pingcap/tiup/pkg/tuiv2/output"
	progressv2 "github.com/pingcap/tiup/pkg/tuiv2/progress"
)

// Playground represent the playground of a cluster.
type Playground struct {
	dataDir         string
	deleteWhenExit  bool
	bootOptions     *BootOptions
	bootBaseConfigs map[proc.ServiceID]proc.Config
	port            int

	// shutdownProcRecords snapshots controller-owned proc records at the moment
	// shutdown starts. It lets termination logic work after the controller loop
	// is canceled (no more events/commands).
	shutdownProcRecords []procRecordSnapshot

	bootCancel context.CancelCauseFunc

	shutdownOnce  sync.Once
	stoppingCh    chan struct{}
	interruptedCh chan struct{}
	processGroup  *ProcessGroup

	ui            *progressv2.UI
	startingGroup *progressv2.Group
	downloadGroup *progressv2.Group
	shutdownGroup *progressv2.Group

	// progressMu protects UI-related state that is accessed from multiple
	// goroutines (boot flow, stop handling, instance waiters), including:
	// - progress groups/tasks (download/starting/shutdown)
	// - procTitleCounts used for user-facing titles
	progressMu sync.Mutex

	// startingTasks holds one progress task per instance during boot, keyed by
	// inst.Info().Name(). It lets "Starting instances" show the
	// full instance list from the beginning (including components that start later,
	// like TiFlash).
	startingTasks map[string]*progressv2.Task

	// procTitleCounts is computed once during boot to decide whether to display
	// an instance index in user-facing output (e.g. "TiKV 0").
	//
	// It is refreshed when the instance set changes (boot, scale-out, scale-in).
	procTitleCounts map[string]int

	terminateDoneCh   chan struct{}
	terminateDoneOnce sync.Once

	controllerOnce   sync.Once
	controllerCancel context.CancelFunc
	cmdReqCh         chan commandRequest
	evtCh            chan controllerEvent
	controllerDoneCh chan struct{}
}

// NewPlayground create a Playground proc.
func NewPlayground(dataDir string, port int) *Playground {
	return &Playground{
		dataDir:         dataDir,
		port:            port,
		stoppingCh:      make(chan struct{}),
		interruptedCh:   make(chan struct{}),
		terminateDoneCh: make(chan struct{}),
		processGroup:    NewProcessGroup(),
	}
}

func (p *Playground) termWriter() io.Writer {
	if p != nil && p.ui != nil {
		return p.ui.Writer()
	}
	return tuiv2output.Stdout.Get()
}
