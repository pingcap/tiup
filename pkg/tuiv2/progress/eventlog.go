package progress

import (
	"encoding/json"
	"io"
	"time"
)

type eventLogSink struct {
	enc *json.Encoder

	lastProgressAt map[uint64]time.Time
}

func newEventLogSink(w io.Writer) *eventLogSink {
	if w == nil {
		return nil
	}
	return &eventLogSink{
		enc:            json.NewEncoder(w),
		lastProgressAt: make(map[uint64]time.Time),
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
	if e.Type == EventTaskProgress && e.Total == nil {
		last := s.lastProgressAt[e.TaskID]
		if !last.IsZero() && now.Sub(last) < time.Second {
			return
		}
		s.lastProgressAt[e.TaskID] = now
	}

	_ = s.enc.Encode(e)
}
