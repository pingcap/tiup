package progress

import (
	"time"
)

type taskStatus int

const (
	taskStatusPending taskStatus = iota
	taskStatusRunning
	taskStatusDone
	taskStatusError
	taskStatusSkipped
	taskStatusCanceled
)

type taskKind int

const (
	taskKindGeneric taskKind = iota
	taskKindDownload
)

// Task represents one line item in a group.
//
// All methods are goroutine-safe.
type Task struct {
	g     *Group
	title string

	kind   taskKind
	status taskStatus
	// hideIfFast hides this task in TTY mode unless it is slow or errors.
	//
	// This is useful for "background" tasks where the happy path is not
	// interesting, but failures (or unusually long operations) should still be
	// surfaced.
	//
	// Behavior in ModeTTY:
	// - pending: hidden
	// - running: shown only when running for >= revealAfter
	// - done/skipped/canceled: hidden
	// - error: always shown
	hideIfFast  bool
	revealAfter time.Duration
	// meta holds stable, user-facing metadata for this task (e.g. component
	// version for downloads). Unlike message, it is not overwritten by Error().
	meta    string
	message string

	current int64
	total   int64

	startAt time.Time
	endAt   time.Time

	// Download speed (bytes/sec), computed from SetCurrent updates.
	speedBps float64

	// Speed calculation state.
	lastSpeedAt    time.Time
	lastSpeedBytes int64

	plainStartPrinted    bool
	downloadStartPrinted bool
}

// SetHideIfFast configures this task to be hidden in TTY mode unless it runs for
// at least revealAfter, or errors.
//
// It is useful for tasks that are usually quick and uninteresting on success
// (e.g. background monitoring shutdown), but should still be visible when they
// become slow or fail.
func (t *Task) SetHideIfFast(revealAfter time.Duration) {
	if t == nil || t.g == nil || t.g.ui == nil {
		return
	}
	ui := t.g.ui

	ui.mu.Lock()
	defer ui.mu.Unlock()

	if ui.closed {
		return
	}

	if revealAfter < 0 {
		revealAfter = 0
	}
	t.hideIfFast = true
	t.revealAfter = revealAfter
	ui.markDirtyLocked()
}

// SetKindDownload marks this task as a download task.
//
// In TTY mode, download tasks render with byte counters, an optional progress
// bar, and an estimated transfer speed.
//
// In plain mode, it emits a stable "Downloading ..." start event.
func (t *Task) SetKindDownload() {
	t.setKind(taskKindDownload)
}

// Start marks the task as running. It is safe to call Start multiple times.
func (t *Task) Start() {
	if t == nil || t.g == nil || t.g.ui == nil {
		return
	}
	ui := t.g.ui

	ui.mu.Lock()
	defer ui.mu.Unlock()

	if ui.closed {
		return
	}
	if t.status != taskStatusRunning {
		// Do not allow restarting already completed tasks.
		switch t.status {
		case taskStatusDone, taskStatusError, taskStatusSkipped, taskStatusCanceled:
			return
		}
	}

	now := time.Now()
	t.status = taskStatusRunning

	if ui.mode == ModePlain && !t.g.plainBegun {
		t.g.plainBegun = true
		ui.printPlainLineLocked(ui.plainGroupHeader(t.g.title))
	}
	if t.g.startedAt.IsZero() {
		t.g.startedAt = now
	}

	// In ModePlain, emit a start event for generic tasks so logs show what the
	// program is currently doing (similar to rustc/cargo output).
	if ui.mode == ModePlain && !t.plainStartPrinted && t.kind != taskKindDownload {
		t.plainStartPrinted = true
		t.startAt = now
		ui.printPlainTaskStartLocked(t)
	}
	if t.startAt.IsZero() {
		t.startAt = now
	}
	ui.markDirtyLocked()
}

// SetMessage sets a human-readable message for this task.
func (t *Task) SetMessage(msg string) {
	if t == nil || t.g == nil || t.g.ui == nil {
		return
	}
	ui := t.g.ui

	ui.mu.Lock()
	defer ui.mu.Unlock()

	if ui.closed {
		return
	}
	t.message = msg
	ui.markDirtyLocked()
}

// SetMeta sets stable, user-facing metadata for this task (e.g. component
// version for downloads).
//
// Unlike SetMessage, metadata is not overwritten by Error().
func (t *Task) SetMeta(meta string) {
	if t == nil || t.g == nil || t.g.ui == nil {
		return
	}
	ui := t.g.ui

	ui.mu.Lock()
	defer ui.mu.Unlock()

	if ui.closed {
		return
	}
	t.meta = meta
	ui.markDirtyLocked()
}

// SetTotal sets the progress total for this task.
func (t *Task) SetTotal(total int64) {
	if t == nil || t.g == nil || t.g.ui == nil {
		return
	}
	ui := t.g.ui

	ui.mu.Lock()
	defer ui.mu.Unlock()

	if ui.closed {
		return
	}
	t.total = total
	ui.markDirtyLocked()
}

