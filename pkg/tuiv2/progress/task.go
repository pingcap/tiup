package progress

import "time"

// Task represents one line item in a group.
//
// Task is a lightweight handle: it emits events into the UI engine.
// It is safe to use from any goroutine.
type Task struct {
	ui *UI

	id      uint64
	groupID uint64

	// title is best-effort local cache for debugging only.
	title string
}

// SetHideIfFast configures this task to be hidden in TTY mode unless it runs for
// at least revealAfter, or errors.
func (t *Task) SetHideIfFast(revealAfter time.Duration) {
	if t == nil || t.ui == nil || t.ui.closed.Load() {
		return
	}
	if revealAfter < 0 {
		revealAfter = 0
	}
	hide := true
	ms := int64(revealAfter / time.Millisecond)
	t.ui.emit(Event{
		Type:          EventTaskUpdate,
		At:            t.ui.now(),
		TaskID:        t.id,
		HideIfFast:    &hide,
		RevealAfterMs: &ms,
	})
}

// SetKindDownload marks this task as a download task.
func (t *Task) SetKindDownload() {
	if t == nil || t.ui == nil || t.ui.closed.Load() {
		return
	}
	kind := TaskKindDownload
	t.ui.emit(Event{
		Type:   EventTaskUpdate,
		At:     t.ui.now(),
		TaskID: t.id,
		Kind:   &kind,
	})
}

// Start marks the task as running. It is safe to call Start multiple times.
func (t *Task) Start() {
	if t == nil || t.ui == nil || t.ui.closed.Load() {
		return
	}
	status := TaskStatusRunning
	t.ui.emit(Event{
		Type:   EventTaskState,
		At:     t.ui.now(),
		TaskID: t.id,
		Status: &status,
	})
}

// SetMessage sets a human-readable message for this task.
func (t *Task) SetMessage(msg string) {
	if t == nil || t.ui == nil || t.ui.closed.Load() {
		return
	}
	m := msg
	t.ui.emit(Event{
		Type:    EventTaskUpdate,
		At:      t.ui.now(),
		TaskID:  t.id,
		Message: &m,
	})
}

// Retrying marks the task as retrying with a message, while keeping it active.
func (t *Task) Retrying(msg string) {
	if t == nil || t.ui == nil || t.ui.closed.Load() {
		return
	}
	status := TaskStatusRetrying
	m := msg
	t.ui.emit(Event{
		Type:    EventTaskState,
		At:      t.ui.now(),
		TaskID:  t.id,
		Status:  &status,
		Message: &m,
	})
}

// SetMeta sets stable, user-facing metadata for this task (e.g. component
// version for downloads).
func (t *Task) SetMeta(meta string) {
	if t == nil || t.ui == nil || t.ui.closed.Load() {
		return
	}
	m := meta
	t.ui.emit(Event{
		Type:   EventTaskUpdate,
		At:     t.ui.now(),
		TaskID: t.id,
		Meta:   &m,
	})
}

// SetTotal sets the progress total for this task.
func (t *Task) SetTotal(total int64) {
	if t == nil || t.ui == nil || t.ui.closed.Load() {
		return
	}
	v := total
	t.ui.emit(Event{
		Type:   EventTaskProgress,
		At:     t.ui.now(),
		TaskID: t.id,
		Total:  &v,
	})
}

// SetCurrent sets the progress current value for this task.
func (t *Task) SetCurrent(current int64) {
	if t == nil || t.ui == nil || t.ui.closed.Load() {
		return
	}
	v := current
	t.ui.emit(Event{
		Type:    EventTaskProgress,
		At:      t.ui.now(),
		TaskID:  t.id,
		Current: &v,
	})
}

// Done marks the task as successfully completed.
func (t *Task) Done() {
	if t == nil || t.ui == nil || t.ui.closed.Load() {
		return
	}
	status := TaskStatusDone
	t.ui.emit(Event{
		Type:   EventTaskState,
		At:     t.ui.now(),
		TaskID: t.id,
		Status: &status,
	})
}

// Error marks the task as failed with a message.
func (t *Task) Error(msg string) {
	if t == nil || t.ui == nil || t.ui.closed.Load() {
		return
	}
	status := TaskStatusError
	m := msg
	t.ui.emit(Event{
		Type:    EventTaskState,
		At:      t.ui.now(),
		TaskID:  t.id,
		Status:  &status,
		Message: &m,
	})
}

// Skip marks the task as skipped with an optional reason.
func (t *Task) Skip(reason string) {
	if t == nil || t.ui == nil || t.ui.closed.Load() {
		return
	}
	status := TaskStatusSkipped
	r := reason
	t.ui.emit(Event{
		Type:    EventTaskState,
		At:      t.ui.now(),
		TaskID:  t.id,
		Status:  &status,
		Message: &r,
	})
}

// Cancel marks the task as canceled with an optional reason.
func (t *Task) Cancel(reason string) {
	if t == nil || t.ui == nil || t.ui.closed.Load() {
		return
	}
	status := TaskStatusCanceled
	r := reason
	t.ui.emit(Event{
		Type:    EventTaskState,
		At:      t.ui.now(),
		TaskID:  t.id,
		Status:  &status,
		Message: &r,
	})
}
