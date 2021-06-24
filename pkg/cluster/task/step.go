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
	"encoding/json"
	"fmt"
	"strings"

	"github.com/pingcap/tiup/pkg/cluster/ctxt"
	"github.com/pingcap/tiup/pkg/logger/log"
	"github.com/pingcap/tiup/pkg/tui/progress"
)

// StepDisplay is a task that will display a progress bar for inner task.
type StepDisplay struct {
	hidden      bool
	inner       Task
	prefix      string
	children    map[Task]struct{}
	DisplayMode log.DisplayMode
	progressBar progress.Bar
}

func addChildren(m map[Task]struct{}, task Task) {
	if _, exists := m[task]; exists {
		return
	}
	m[task] = struct{}{}
	if t, ok := task.(*Serial); ok {
		t.hideDetailDisplay = true
		for _, tx := range t.inner {
			if _, exists := m[tx]; !exists {
				addChildren(m, tx)
			}
		}
	} else if t, ok := task.(*Parallel); ok {
		t.hideDetailDisplay = true
		for _, tx := range t.inner {
			if _, exists := m[tx]; !exists {
				addChildren(m, tx)
			}
		}
	}
}

func newStepDisplay(prefix string, inner Task, m log.DisplayMode) *StepDisplay {
	children := make(map[Task]struct{})
	addChildren(children, inner)
	return &StepDisplay{
		inner:       inner,
		prefix:      prefix,
		children:    children,
		DisplayMode: m,
		progressBar: progress.NewSingleBar(prefix),
	}
}

// SetHidden set step hidden or not.
func (s *StepDisplay) SetHidden(h bool) *StepDisplay {
	s.hidden = h
	return s
}

// SetDisplayMode set the value of log.DisplayMode flag
func (s *StepDisplay) SetDisplayMode(m log.DisplayMode) *StepDisplay {
	s.DisplayMode = m
	return s
}

func (s *StepDisplay) resetAsMultiBarItem(b *progress.MultiBar) {
	s.progressBar = b.AddBar(s.prefix)
}

// Execute implements the Task interface
func (s *StepDisplay) Execute(ctx context.Context) error {
	if s.hidden {
		ctxt.GetInner(ctx).Ev.Subscribe(ctxt.EventTaskBegin, s.handleTaskBegin)
		ctxt.GetInner(ctx).Ev.Subscribe(ctxt.EventTaskProgress, s.handleTaskProgress)
		err := s.inner.Execute(ctx)
		ctxt.GetInner(ctx).Ev.Unsubscribe(ctxt.EventTaskProgress, s.handleTaskProgress)
		ctxt.GetInner(ctx).Ev.Unsubscribe(ctxt.EventTaskBegin, s.handleTaskBegin)
		return err
	}

	switch s.DisplayMode {
	case log.DisplayModeJSON:
		break
	default:
		if singleBar, ok := s.progressBar.(*progress.SingleBar); ok {
			singleBar.StartRenderLoop()
			defer singleBar.StopRenderLoop()
		}
	}

	ctxt.GetInner(ctx).Ev.Subscribe(ctxt.EventTaskBegin, s.handleTaskBegin)
	ctxt.GetInner(ctx).Ev.Subscribe(ctxt.EventTaskProgress, s.handleTaskProgress)
	err := s.inner.Execute(ctx)
	ctxt.GetInner(ctx).Ev.Unsubscribe(ctxt.EventTaskProgress, s.handleTaskProgress)
	ctxt.GetInner(ctx).Ev.Unsubscribe(ctxt.EventTaskBegin, s.handleTaskBegin)

	var dp *progress.DisplayProps
	if err != nil {
		dp = &progress.DisplayProps{
			Prefix: s.prefix,
			Mode:   progress.ModeError,
		}
	} else {
		dp = &progress.DisplayProps{
			Prefix: s.prefix,
			Mode:   progress.ModeDone,
		}
	}

	switch s.DisplayMode {
	case log.DisplayModeJSON:
		return printDp(dp)
	default:
		s.progressBar.UpdateDisplay(dp)
	}
	return nil
}

// Rollback implements the Task interface
func (s *StepDisplay) Rollback(ctx context.Context) error {
	return s.inner.Rollback(ctx)
}

// String implements the fmt.Stringer interface
func (s *StepDisplay) String() string {
	return s.inner.String()
}

func (s *StepDisplay) handleTaskBegin(task Task) {
	if _, ok := s.children[task]; !ok {
		return
	}
	dp := &progress.DisplayProps{
		Prefix: s.prefix,
		Suffix: strings.Split(task.String(), "\n")[0],
	}
	switch s.DisplayMode {
	case log.DisplayModeJSON:
		_ = printDp(dp)
	default:
		s.progressBar.UpdateDisplay(dp)
	}
}

func (s *StepDisplay) handleTaskProgress(task Task, p string) {
	if _, ok := s.children[task]; !ok {
		return
	}
	dp := &progress.DisplayProps{
		Prefix: s.prefix,
		Suffix: strings.Split(p, "\n")[0],
	}
	switch s.DisplayMode {
	case log.DisplayModeJSON:
		_ = printDp(dp)
	default:
		s.progressBar.UpdateDisplay(dp)
	}
}

// ParallelStepDisplay is a task that will display multiple progress bars in parallel for inner tasks.
// Inner tasks will be executed in parallel.
type ParallelStepDisplay struct {
	inner       *Parallel
	prefix      string
	DisplayMode log.DisplayMode
	progressBar *progress.MultiBar
}

func newParallelStepDisplay(prefix string, ignoreError bool, sdTasks ...*StepDisplay) *ParallelStepDisplay {
	bar := progress.NewMultiBar(prefix)
	tasks := make([]Task, 0, len(sdTasks))
	for _, t := range sdTasks {
		if !t.hidden {
			t.resetAsMultiBarItem(bar)
		}
		tasks = append(tasks, t)
	}
	return &ParallelStepDisplay{
		inner:       &Parallel{inner: tasks, ignoreError: ignoreError},
		prefix:      prefix,
		progressBar: bar,
	}
}

// SetDisplayMode set the value of log.DisplayMode flag
func (ps *ParallelStepDisplay) SetDisplayMode(m log.DisplayMode) *ParallelStepDisplay {
	ps.DisplayMode = m
	return ps
}

// Execute implements the Task interface
func (ps *ParallelStepDisplay) Execute(ctx context.Context) error {
	switch ps.DisplayMode {
	case log.DisplayModeJSON:
		break
	default:
		ps.progressBar.StartRenderLoop()
		defer ps.progressBar.StopRenderLoop()
	}
	err := ps.inner.Execute(ctx)
	return err
}

// Rollback implements the Task interface
func (ps *ParallelStepDisplay) Rollback(ctx context.Context) error {
	return ps.inner.Rollback(ctx)
}

// String implements the fmt.Stringer interface
func (ps *ParallelStepDisplay) String() string {
	return ps.inner.String()
}

func printDp(dp *progress.DisplayProps) error {
	output, err := json.Marshal(dp)
	if err != nil {
		return err
	}
	fmt.Println(string(output))
	return nil
}