// SetCurrent sets the progress current value for this task.
func (t *Task) SetCurrent(current int64) {
	if t == nil || t.g == nil || t.g.ui == nil {
		return
	}
	ui := t.g.ui

	ui.mu.Lock()
	defer ui.mu.Unlock()

	if ui.closed {
		return
	}
	if current < 0 {
		current = 0
	}
	if t.status != taskStatusRunning {
		// Ignore late updates after completion.
		return
	}

	now := time.Now()
	t.current = current

	// Speed is only meaningful for download-like tasks.
	if t.kind == taskKindDownload {
		// Compute download speed from a time window rather than per-callback
		// instantaneous deltas, because download progress callbacks can be very
		// frequent and jittery.
		//
		// We still apply a lightweight EWMA on top to keep the output stable.
		const (
			minWindow = time.Second
			alpha     = 0.2
		)

		if t.lastSpeedAt.IsZero() || !now.After(t.lastSpeedAt) {
			t.lastSpeedAt = now
			t.lastSpeedBytes = current
		} else {
			dt := now.Sub(t.lastSpeedAt)
			delta := current - t.lastSpeedBytes
			switch {
			case delta < 0:
				// Best-effort: if upstream ever reports a decreasing current, treat it
				// as a reset and restart the speed window.
				t.lastSpeedAt = now
				t.lastSpeedBytes = current
			case dt >= minWindow && delta > 0:
				inst := float64(delta) / dt.Seconds()
				if t.speedBps == 0 {
					t.speedBps = inst
				} else {
					t.speedBps = alpha*inst + (1-alpha)*t.speedBps
				}
				t.lastSpeedAt = now
				t.lastSpeedBytes = current
			}
		}
	}

	ui.markDirtyLocked()
}

// Done marks the task as successfully completed.
func (t *Task) Done() {
	if t == nil || t.g == nil || t.g.ui == nil {
		return
	}
	ui := t.g.ui

	ui.mu.Lock()
	if ui.closed || t.status != taskStatusRunning {
		ui.mu.Unlock()
		return
	}

	now := time.Now()
	t.status = taskStatusDone
	t.endAt = now

	// Best-effort: if the download completed before we had enough samples to
	// estimate a stable speed, fall back to the overall average.
	if t.kind == taskKindDownload && t.speedBps <= 0 && !t.startAt.IsZero() && now.After(t.startAt) {
		elapsed := now.Sub(t.startAt).Seconds()
		if elapsed > 0 {
			size := t.total
			if size <= 0 {
				size = t.current
			}
			if size > 0 {
				t.speedBps = float64(size) / elapsed
			}
		}
	}

	if ui.mode == ModePlain {
		ui.printPlainTaskDoneLocked(t)
	}

	history := ui.maybeSealGroupLocked(t.g)
	if len(history) == 0 {
		ui.markDirtyLocked()
	}
	ui.mu.Unlock()

	ui.printHistoryLines(history)
}

// Error marks the task as failed with a message.
func (t *Task) Error(msg string) {
	if t == nil || t.g == nil || t.g.ui == nil {
		return
	}
	ui := t.g.ui

	ui.mu.Lock()
	if ui.closed {
		ui.mu.Unlock()
		return
	}
	// Once a group is sealed into the immutable history area (TTY mode), avoid
	// mutating tasks so previously rendered output remains stable.
	if t.g.sealed {
		ui.mu.Unlock()
		return
	}

	if t.status == taskStatusDone || t.status == taskStatusSkipped || t.status == taskStatusCanceled {
		ui.mu.Unlock()
		return
	}

	// In plain mode, avoid emitting duplicate error logs when multiple goroutines
	// race to report the same failure.
	if ui.mode == ModePlain && t.status == taskStatusError {
		ui.mu.Unlock()
		return
	}

	now := time.Now()
	t.status = taskStatusError
	t.message = msg
	t.endAt = now
	if t.startAt.IsZero() {
		t.startAt = now
	}

	if ui.mode == ModePlain {
		ui.printPlainTaskErrorLocked(t)
	}

	history := ui.maybeSealGroupLocked(t.g)
	if len(history) == 0 {
		ui.markDirtyLocked()
	}
	ui.mu.Unlock()

	ui.printHistoryLines(history)
}

// Skip marks the task as skipped with an optional reason.
//
// A skipped task is not considered an error (e.g. when a step is not applicable
// due to unmet prerequisites).
func (t *Task) Skip(reason string) {
	if t == nil || t.g == nil || t.g.ui == nil {
		return
	}
	ui := t.g.ui

	ui.mu.Lock()
	if ui.closed || t.status != taskStatusRunning {
		ui.mu.Unlock()
		return
	}

	now := time.Now()
	t.status = taskStatusSkipped
	t.message = reason
	t.endAt = now

	if ui.mode == ModePlain {
		ui.printPlainTaskSkippedLocked(t)
	}

	history := ui.maybeSealGroupLocked(t.g)
	if len(history) == 0 {
		ui.markDirtyLocked()
	}
	ui.mu.Unlock()

	ui.printHistoryLines(history)
}

// Cancel marks the task as canceled with an optional reason.
//
// Cancel is typically used when the user interrupts execution (Ctrl+C).
func (t *Task) Cancel(reason string) {
	if t == nil || t.g == nil || t.g.ui == nil {
		return
	}
	ui := t.g.ui

	ui.mu.Lock()
	if ui.closed || t.status != taskStatusRunning {
		ui.mu.Unlock()
		return
	}

	now := time.Now()
	t.status = taskStatusCanceled
	t.message = reason
	t.endAt = now

	if ui.mode == ModePlain {
		ui.printPlainTaskCanceledLocked(t)
	}

	history := ui.maybeSealGroupLocked(t.g)
	if len(history) == 0 {
		ui.markDirtyLocked()
	}
	ui.mu.Unlock()

	ui.printHistoryLines(history)
}

func (t *Task) setKind(kind taskKind) {
	if t == nil || t.g == nil || t.g.ui == nil {
		return
	}
	ui := t.g.ui

	ui.mu.Lock()
	defer ui.mu.Unlock()

	if ui.closed {
		return
	}
	t.kind = kind
	if ui.mode == ModePlain && kind == taskKindDownload && t.status == taskStatusRunning && !t.downloadStartPrinted {
		t.downloadStartPrinted = true
		ui.printPlainDownloadStartLocked(t)
	}
	ui.markDirtyLocked()
}
