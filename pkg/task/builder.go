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

// Builder is used to build TiOps task
type Builder struct {
	tasks []Task
}

// NewBuilder returns a *Builder instance
func NewBuilder() *Builder {
	return &Builder{}
}

// SSH appends a SSH task to the current task collection
func (b *Builder) SSH(host, keypath, user string) *Builder {
	b.tasks = append(b.tasks, SSH{
		host:    host,
		keypath: keypath,
		user:    user,
	})
	return b
}

// CopyFile appends a CopyFile task to the current task collection
func (b *Builder) CopyFile(src, dstHost, dstPath string) *Builder {
	b.tasks = append(b.tasks, &CopyFile{
		src:     src,
		dstHost: dstHost,
		dstPath: dstPath,
	})
	return b
}

// Parallel appends a parallel task to the current task collection
func (b *Builder) Parallel(tasks ...Task) *Builder {
	b.tasks = append(b.tasks, Parallel(tasks))
	return b
}

// Build returns a task that contains all tasks appended by previous operation
func (b *Builder) Build() Task {
	return Serial(b.tasks)
}
