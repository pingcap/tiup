// Copyright 2021 PingCAP, Inc.
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

package ctxt

import (
	"context"
	"runtime"
	"sync"
	"time"

	"github.com/pingcap/tiup/pkg/checkpoint"
	logprinter "github.com/pingcap/tiup/pkg/logger/printer"
	"github.com/pingcap/tiup/pkg/utils/mock"
)

type contextKey string

const (
	ctxKey = contextKey("TASK_CONTEXT")
)

const (
	// CtxBaseTopo is key of store the base topology in context.Context
	CtxBaseTopo = contextKey("BASE_TOPO")
)

type (
	// Executor is the executor interface for TiUP, all tasks will in the end
	// be passed to a executor and then be actually performed.
	Executor interface {
		// Execute run the command, then return it's stdout and stderr
		// NOTE: stdin is not supported as it seems we don't need it (for now). If
		// at some point in the future we need to pass stdin to a command, we'll
		// need to refactor this function and its implementations.
		// If the cmd can't quit in timeout, it will return error, the default timeout is 60 seconds.
		Execute(ctx context.Context, cmd string, sudo bool, timeout ...time.Duration) (stdout []byte, stderr []byte, err error)

		// Transfer copies files from or to a target
		Transfer(ctx context.Context, src, dst string, download bool, limit int, compress bool) error
	}

	// ExecutorGetter get the executor by host.
	ExecutorGetter interface {
		Get(host string) (e Executor)
		// GetSSHKeySet gets the SSH private and public key path
		GetSSHKeySet() (privateKeyPath, publicKeyPath string)
	}

	// Context is used to share state while multiple tasks execution.
	// We should use mutex to prevent concurrent R/W for some fields
	// because of the same context can be shared in parallel tasks.
	Context struct {
		mutex sync.RWMutex

		Ev EventBus

		exec struct {
			executors    map[string]Executor
			stdouts      map[string][]byte
			stderrs      map[string][]byte
			checkResults map[string][]interface{}
		}

		// The private/public key is used to access remote server via the user `tidb`
		PrivateKeyPath string
		PublicKeyPath  string

		Concurrency int // max number of parallel tasks running at the same time
	}
)

// New create a context instance.
func New(ctx context.Context, limit int, logger *logprinter.Logger) context.Context {
	concurrency := runtime.NumCPU()
	if limit > 0 {
		concurrency = limit
	}

	return context.WithValue(
		context.WithValue(
			checkpoint.NewContext(ctx),
			logprinter.ContextKeyLogger,
			logger,
		),
		ctxKey,
		&Context{
			mutex: sync.RWMutex{},
			Ev:    NewEventBus(),
			exec: struct {
				executors    map[string]Executor
				stdouts      map[string][]byte
				stderrs      map[string][]byte
				checkResults map[string][]interface{}
			}{
				executors:    make(map[string]Executor),
				stdouts:      make(map[string][]byte),
				stderrs:      make(map[string][]byte),
				checkResults: make(map[string][]interface{}),
			},
			Concurrency: concurrency, // default to CPU count
		},
	)
}

// GetInner return *Context from context.Context's value
func GetInner(ctx context.Context) *Context {
	return ctx.Value(ctxKey).(*Context)
}

// Get implements the operation.ExecutorGetter interface.
func (ctx *Context) Get(host string) (e Executor) {
	ctx.mutex.Lock()
	e, ok := ctx.exec.executors[host]
	ctx.mutex.Unlock()

	if !ok {
		panic("no init executor for " + host)
	}
	return
}

// GetSSHKeySet implements the operation.ExecutorGetter interface.
func (ctx *Context) GetSSHKeySet() (privateKeyPath, publicKeyPath string) {
	return ctx.PrivateKeyPath, ctx.PublicKeyPath
}

// GetExecutor get the executor.
func (ctx *Context) GetExecutor(host string) (e Executor, ok bool) {
	// Mock point for unit test
	if e := mock.On("FakeExecutor"); e != nil {
		return e.(Executor), true
	}

	ctx.mutex.RLock()
	e, ok = ctx.exec.executors[host]
	ctx.mutex.RUnlock()
	return
}

// SetExecutor set the executor.
func (ctx *Context) SetExecutor(host string, e Executor) {
	ctx.mutex.Lock()
	if e != nil {
		ctx.exec.executors[host] = e
	} else {
		delete(ctx.exec.executors, host)
	}
	ctx.mutex.Unlock()
}

// GetOutputs get the outputs of a host (if has any)
func (ctx *Context) GetOutputs(hostID string) ([]byte, []byte, bool) {
	ctx.mutex.RLock()
	stdout, ok1 := ctx.exec.stdouts[hostID]
	stderr, ok2 := ctx.exec.stderrs[hostID]
	ctx.mutex.RUnlock()
	return stdout, stderr, ok1 && ok2
}

// SetOutputs set the outputs of a host
func (ctx *Context) SetOutputs(hostID string, stdout []byte, stderr []byte) {
	ctx.mutex.Lock()
	ctx.exec.stdouts[hostID] = stdout
	ctx.exec.stderrs[hostID] = stderr
	ctx.mutex.Unlock()
}

// GetCheckResults get the the check result of a host (if has any)
func (ctx *Context) GetCheckResults(host string) (results []interface{}, ok bool) {
	ctx.mutex.RLock()
	results, ok = ctx.exec.checkResults[host]
	ctx.mutex.RUnlock()
	return
}

// SetCheckResults append the check result of a host to the list
func (ctx *Context) SetCheckResults(host string, results []interface{}) {
	ctx.mutex.Lock()
	if currResult, ok := ctx.exec.checkResults[host]; ok {
		ctx.exec.checkResults[host] = append(currResult, results...)
	} else {
		ctx.exec.checkResults[host] = results
	}
	ctx.mutex.Unlock()
}
