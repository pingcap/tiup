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

	"github.com/pingcap/tiup/pkg/cluster/executor"
	operator "github.com/pingcap/tiup/pkg/cluster/operation"
	"github.com/pingcap/tiup/pkg/logger/log"
	"github.com/pingcap/tiup/pkg/utils/mock"
)

var (
	// ErrUnsupportedRollback means the task do not support rollback.
	ErrUnsupportedRollback = stderrors.New("unsupported rollback")
	// ErrNoExecutor means not being able to get the executor.
	ErrNoExecutor = stderrors.New("no executor")
	// ErrNoOutput means not being able to get the output of host.
	ErrNoOutput = stderrors.New("no outputs available")
)

type (
	// Task represents a operation while TiOps execution
	Task interface {
		fmt.Stringer
		Execute(ctx *Context) error
		Rollback(ctx *Context) error
	}

	// Context is used to share state while multiple tasks execution.
	// We should use mutex to prevent concurrent R/W for some fields
	// because of the same context can be shared in parallel tasks.
	Context struct {
		ev EventBus

		exec struct {
			sync.RWMutex
			executors    map[string]executor.Executor
			stdouts      map[string][]byte
			stderrs      map[string][]byte
			checkResults map[string][]*operator.CheckResult
		}

		// The public/private key is used to access remote server via the user `tidb`
		PrivateKeyPath string
		PublicKeyPath  string
	}

	// Serial will execute a bundle of task in serialized way
	Serial struct {
		ignoreError       bool
		hideDetailDisplay bool
		inner             []Task
	}

	// Parallel will execute a bundle of task in parallelism way
	Parallel struct {
		ignoreError       bool
		hideDetailDisplay bool
		inner             []Task
	}
)

// NewContext create a context instance.
func NewContext() *Context {
	return &Context{
		ev: NewEventBus(),
		exec: struct {
			sync.RWMutex
			executors    map[string]executor.Executor
			stdouts      map[string][]byte
			stderrs      map[string][]byte
			checkResults map[string][]*operator.CheckResult
		}{
			executors:    make(map[string]executor.Executor),
			stdouts:      make(map[string][]byte),
			stderrs:      make(map[string][]byte),
			checkResults: make(map[string][]*operator.CheckResult),
		},
	}
}

// Get implements operation ExecutorGetter interface.
func (ctx *Context) Get(host string) (e executor.Executor) {
	ctx.exec.Lock()
	e, ok := ctx.exec.executors[host]
	ctx.exec.Unlock()

	if !ok {
		panic("no init executor for " + host)
	}
	return
}

// GetExecutor get the executor.
func (ctx *Context) GetExecutor(host string) (e executor.Executor, ok bool) {
	// Mock point for unit test
	if e := mock.On("FakeExecutor"); e != nil {
		return e.(executor.Executor), true
	}

	ctx.exec.RLock()
	e, ok = ctx.exec.executors[host]
	ctx.exec.RUnlock()
	return
}

// SetExecutor set the executor.
func (ctx *Context) SetExecutor(host string, e executor.Executor) {
	ctx.exec.Lock()
	ctx.exec.executors[host] = e
	ctx.exec.Unlock()
}

// GetOutputs get the outputs of a host (if has any)
func (ctx *Context) GetOutputs(host string) ([]byte, []byte, bool) {
	ctx.exec.RLock()
	stdout, ok1 := ctx.exec.stderrs[host]
	stderr, ok2 := ctx.exec.stdouts[host]
	ctx.exec.RUnlock()
	return stdout, stderr, ok1 && ok2
}

// SetOutputs set the outputs of a host
func (ctx *Context) SetOutputs(host string, stdout []byte, stderr []byte) {
	ctx.exec.Lock()
	ctx.exec.stderrs[host] = stdout
	ctx.exec.stdouts[host] = stderr
	ctx.exec.Unlock()
}

// GetCheckResults get the the check result of a host (if has any)
func (ctx *Context) GetCheckResults(host string) (results []*operator.CheckResult, ok bool) {
	ctx.exec.RLock()
	results, ok = ctx.exec.checkResults[host]
	ctx.exec.RUnlock()
	return
}

// SetCheckResults append the check result of a host to the list
func (ctx *Context) SetCheckResults(host string, results []*operator.CheckResult) {
	ctx.exec.Lock()
	if currResult, ok := ctx.exec.checkResults[host]; ok {
		ctx.exec.checkResults[host] = append(currResult, results...)
	} else {
		ctx.exec.checkResults[host] = results
	}
	ctx.exec.Unlock()
}

func isDisplayTask(t Task) bool {
	if _, ok := t.(*Serial); ok {
		return true
	}
	if _, ok := t.(*Parallel); ok {
		return true
	}
	if _, ok := t.(*StepDisplay); ok {
		return true
	}
	if _, ok := t.(*ParallelStepDisplay); ok {
		return true
	}
	return false
}

// Execute implements the Task interface
func (s *Serial) Execute(ctx *Context) error {
	for _, t := range s.inner {
		if !isDisplayTask(t) {
			if !s.hideDetailDisplay {
				log.Infof("+ [ Serial ] - %s", t.String())
			}
		}
		ctx.ev.PublishTaskBegin(t)
		err := t.Execute(ctx)
		ctx.ev.PublishTaskFinish(t, err)
		if err != nil && !s.ignoreError {
			return err
		}
	}
	return nil
}

// Rollback implements the Task interface
func (s *Serial) Rollback(ctx *Context) error {
	// Rollback in reverse order
	for i := len(s.inner) - 1; i >= 0; i-- {
		err := s.inner[i].Rollback(ctx)
		if err != nil {
			return err
		}
	}
	return nil
}

// String implements the fmt.Stringer interface
func (s *Serial) String() string {
	var ss []string
	for _, t := range s.inner {
		ss = append(ss, t.String())
	}
	return strings.Join(ss, "\n")
}

// Execute implements the Task interface
func (pt *Parallel) Execute(ctx *Context) error {
	var firstError error
	var mu sync.Mutex
	wg := sync.WaitGroup{}
	for _, t := range pt.inner {
		wg.Add(1)
		go func(t Task) {
			defer wg.Done()
			if !isDisplayTask(t) {
				if !pt.hideDetailDisplay {
					log.Infof("+ [Parallel] - %s", t.String())
				}
			}
			ctx.ev.PublishTaskBegin(t)
			err := t.Execute(ctx)
			ctx.ev.PublishTaskFinish(t, err)
			if err != nil {
				mu.Lock()
				if firstError == nil {
					firstError = err
				}
				mu.Unlock()
			}
		}(t)
	}
	wg.Wait()
	if pt.ignoreError {
		return nil
	}
	return firstError
}

// Rollback implements the Task interface
func (pt *Parallel) Rollback(ctx *Context) error {
	var firstError error
	var mu sync.Mutex
	wg := sync.WaitGroup{}
	for _, t := range pt.inner {
		wg.Add(1)
		go func(t Task) {
			defer wg.Done()
			err := t.Rollback(ctx)
			if err != nil {
				mu.Lock()
				if firstError == nil {
					firstError = err
				}
				mu.Unlock()
			}
		}(t)
	}
	wg.Wait()
	return firstError
}

// String implements the fmt.Stringer interface
func (pt *Parallel) String() string {
	var ss []string
	for _, t := range pt.inner {
		ss = append(ss, t.String())
	}
	return strings.Join(ss, "\n")
}
