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

package ctxt

import (
	"fmt"

	ev "github.com/asaskevich/EventBus"
	"go.uber.org/zap"
)

// EventBus is an event bus for task events.
type EventBus struct {
	eventBus ev.Bus
}

// EventKind is the task event kind.
type EventKind string

const (
	// EventTaskBegin is emitted when a task is going to be executed.
	EventTaskBegin EventKind = "task_begin"
	// EventTaskFinish is emitted when a task finishes executing.
	EventTaskFinish EventKind = "task_finish"
	// EventTaskProgress is emitted when a task has made some progress.
	EventTaskProgress EventKind = "task_progress"
)

// NewEventBus creates a new EventBus.
func NewEventBus() EventBus {
	return EventBus{
		eventBus: ev.New(),
	}
}

// PublishTaskBegin publishes a TaskBegin event. This should be called only by Parallel or Serial.
func (ev *EventBus) PublishTaskBegin(task fmt.Stringer) {
	zap.L().Debug("TaskBegin", zap.String("task", task.String()))
	ev.eventBus.Publish(string(EventTaskBegin), task)
}

// PublishTaskFinish publishes a TaskFinish event. This should be called only by Parallel or Serial.
func (ev *EventBus) PublishTaskFinish(task fmt.Stringer, err error) {
	zap.L().Debug("TaskFinish", zap.String("task", task.String()), zap.Error(err))
	ev.eventBus.Publish(string(EventTaskFinish), task, err)
}

// PublishTaskProgress publishes a TaskProgress event.
func (ev *EventBus) PublishTaskProgress(task fmt.Stringer, progress string) {
	zap.L().Debug("TaskProgress", zap.String("task", task.String()), zap.String("progress", progress))
	ev.eventBus.Publish(string(EventTaskProgress), task, progress)
}

// Subscribe subscribes events.
func (ev *EventBus) Subscribe(eventName EventKind, handler any) {
	err := ev.eventBus.Subscribe(string(eventName), handler)
	if err != nil {
		panic(err)
	}
}

// Unsubscribe unsubscribes events.
func (ev *EventBus) Unsubscribe(eventName EventKind, handler any) {
	err := ev.eventBus.Unsubscribe(string(eventName), handler)
	if err != nil {
		panic(err)
	}
}
