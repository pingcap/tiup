package progress

import "time"

type taskStatus int

const (
	taskStatusPending taskStatus = iota
	taskStatusRunning
	taskStatusRetrying
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

type groupState struct {
	id    uint64
	title string

	startedAt time.Time
	closedAt  time.Time
	closed    bool
	sealed    bool

	tasks []*taskState

	showMeta             bool
	hideDetailsOnSuccess bool
	sortTasksByTitle     bool
}

func (g *groupState) canAutoSeal() bool {
	if g == nil || g.sealed || !g.closed || len(g.tasks) == 0 {
		return false
	}
	for _, t := range g.tasks {
		if t == nil {
			continue
		}
		if t.status == taskStatusRunning || t.status == taskStatusRetrying {
			return false
		}
	}
	return true
}

func (g *groupState) elapsed(now time.Time) time.Duration {
	if g == nil || g.startedAt.IsZero() {
		return 0
	}
	if now.IsZero() {
		now = time.Now()
	}

	hasRunning := false
	var lastEnd time.Time
	for _, t := range g.tasks {
		if t == nil {
			continue
		}
		if t.status == taskStatusRunning || t.status == taskStatusRetrying {
			hasRunning = true
		}
		if !t.endAt.IsZero() && t.endAt.After(lastEnd) {
			lastEnd = t.endAt
		}
	}
	if hasRunning {
		return now.Sub(g.startedAt)
	}

	end := lastEnd
	if g.closed && !g.closedAt.IsZero() && g.closedAt.After(end) {
		end = g.closedAt
	}
	if !end.IsZero() {
		return end.Sub(g.startedAt)
	}
	return now.Sub(g.startedAt)
}

type taskState struct {
	id uint64
	g  *groupState

	title string

	kind   taskKind
	status taskStatus

	hideIfFast  bool
	revealAfter time.Duration

	meta    string
	message string

	current int64
	total   int64

	startAt time.Time
	endAt   time.Time

	speedBps float64

	lastSpeedAt    time.Time
	lastSpeedBytes int64

	plainStartPrinted    bool
	downloadStartPrinted bool
}

type engineState struct {
	groups    []*groupState
	groupByID map[uint64]*groupState
	taskByID  map[uint64]*taskState
}

func newEngineState() *engineState {
	return &engineState{
		groupByID: make(map[uint64]*groupState),
		taskByID:  make(map[uint64]*taskState),
	}
}

func (s *engineState) hasRunning() bool {
	if s == nil {
		return false
	}
	for _, g := range s.groups {
		if g == nil || g.sealed {
			continue
		}
		for _, t := range g.tasks {
			if t == nil {
				continue
			}
			if t.status == taskStatusRunning || t.status == taskStatusRetrying {
				return true
			}
		}
	}
	return false
}

func (s *engineState) applyEvent(now time.Time, e Event) {
	if s == nil {
		return
	}
	if now.IsZero() {
		now = time.Now()
	}

	switch e.Type {
	case EventGroupAdd:
		s.applyGroupAdd(now, e)
	case EventGroupUpdate:
		s.applyGroupUpdate(e)
	case EventGroupClose:
		s.applyGroupClose(now, e)
	case EventTaskAdd:
		s.applyTaskAdd(now, e)
	case EventTaskUpdate:
		s.applyTaskUpdate(e)
	case EventTaskProgress:
		s.applyTaskProgress(now, e)
	case EventTaskState:
		s.applyTaskState(now, e)
	default:
		return
	}
}

func (s *engineState) applyGroupAdd(now time.Time, e Event) {
	id := e.GroupID
	if id == 0 {
		return
	}
	if _, ok := s.groupByID[id]; ok {
		// Idempotent best-effort.
		return
	}
	title := ""
	if e.Title != nil {
		title = *e.Title
	}
	g := &groupState{
		id:        id,
		title:     title,
		showMeta:  true,
		startedAt: now,
	}
	s.groupByID[id] = g
	s.groups = append(s.groups, g)
}

func (s *engineState) applyGroupUpdate(e Event) {
	g := s.groupByID[e.GroupID]
	if g == nil || g.sealed {
		return
	}
	if e.Title != nil {
		g.title = *e.Title
	}
	if e.ShowMeta != nil {
		g.showMeta = *e.ShowMeta
	}
	if e.HideDetailsOnSuccess != nil {
		g.hideDetailsOnSuccess = *e.HideDetailsOnSuccess
	}
	if e.SortTasksByTitle != nil {
		g.sortTasksByTitle = *e.SortTasksByTitle
	}
}

func (s *engineState) applyGroupClose(now time.Time, e Event) {
	g := s.groupByID[e.GroupID]
	if g == nil || g.sealed {
		return
	}

	finished := true
	if e.Finished != nil {
		finished = *e.Finished
	}
	if !finished {
		g.sealed = true
		return
	}

	if g.closed {
		return
	}
	g.closed = true
	g.closedAt = now
}

func (s *engineState) applyTaskAdd(now time.Time, e Event) {
	g := s.groupByID[e.GroupID]
	if g == nil || g.sealed {
		return
	}
	id := e.TaskID
	if id == 0 {
		return
	}
	if _, ok := s.taskByID[id]; ok {
		return
	}
	title := ""
	if e.Title != nil {
		title = *e.Title
	}
	t := &taskState{
		id:     id,
		g:      g,
		title:  title,
		kind:   taskKindGeneric,
		status: taskStatusRunning,
	}
	if e.Pending {
		t.status = taskStatusPending
	} else {
		t.startAt = now
	}
	s.taskByID[id] = t
	g.tasks = append(g.tasks, t)
	if g.startedAt.IsZero() {
		g.startedAt = now
	}
}

func (s *engineState) applyTaskUpdate(e Event) {
	t := s.taskByID[e.TaskID]
	if t == nil || t.g == nil || t.g.sealed {
		return
	}
	if e.Kind != nil {
		switch *e.Kind {
		case TaskKindDownload:
			t.kind = taskKindDownload
		default:
			t.kind = taskKindGeneric
		}
	}
	if e.Meta != nil {
		t.meta = *e.Meta
	}
	if e.Message != nil {
		t.message = *e.Message
	}
	if e.HideIfFast != nil {
		t.hideIfFast = *e.HideIfFast
	}
	if e.RevealAfterMs != nil {
		d := time.Duration(*e.RevealAfterMs) * time.Millisecond
		if d < 0 {
			d = 0
		}
		t.revealAfter = d
	}
}

func (s *engineState) applyTaskProgress(now time.Time, e Event) {
	t := s.taskByID[e.TaskID]
	if t == nil || t.g == nil || t.g.sealed {
		return
	}
	if e.Total != nil && (t.status == taskStatusPending || t.status == taskStatusRunning || t.status == taskStatusRetrying) {
		total := *e.Total
		if total < 0 {
			total = 0
		}
		t.total = total
	}
	if e.Current != nil && (t.status == taskStatusPending || t.status == taskStatusRunning || t.status == taskStatusRetrying) {
		cur := *e.Current
		if cur < 0 {
			cur = 0
		}
		t.current = cur
	}

	if t.status != taskStatusRunning {
		return
	}

	if t.kind != taskKindDownload {
		return
	}

	const (
		minWindow = time.Second
		alpha     = 0.2
	)
	if t.lastSpeedAt.IsZero() || !now.After(t.lastSpeedAt) {
		t.lastSpeedAt = now
		t.lastSpeedBytes = t.current
		return
	}

	dt := now.Sub(t.lastSpeedAt)
	delta := t.current - t.lastSpeedBytes
	switch {
	case delta < 0:
		t.lastSpeedAt = now
		t.lastSpeedBytes = t.current
	case dt >= minWindow && delta > 0:
		inst := float64(delta) / dt.Seconds()
		if t.speedBps == 0 {
			t.speedBps = inst
		} else {
			t.speedBps = alpha*inst + (1-alpha)*t.speedBps
		}
		t.lastSpeedAt = now
		t.lastSpeedBytes = t.current
	}
}

func (t *taskState) ensureStarted(now time.Time) {
	if t == nil {
		return
	}
	if t.startAt.IsZero() {
		t.startAt = now
	}
	if t.g != nil && t.g.startedAt.IsZero() {
		t.g.startedAt = now
	}
}

func (s *engineState) applyTaskState(now time.Time, e Event) {
	t := s.taskByID[e.TaskID]
	if t == nil || t.g == nil || t.g.sealed {
		return
	}
	if e.Status == nil {
		return
	}
	status := *e.Status

	if t.status == taskStatusError && status == TaskStatusError {
		return
	}

	switch status {
	case TaskStatusPending:
		t.status = taskStatusPending
	case TaskStatusRunning:
		switch t.status {
		case taskStatusDone, taskStatusError, taskStatusSkipped, taskStatusCanceled:
			return
		}
		t.status = taskStatusRunning
		t.ensureStarted(now)
	case TaskStatusRetrying:
		switch t.status {
		case taskStatusDone, taskStatusError, taskStatusSkipped, taskStatusCanceled:
			return
		}
		t.status = taskStatusRetrying
		t.ensureStarted(now)
	case TaskStatusDone:
		if t.status != taskStatusRunning && t.status != taskStatusRetrying {
			return
		}
		t.status = taskStatusDone
		t.endAt = now
	case TaskStatusError:
		if t.status == taskStatusDone || t.status == taskStatusSkipped || t.status == taskStatusCanceled {
			return
		}
		t.status = taskStatusError
		t.endAt = now
		t.ensureStarted(now)
	case TaskStatusSkipped:
		switch t.status {
		case taskStatusDone, taskStatusError, taskStatusSkipped, taskStatusCanceled:
			return
		}
		t.status = taskStatusSkipped
		t.endAt = now
		t.ensureStarted(now)
	case TaskStatusCanceled:
		switch t.status {
		case taskStatusDone, taskStatusError, taskStatusSkipped, taskStatusCanceled:
			return
		}
		t.status = taskStatusCanceled
		t.endAt = now
		t.ensureStarted(now)
	default:
		return
	}

	if e.Message != nil {
		t.message = *e.Message
	}

	if t.status != taskStatusDone && t.status != taskStatusError && t.status != taskStatusSkipped && t.status != taskStatusCanceled {
		return
	}
	if t.kind != taskKindDownload || t.speedBps > 0 || t.startAt.IsZero() || !now.After(t.startAt) {
		return
	}

	elapsed := now.Sub(t.startAt).Seconds()
	if elapsed <= 0 {
		return
	}
	size := t.total
	if size <= 0 {
		size = t.current
	}
	if size <= 0 {
		return
	}
	t.speedBps = float64(size) / elapsed
}
