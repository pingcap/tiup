package progress

import (
	"io"
	"strings"
	"testing"
	"time"

	"github.com/charmbracelet/x/ansi"
	"github.com/stretchr/testify/require"
)

func TestEventLogDrivenTTYRender_AutoSealMovesGroupToHistory(t *testing.T) {
	now := time.Unix(1_000_000, 0)

	st := newEngineState()

	apply := func(e Event) {
		st.applyEvent(now, e)
	}

	// Group A with one running task.
	ga := "A"
	ta := "task-a"
	apply(Event{Type: EventGroupAdd, GroupID: 1, Title: &ga})
	apply(Event{Type: EventTaskAdd, GroupID: 1, TaskID: 10, Title: &ta})

	// Group B completes and becomes sealable.
	gb := "B"
	tb := "task-b"
	apply(Event{Type: EventGroupAdd, GroupID: 2, Title: &gb})
	apply(Event{Type: EventTaskAdd, GroupID: 2, TaskID: 20, Title: &tb})
	apply(Event{Type: EventGroupClose, GroupID: 2})
	done := TaskStatusDone
	apply(Event{Type: EventTaskState, TaskID: 20, Status: &done})

	g2 := st.groupByID[2]
	require.NotNil(t, g2)
	require.True(t, g2.canAutoSeal())

	// Simulate the TTY engine behavior: seal finished groups and print snapshot.
	ctx := ttyRenderContext{
		styles:  newTTYStyles(io.Discard),
		width:   200,
		spinner: "",
		now:     now,
	}

	var history []string
	for _, g := range st.groups {
		if g == nil || !g.canAutoSeal() {
			continue
		}
		g.sealed = true
		history = append(history, ttyGroupComponent{group: g}.Lines(ctx, 1_000_000)...)
	}

	active := ansi.Strip(strings.Join(flattenBlocks(renderTTYBlocks(st, ctx, 1_000_000)), "\n"))
	require.Contains(t, active, "A")
	require.NotContains(t, active, "B", "sealed group must be removed from active render")

	hist := ansi.Strip(strings.Join(history, "\n"))
	require.Contains(t, hist, "B")
	require.Contains(t, hist, "task-b")
}

func TestGroupStartedAt_SetOnGroupAdd(t *testing.T) {
	start := time.Unix(1_000_000, 0)

	st := newEngineState()
	title := "Download components"
	st.applyEvent(start, Event{Type: EventGroupAdd, GroupID: 1, Title: &title})

	taskAt := start.Add(5 * time.Second)
	taskTitle := "TiDB"
	st.applyEvent(taskAt, Event{Type: EventTaskAdd, GroupID: 1, TaskID: 10, Title: &taskTitle})

	g := st.groupByID[1]
	require.NotNil(t, g)
	require.Equal(t, start, g.startedAt)
	require.Equal(t, 5*time.Second, g.elapsed(taskAt))
}

func TestTaskCancelFromPending_IsTerminalAndBlocksStart(t *testing.T) {
	now := time.Unix(1_000_000, 0)

	st := newEngineState()
	groupTitle := "Start instances"
	taskTitle := "TiDB"

	st.applyEvent(now, Event{Type: EventGroupAdd, GroupID: 1, Title: &groupTitle})
	st.applyEvent(now, Event{Type: EventTaskAdd, GroupID: 1, TaskID: 10, Title: &taskTitle, Pending: true})

	canceled := TaskStatusCanceled
	st.applyEvent(now, Event{Type: EventTaskState, TaskID: 10, Status: &canceled})

	task := st.taskByID[10]
	require.NotNil(t, task)
	require.Equal(t, taskStatusCanceled, task.status)
	require.False(t, task.startAt.IsZero())
	require.Equal(t, now, task.startAt)
	require.Equal(t, now, task.endAt)

	running := TaskStatusRunning
	st.applyEvent(now.Add(time.Second), Event{Type: EventTaskState, TaskID: 10, Status: &running})
	require.Equal(t, taskStatusCanceled, task.status)
}
