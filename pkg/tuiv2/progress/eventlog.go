package progress

import (
	"encoding/json"
	"io"
	"time"
)

type eventLogSink struct {
	enc *json.Encoder

	lastProgressAt map[uint64]time.Time
	pendingCurrent map[uint64]Event
}

func newEventLogSink(w io.Writer) *eventLogSink {
	if w == nil {
		return nil
	}
	return &eventLogSink{
		enc:            json.NewEncoder(w),
		lastProgressAt: make(map[uint64]time.Time),
		pendingCurrent: make(map[uint64]Event),
	}
}

func (s *eventLogSink) write(now time.Time, e Event) {
	if s == nil || s.enc == nil {
		return
	}
	if e.V == 0 {
		e.V = EventVersion
	}
	if e.At.IsZero() {
		e.At = now
	}

	// Throttle frequent progress updates for daemon mode.
	//
	// Keep the last suppressed Current-only update and flush it when the task
	// finishes, so replay can still display a correct final size when Total is
	// unknown.
	if e.Type == EventTaskProgress && e.TaskID != 0 && e.Total == nil {
		last := s.lastProgressAt[e.TaskID]
		if !last.IsZero() && now.Sub(last) < time.Second {
			s.pendingCurrent[e.TaskID] = e
			return
		}
		s.lastProgressAt[e.TaskID] = now
		delete(s.pendingCurrent, e.TaskID)
		_ = s.enc.Encode(e)
		return
	}

	if e.Type == EventTaskState && e.TaskID != 0 && e.Status != nil && taskStatusTerminal(*e.Status) {
		if pe, ok := s.pendingCurrent[e.TaskID]; ok {
			delete(s.pendingCurrent, e.TaskID)
			s.lastProgressAt[e.TaskID] = now
			_ = s.enc.Encode(pe)
		}
	}

	_ = s.enc.Encode(e)
}

func taskStatusTerminal(s TaskStatus) bool {
	switch s {
	case TaskStatusDone, TaskStatusError, TaskStatusSkipped, TaskStatusCanceled:
		return true
	default:
		return false
	}
}
