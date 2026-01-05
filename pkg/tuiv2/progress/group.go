package progress

import "time"

// Group groups a set of related tasks (usually one stage).
type Group struct {
	ui    *UI
	title string

	startedAt time.Time
	closedAt  time.Time
	closed    bool
	// sealed indicates the group has been printed to the immutable history area
	// in TTY mode. Once sealed, it will no longer be part of the active redraw
	// region to avoid reordering with normal output.
	sealed bool

	tasks []*Task

	// showMeta controls the TTY renderer: if true, the group header includes
	// the elapsed time.
	// It has no effect in plain mode.
	showMeta bool

	// hideDetailsOnSuccess controls the TTY renderer: if true, once the group is
	// closed and all tasks are successful, the group will be rendered as a single
	// summary line (no per-task lines).
	hideDetailsOnSuccess bool

	plainBegun bool

	// sortTasksByTitle controls the TTY renderer: if true, tasks are displayed in
	// a stable title-sorted order in the Active area.
	//
	// It has no effect in plain mode.
	sortTasksByTitle bool
}

// SetTitle updates the group title.
//
// It is safe to call multiple times. It has no effect after the UI is closed.
func (g *Group) SetTitle(title string) {
	if g == nil || g.ui == nil {
		return
	}
	ui := g.ui

	ui.mu.Lock()
	defer ui.mu.Unlock()

	if ui.closed {
		return
	}
	g.title = title
	ui.markDirtyLocked()
}

// SetShowMeta configures whether the group header should include elapsed timing
// information (TTY mode only).
func (g *Group) SetShowMeta(show bool) {
	if g == nil || g.ui == nil {
		return
	}
	ui := g.ui

	ui.mu.Lock()
	defer ui.mu.Unlock()

	if ui.closed {
		return
	}
	g.showMeta = show
	ui.markDirtyLocked()
}

// SetHideDetailsOnSuccess configures whether per-task details should be hidden
// when the group is closed and all tasks succeed (TTY mode only).
//
// This is useful for "wait" stages where details are only valuable on failures.
// It has no effect in plain mode, where events are emitted as they happen.
func (g *Group) SetHideDetailsOnSuccess(hide bool) {
	if g == nil || g.ui == nil {
		return
	}
	ui := g.ui

	ui.mu.Lock()
	defer ui.mu.Unlock()

	if ui.closed {
		return
	}
	g.hideDetailsOnSuccess = hide
	ui.markDirtyLocked()
}

// Task creates a new running task under this group.
func (g *Group) Task(title string) *Task {
	if g == nil || g.ui == nil {
		return &Task{title: title}
	}

	ui := g.ui
	now := time.Now()

	ui.mu.Lock()
	defer ui.mu.Unlock()

	if ui.closed {
		return &Task{title: title}
	}

	if g.startedAt.IsZero() {
		g.startedAt = now
	}
	if ui.mode == ModePlain && !g.plainBegun {
		g.plainBegun = true
		ui.printPlainLineLocked(ui.plainGroupHeader(g.title))
	}

	t := &Task{
		g:       g,
		title:   title,
		status:  taskStatusRunning,
		startAt: now,
	}
	g.tasks = append(g.tasks, t)
	ui.markDirtyLocked()
	return t
}

// TaskPending creates a new task under this group in a "pending" state.
//
// Pending tasks are visible in TTY mode but do not count as active/running until
// Start() is called. This is useful for stages like shutdown where you want to
// show the full worklist up front without implying everything is in progress.
func (g *Group) TaskPending(title string) *Task {
	if g == nil || g.ui == nil {
		return &Task{title: title}
	}

	ui := g.ui
	now := time.Now()

	ui.mu.Lock()
	defer ui.mu.Unlock()

	if ui.closed {
		return &Task{title: title}
	}

	if g.startedAt.IsZero() {
		g.startedAt = now
	}
	if ui.mode == ModePlain && !g.plainBegun {
		g.plainBegun = true
		ui.printPlainLineLocked(ui.plainGroupHeader(g.title))
	}

	t := &Task{
		g:      g,
		title:  title,
		status: taskStatusPending,
	}
	g.tasks = append(g.tasks, t)
	ui.markDirtyLocked()
	return t
}

// Close marks the group as closed.
//
// It is safe to call Close multiple times.
func (g *Group) Close() {
	if g == nil || g.ui == nil {
		return
	}

	ui := g.ui
	ui.mu.Lock()
	if ui.closed || g.closed {
		ui.mu.Unlock()
		return
	}
	g.closed = true
	g.closedAt = time.Now()

	history := ui.maybeSealGroupLocked(g)
	if len(history) == 0 {
		ui.markDirtyLocked()
	}
	ui.mu.Unlock()

	ui.printHistoryLines(history)
}

// Seal moves the group from the Active area to the immutable History area in
// ModeTTY.
//
// It prints a snapshot of the current group state (including any still-running
// tasks) and then stops rendering the group in the Active area. This is useful
// when abandoning an in-progress stage (e.g. on Ctrl+C) and you want to keep a
// stable record of what had happened so far.
//
// Seal is idempotent and has no effect in non-TTY modes.
func (g *Group) Seal() {
	if g == nil || g.ui == nil {
		return
	}

	ui := g.ui
	ui.mu.Lock()
	if ui.closed {
		ui.mu.Unlock()
		return
	}

	history := ui.sealGroupSnapshotLocked(g)
	if len(history) == 0 {
		ui.markDirtyLocked()
	}
	ui.mu.Unlock()

	ui.printHistoryLines(history)
}

func (g *Group) elapsedLocked() time.Duration {
	if g.startedAt.IsZero() {
		return 0
	}
	if g.closed && !g.closedAt.IsZero() {
		return g.closedAt.Sub(g.startedAt)
	}

	// If there is no running task, freeze the elapsed time at the last task end.
	// This avoids confusing "elapsed keeps growing" output for long-lived groups
	// (e.g. the repository download adapter group) once all tasks are done.
	hasRunning := false
	var lastEnd time.Time
	for _, t := range g.tasks {
		if t.status == taskStatusRunning {
			hasRunning = true
		}
		if !t.endAt.IsZero() && t.endAt.After(lastEnd) {
			lastEnd = t.endAt
		}
	}
	if hasRunning {
		return time.Since(g.startedAt)
	}
	if !lastEnd.IsZero() {
		return lastEnd.Sub(g.startedAt)
	}

	return time.Since(g.startedAt)
}

func (g *Group) countLocked() (ok, err, skipped, canceled int) {
	for _, t := range g.tasks {
		switch t.status {
		case taskStatusDone:
			ok++
		case taskStatusError:
			err++
		case taskStatusSkipped:
			skipped++
		case taskStatusCanceled:
			canceled++
		}
	}
	return ok, err, skipped, canceled
}

// SetSortTasksByTitle configures whether tasks should be shown in a stable
// title-sorted order in the TTY Active area.
//
// This is useful for concurrent downloads where tasks might otherwise appear in
// a non-deterministic order.
func (g *Group) SetSortTasksByTitle(sort bool) {
	if g == nil || g.ui == nil {
		return
	}
	ui := g.ui

	ui.mu.Lock()
	defer ui.mu.Unlock()

	if ui.closed {
		return
	}
	g.sortTasksByTitle = sort
	ui.markDirtyLocked()
}
