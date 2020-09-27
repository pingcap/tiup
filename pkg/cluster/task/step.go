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
	"strings"

	"github.com/pingcap/tiup/pkg/cliutil/progress"
)

// StepDisplay is a task that will display a progress bar for inner task.
type StepDisplay struct {
	hidden      bool
	inner       Task
	prefix      string
	children    map[Task]struct{}
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

func newStepDisplay(prefix string, inner Task) *StepDisplay {
	children := make(map[Task]struct{})
	addChildren(children, inner)
	return &StepDisplay{
		inner:       inner,
		prefix:      prefix,
		children:    children,
		progressBar: progress.NewSingleBar(prefix),
	}
}

// SetHidden set step hidden or not.
func (s *StepDisplay) SetHidden(h bool) *StepDisplay {
	s.hidden = h
	return s
}

func (s *StepDisplay) resetAsMultiBarItem(b *progress.MultiBar) {
	s.progressBar = b.AddBar(s.prefix)
}

// Execute implements the Task interface
func (s *StepDisplay) Execute(ctx *Context) error {
	if s.hidden {
		return s.inner.Execute(ctx)
	}

	if singleBar, ok := s.progressBar.(*progress.SingleBar); ok {
		singleBar.StartRenderLoop()
	}
	ctx.ev.Subscribe(EventTaskBegin, s.handleTaskBegin)
	ctx.ev.Subscribe(EventTaskProgress, s.handleTaskProgress)
	err := s.inner.Execute(ctx)
	ctx.ev.Unsubscribe(EventTaskProgress, s.handleTaskProgress)
	ctx.ev.Unsubscribe(EventTaskBegin, s.handleTaskBegin)
	if err != nil {
		s.progressBar.UpdateDisplay(&progress.DisplayProps{
			Prefix: s.prefix,
			Mode:   progress.ModeError,
		})
	} else {
		s.progressBar.UpdateDisplay(&progress.DisplayProps{
			Prefix: s.prefix,
			Mode:   progress.ModeDone,
		})
	}
	if singleBar, ok := s.progressBar.(*progress.SingleBar); ok {
		singleBar.StopRenderLoop()
	}
	return err
}

// Rollback implements the Task interface
func (s *StepDisplay) Rollback(ctx *Context) error {
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
	s.progressBar.UpdateDisplay(&progress.DisplayProps{
		Prefix: s.prefix,
		Suffix: strings.Split(task.String(), "\n")[0],
	})
}

func (s *StepDisplay) handleTaskProgress(task Task, p string) {
	if _, ok := s.children[task]; !ok {
		return
	}
	s.progressBar.UpdateDisplay(&progress.DisplayProps{
		Prefix: s.prefix,
		Suffix: strings.Split(p, "\n")[0],
	})
}

// ParallelStepDisplay is a task that will display multiple progress bars in parallel for inner tasks.
// Inner tasks will be executed in parallel.
type ParallelStepDisplay struct {
	inner       *Parallel
	prefix      string
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

// Execute implements the Task interface
func (ps *ParallelStepDisplay) Execute(ctx *Context) error {
	ps.progressBar.StartRenderLoop()
	err := ps.inner.Execute(ctx)
	ps.progressBar.StopRenderLoop()
	return err
}

// Rollback implements the Task interface
func (ps *ParallelStepDisplay) Rollback(ctx *Context) error {
	return ps.inner.Rollback(ctx)
}

// String implements the fmt.Stringer interface
func (ps *ParallelStepDisplay) String() string {
	return ps.inner.String()
}
