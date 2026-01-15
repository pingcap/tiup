package progress

import (
	"os"
	"testing"
	"time"

	"github.com/charmbracelet/x/ansi"
	"github.com/stretchr/testify/require"
)

func TestTTYModel_PrintOrder_GroupSnapshotAndPrintLines(t *testing.T) {
	now := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	ui := &UI{
		out: os.Stdout,
		now: func() time.Time { return now },
	}

	m := newTTYModel(ui)

	apply := func(e Event) []string {
		ackCh := make(chan ttyEventAck, 1)
		next, _ := m.Update(ttyEventMsg{Event: e, Ack: ackCh})
		m = next.(ttyModel)
		ack := <-ackCh
		return ack.Prints
	}

	downloadTitle := "Download components"
	startTitle := "Start instances"
	clusterInfoLine := "Cluster info"
	taskTitle := "task"

	done := TaskStatusDone

	var printed []string

	printed = append(printed, apply(Event{Type: EventGroupAdd, At: now, GroupID: 1, Title: &downloadTitle})...)
	printed = append(printed, apply(Event{Type: EventTaskAdd, At: now, GroupID: 1, TaskID: 10, Title: &taskTitle})...)
	printed = append(printed, apply(Event{Type: EventTaskState, At: now.Add(time.Second), TaskID: 10, Status: &done})...)
	printed = append(printed, apply(Event{Type: EventGroupClose, At: now.Add(time.Second), GroupID: 1})...)

	printed = append(printed, apply(Event{Type: EventGroupAdd, At: now.Add(2 * time.Second), GroupID: 2, Title: &startTitle})...)
	printed = append(printed, apply(Event{Type: EventTaskAdd, At: now.Add(2 * time.Second), GroupID: 2, TaskID: 20, Title: &taskTitle})...)
	printed = append(printed, apply(Event{Type: EventTaskState, At: now.Add(3 * time.Second), TaskID: 20, Status: &done})...)
	printed = append(printed, apply(Event{Type: EventGroupClose, At: now.Add(3 * time.Second), GroupID: 2})...)

	printed = append(printed, apply(Event{Type: EventPrintLines, At: now.Add(4 * time.Second), Lines: []string{""}})...)
	printed = append(printed, apply(Event{Type: EventPrintLines, At: now.Add(4 * time.Second), Lines: []string{clusterInfoLine}})...)

	require.GreaterOrEqual(t, len(printed), 4)
	require.Contains(t, ansi.Strip(printed[0]), downloadTitle)
	require.Contains(t, ansi.Strip(printed[1]), startTitle)
	require.Equal(t, "\r"+ansi.EraseLineRight, printed[2])
	require.Equal(t, "\r"+clusterInfoLine+ansi.EraseLineRight, printed[3])
}
