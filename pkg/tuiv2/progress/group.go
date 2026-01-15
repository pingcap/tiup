package progress

// Group groups a set of related tasks (usually one stage).
//
// Group is a lightweight handle: it emits events into the UI engine.
// It is safe to use from any goroutine.
type Group struct {
	ui *UI
	id uint64

	// title is best-effort local cache for debugging only.
	title string
}

// SetTitle updates the group title.
//
// It is safe to call multiple times. It has no effect after the UI is closed.
func (g *Group) SetTitle(title string) {
	if g == nil || g.ui == nil || g.ui.closed.Load() {
		return
	}
	g.title = title
	t := title
	g.ui.emit(Event{
		Type:    EventGroupUpdate,
		At:      g.ui.now(),
		GroupID: g.id,
		Title:   &t,
	})
}

// SetShowMeta configures whether the group header should include elapsed timing
// information (TTY mode only).
func (g *Group) SetShowMeta(show bool) {
	if g == nil || g.ui == nil || g.ui.closed.Load() {
		return
	}
	v := show
	g.ui.emit(Event{
		Type:     EventGroupUpdate,
		At:       g.ui.now(),
		GroupID:  g.id,
		ShowMeta: &v,
	})
}

// SetHideDetailsOnSuccess configures whether per-task details should be hidden
// when the group is closed and all tasks succeed (TTY mode only).
func (g *Group) SetHideDetailsOnSuccess(hide bool) {
	if g == nil || g.ui == nil || g.ui.closed.Load() {
		return
	}
	v := hide
	g.ui.emit(Event{
		Type:                 EventGroupUpdate,
		At:                   g.ui.now(),
		GroupID:              g.id,
		HideDetailsOnSuccess: &v,
	})
}

// SetSortTasksByTitle configures whether tasks should be shown in a stable
// title-sorted order in the TTY Active area.
func (g *Group) SetSortTasksByTitle(sort bool) {
	if g == nil || g.ui == nil || g.ui.closed.Load() {
		return
	}
	v := sort
	g.ui.emit(Event{
		Type:             EventGroupUpdate,
		At:               g.ui.now(),
		GroupID:          g.id,
		SortTasksByTitle: &v,
	})
}

// Task creates a new running task under this group.
func (g *Group) Task(title string) *Task {
	return g.newTask(title, false)
}

// TaskPending creates a new task under this group in a "pending" state.
func (g *Group) TaskPending(title string) *Task {
	return g.newTask(title, true)
}

func (g *Group) newTask(title string, pending bool) *Task {
	if g == nil || g.ui == nil || g.ui.closed.Load() {
		return &Task{title: title}
	}
	tid := g.ui.nextID.Add(1)
	t := &Task{ui: g.ui, id: tid, groupID: g.id, title: title}
	tt := title
	g.ui.emit(Event{
		Type:    EventTaskAdd,
		At:      g.ui.now(),
		GroupID: g.id,
		TaskID:  tid,
		Title:   &tt,
		Pending: pending,
	})
	return t
}

// Close marks the group as closed.
//
// It is safe to call Close multiple times.
func (g *Group) Close() {
	if g == nil || g.ui == nil || g.ui.closed.Load() {
		return
	}
	g.ui.emit(Event{
		Type:    EventGroupClose,
		At:      g.ui.now(),
		GroupID: g.id,
	})
}

// Seal moves the group from the Active area to the immutable History area in
// ModeTTY by printing a snapshot of current state.
//
// Seal is idempotent and has no effect in non-TTY modes.
func (g *Group) Seal() {
	if g == nil || g.ui == nil || g.ui.closed.Load() {
		return
	}
	finished := false
	g.ui.emit(Event{
		Type:     EventGroupClose,
		At:       g.ui.now(),
		GroupID:  g.id,
		Finished: &finished,
	})
}
