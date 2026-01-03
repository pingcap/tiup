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
	booted          bool
	bootOptions     *BootOptions
	bootBaseConfigs map[proc.ServiceID]proc.Config
	port            int

	startedProcs []proc.Process

	// procs holds all running (or planned) component processes grouped by ServiceID.
	//
	// It replaces the previous "one field per service" slices, so service-level
	// behaviors (walk/display/scale/remove) can be implemented generically.
	//
	// It is owned by the controller goroutine after boot, so it is intentionally
	// not protected by a mutex. Boot code builds it before the command server is
	// started.
	procs map[proc.ServiceID][]proc.Process

	requiredServices map[proc.ServiceID]int
	criticalRunning  map[proc.ServiceID]int
	bootCancel       context.CancelCauseFunc

	shutdownOnce  sync.Once
	stoppingCh    chan struct{}
	interruptedCh chan struct{}

	expectedExit map[int]struct{}

	idAlloc      map[proc.ServiceID]int
	processGroup *ProcessGroup

	ui            *progressv2.UI
	startingGroup *progressv2.Group
	downloadGroup *progressv2.Group
	shutdownGroup *progressv2.Group

	// progressMu protects UI-related state that is accessed from multiple
	// goroutines (boot flow, stop handling, instance waiters), including:
	// - progress groups/tasks (download/starting/shutdown)
	// - startedProcs snapshots for interrupt handling
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
	cmdReqCh         chan commandRequest
	evtCh            chan controllerEvent
	controllerDoneCh chan struct{}
}

// NewPlayground create a Playground proc.
func NewPlayground(dataDir string, port int) *Playground {
	return &Playground{
		dataDir:         dataDir,
		port:            port,
		idAlloc:         make(map[proc.ServiceID]int),
		criticalRunning: make(map[proc.ServiceID]int),
		expectedExit:    make(map[int]struct{}),
		procs:           make(map[proc.ServiceID][]proc.Process),
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
