package progress

import (
	"bytes"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestEventLogSink_WritesAllEvents(t *testing.T) {
	var buf bytes.Buffer
	sink := newEventLogSink(&buf)
	require.NotNil(t, sink)

	t0 := time.Unix(1_000_000, 0)
	t1 := t0.Add(200 * time.Millisecond)

	c1 := int64(1)
	c2 := int64(2)
	sink.write(t0, Event{Type: EventTaskProgress, TaskID: 1, Current: &c1})
	sink.write(t1, Event{Type: EventTaskProgress, TaskID: 1, Current: &c2})

	lines := bytes.Split(bytes.TrimSpace(buf.Bytes()), []byte("\n"))
	require.Len(t, lines, 2)

	e1, err := DecodeEvent(lines[0])
	require.NoError(t, err)
	e2, err := DecodeEvent(lines[1])
	require.NoError(t, err)

	require.Equal(t, EventTaskProgress, e1.Type)
	require.NotNil(t, e1.Current)
	require.Equal(t, int64(1), *e1.Current)

	require.Equal(t, EventTaskProgress, e2.Type)
	require.NotNil(t, e2.Current)
	require.Equal(t, int64(2), *e2.Current)
}
