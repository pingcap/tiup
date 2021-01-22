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

package queue

// AnyQueue is a queue stores interface{}
type AnyQueue struct {
	eq    func(a interface{}, b interface{}) bool
	slice []interface{}
}

// NewAnyQueue builds a AnyQueue
func NewAnyQueue(eq func(a interface{}, b interface{}) bool, aa ...interface{}) *AnyQueue {
	return &AnyQueue{eq, aa}
}

// Get returns previous stored value that equals to val and remove it from the queue, if not found, return nil
func (q *AnyQueue) Get(val interface{}) interface{} {
	for i, a := range q.slice {
		if q.eq(a, val) {
			q.slice = append(q.slice[:i], q.slice[i+1:]...)
			return a
		}
	}
	return nil
}

// Put inserts `val` into `q`.
func (q *AnyQueue) Put(val interface{}) {
	q.slice = append(q.slice, val)
}
