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
	stderrors "errors"
	"fmt"
	"strings"
	"sync"

	"github.com/pingcap-incubator/tiops/pkg/executor"
)

var (
	// ErrUnsupportRollback means the task do not support rollback.
	ErrUnsupportRollback = stderrors.New("unsupport rollback")
	// ErrNoExecutor means can get the executor.
	ErrNoExecutor = stderrors.New("no executor")
)

type (
	// Task represents a operation while TiOps execution
	Task interface {
		Execute(ctx *Context) error
		Rollback(ctx *Context) error
	}

	// Context is used to share state while multiple tasks execution
	Context struct {
		exec struct {
			sync.Mutex
			executors map[string]executor.TiOpsExecutor
		}
	}

	// Serial will execute a bundle of task in serialized way
	Serial []Task

	// Parallel will execute a bundle of task in parallelism way
	Parallel []Task
)

// NewContext create a context instance.
func NewContext() *Context {
	return &Context{
		exec: struct {
			sync.Mutex
			executors map[string]executor.TiOpsExecutor
		}{
			executors: make(map[string]executor.TiOpsExecutor),
		},
	}
}

// GetExecutor get the executor.
func (ctx *Context) GetExecutor(host string) (e executor.TiOpsExecutor, ok bool) {
	ctx.exec.Lock()
	e, ok = ctx.exec.executors[host]
	ctx.exec.Unlock()
	return
}

// SetExecutor set the executor.
func (ctx *Context) SetExecutor(host string, e executor.TiOpsExecutor) {
	ctx.exec.Lock()
	ctx.exec.executors[host] = e
	ctx.exec.Unlock()
	return
}

// Execute implements the Task interface
func (s Serial) Execute(ctx *Context) error {
	for _, t := range s {
		err := t.Execute(ctx)
		if err != nil {
			return err
		}
	}
	return nil
}

// Rollback implements the Task interface
func (s Serial) Rollback(ctx *Context) error {
	for _, t := range s {
		err := t.Rollback(ctx)
		if err != nil {
			return err
		}
	}
	return nil
}

type errs []error

// Error implements the error interface
func (es errs) Error() string {
	ss := make([]string, 0, len(es))
	for _, e := range es {
		ss = append(ss, fmt.Sprintf("%+v", e))
	}
	return strings.Join(ss, "\n")
}

// Execute implements the Task interface
func (pt Parallel) Execute(ctx *Context) error {
	var es errs
	var mu sync.Mutex
	wg := sync.WaitGroup{}
	for _, t := range pt {
		wg.Add(1)
		go func(t Task) {
			err := t.Execute(ctx)
			if err != nil {
				mu.Lock()
				es = append(es, err)
				mu.Unlock()
			}
		}(t)
	}
	wg.Wait()
	return es
}

// Rollback implements the Task interface
func (pt Parallel) Rollback(ctx *Context) error {
	var es errs
	var mu sync.Mutex
	wg := sync.WaitGroup{}
	for _, t := range pt {
		wg.Add(1)
		go func(t Task) {
			err := t.Rollback(ctx)
			if err != nil {
				mu.Lock()
				es = append(es, err)
				mu.Unlock()
			}
		}(t)
	}
	wg.Wait()
	return es
}
