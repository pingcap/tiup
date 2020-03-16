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
	"fmt"
	"io"
	"strings"
	"sync"
)

type (
	// Task represents a operation while TiOps execution
	Task interface {
		Execute(ctx *Context) error
		Rollback(ctx *Context) error
	}

	// Context is used to share state while multiple tasks execution
	Context struct {
		SSHConnection io.ReadWriter
	}

	// Serial will execute a bundle of task in serialized way
	Serial []Task

	// Parallel will execute a bundle of task in parallelism way
	Parallel []Task
)

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

type errors []error

// Error implements the error interface
func (es errors) Error() string {
	ss := make([]string, 0, len(es))
	for _, e := range es {
		ss = append(ss, fmt.Sprintf("%+v", e))
	}
	return strings.Join(ss, "\n")
}

// Execute implements the Task interface
func (pt Parallel) Execute(ctx *Context) error {
	var es errors
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
	var es errors
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
