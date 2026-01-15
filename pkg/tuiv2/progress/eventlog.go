package progress

import (
	"encoding/json"
	"io"
	"time"
)

type eventLogSink struct {
	enc *json.Encoder
}

func newEventLogSink(w io.Writer) *eventLogSink {
	if w == nil {
		return nil
	}
	return &eventLogSink{
		enc: json.NewEncoder(w),
	}
}

func (s *eventLogSink) write(now time.Time, e Event) {
	if s == nil || s.enc == nil {
		return
	}
	if e.At.IsZero() {
		e.At = now
	}

	_ = s.enc.Encode(e)
}
