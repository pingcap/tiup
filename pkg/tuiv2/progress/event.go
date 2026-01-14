package progress

import (
	"encoding/json"
	"time"
)

// EventVersion is the current JSON schema version for Event.
const EventVersion = 1

// EventType is the stable string representation of an event kind.
type EventType string

// Event types.
const (
	EventPrintLine    EventType = "print_line"
	EventBlankLine    EventType = "blank_line"
	EventGroupAdd     EventType = "group_add"
	EventGroupUpdate  EventType = "group_update"
	EventGroupClose   EventType = "group_close"
	EventGroupSeal    EventType = "group_seal"
	EventTaskAdd      EventType = "task_add"
	EventTaskUpdate   EventType = "task_update"
	EventTaskProgress EventType = "task_progress"
	EventTaskState    EventType = "task_state"
)

// TaskStatus is the stable string representation of a task status.
type TaskStatus string

// Task statuses.
const (
	TaskStatusPending  TaskStatus = "pending"
	TaskStatusRunning  TaskStatus = "running"
	TaskStatusRetrying TaskStatus = "retrying"
	TaskStatusDone     TaskStatus = "done"
	TaskStatusError    TaskStatus = "error"
	TaskStatusSkipped  TaskStatus = "skipped"
	TaskStatusCanceled TaskStatus = "canceled"
)

// TaskKind is the stable string representation of a task kind.
type TaskKind string

// Task kinds.
const (
	TaskKindGeneric  TaskKind = "generic"
	TaskKindDownload TaskKind = "download"
)

// Event is the canonical, append-only input to the tuiv2 progress engine.
//
// It is intentionally designed to be JSON-lines friendly so it can be persisted
// and replayed in daemon mode.
//
// Fields are mostly optional and interpreted based on Type.
type Event struct {
	// V is the schema version.
	V int `json:"v"`
	// Type is the event type discriminator.
	Type EventType `json:"type"`
	// At is the event timestamp.
	At time.Time `json:"at,omitempty"`

	// IDs (stable).
	GroupID uint64 `json:"gid,omitempty"`
	TaskID  uint64 `json:"tid,omitempty"`

	// PrintLine payload.
	Stream string `json:"stream,omitempty"`
	Text   string `json:"text,omitempty"`

	// Common "title" field (group/task add, group update).
	Title *string `json:"title,omitempty"`

	// Group options (group update).
	ShowMeta             *bool `json:"show_meta,omitempty"`
	HideDetailsOnSuccess *bool `json:"hide_details_on_success,omitempty"`
	SortTasksByTitle     *bool `json:"sort_tasks_by_title,omitempty"`

	// Task add.
	Pending bool `json:"pending,omitempty"`

	// Task update.
	Kind          *TaskKind `json:"kind,omitempty"`
	Meta          *string   `json:"meta,omitempty"`
	Message       *string   `json:"message,omitempty"`
	HideIfFast    *bool     `json:"hide_if_fast,omitempty"`
	RevealAfterMs *int64    `json:"reveal_after_ms,omitempty"`

	// Task progress.
	Current *int64 `json:"current,omitempty"`
	Total   *int64 `json:"total,omitempty"`

	// Task state transition.
	Status *TaskStatus `json:"status,omitempty"`
}

func (e Event) lossy() bool {
	return e.Type == EventTaskProgress
}

func parseEventLine(line []byte) (Event, error) {
	var e Event
	err := json.Unmarshal(line, &e)
	return e, err
}
