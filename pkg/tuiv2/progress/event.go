package progress

import (
	"encoding/json"
	"time"
)

// EventType is the stable string representation of an event kind.
type EventType string

// Event types.
const (
	// EventPrintLines prints one or more text lines as a single output block.
	//
	// A blank line can be represented as `EventPrintLines{Lines: []string{""}}`.
	EventPrintLines EventType = "print_lines"
	// EventSync is an internal barrier event.
	//
	// It is emitted by UI.Sync and allows callers to wait until all previously
	// emitted events are processed (and persisted to the event log when enabled).
	//
	// Renderers should ignore it.
	EventSync         EventType = "sync"
	EventGroupAdd     EventType = "group_add"
	EventGroupUpdate  EventType = "group_update"
	EventGroupClose   EventType = "group_close"
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
// It is intentionally designed to be JSON-lines friendly.
//
// Fields are mostly optional and interpreted based on Type.
type Event struct {
	// Type is the event type discriminator.
	Type EventType `json:"type"`
	// At is the event timestamp.
	At time.Time `json:"at,omitempty"`

	// IDs (stable).
	GroupID uint64 `json:"gid,omitempty"`
	TaskID  uint64 `json:"tid,omitempty"`

	// PrintLines payload.
	Lines []string `json:"lines,omitempty"`

	// Sync payload.
	SyncID uint64 `json:"sync_id,omitempty"`

	// Common "title" field (group/task add, group update).
	Title *string `json:"title,omitempty"`

	// Group options (group update).
	ShowMeta             *bool `json:"show_meta,omitempty"`
	HideDetailsOnSuccess *bool `json:"hide_details_on_success,omitempty"`
	SortTasksByTitle     *bool `json:"sort_tasks_by_title,omitempty"`
	// Group close.
	//
	// Finished=false means "seal snapshot": the group is moved from Active to
	// History immediately (used for interrupts). When omitted, it defaults to
	// true ("normal close").
	Finished *bool `json:"finished,omitempty"`

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

func parseEventLine(line []byte) (Event, error) {
	var e Event
	err := json.Unmarshal(line, &e)
	return e, err
}
