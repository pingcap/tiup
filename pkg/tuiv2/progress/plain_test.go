package progress

import (
	"io"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/pingcap/tiup/pkg/tui/colorstr"
)

func TestPlainOutput_IsStableAndNoANSI(t *testing.T) {
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("pipe: %v", err)
	}
	defer r.Close()
	defer w.Close()

	ui := New(Options{Mode: ModePlain, Out: w})
	defer ui.Close()

	g := ui.Group("Waiting for things")
	t1 := g.Task("task-ok")
	t1.Start()
	t1.Done()
	t2 := g.Task("task-err")
	t2.Start()
	t2.Error("boom")
	g.Close()

	_ = w.Close()
	out, err := io.ReadAll(r)
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	got := string(out)

	if strings.Contains(got, "\033[") {
		t.Fatalf("plain output must not contain ANSI sequences, got: %q", got)
	}
	for _, want := range []string{
		"==> Waiting for things\n",
		"Waiting task-ok\n",
		"Waiting task-err\n",
		"ERR - task-err: boom (",
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("missing %q in output: %q", want, got)
		}
	}
}

func TestDownloadTask_Plain(t *testing.T) {
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("pipe: %v", err)
	}
	defer r.Close()
	defer w.Close()

	ui := New(Options{Mode: ModePlain, Out: w})
	defer ui.Close()

	g := ui.Group("Downloading components")
	t1 := g.Task("TiDB")
	t1.SetTotal(1024 * 1024)
	t1.SetMeta("v7.5.0")
	t1.SetKindDownload()
	t1.SetCurrent(512 * 1024)
	t1.SetCurrent(1024 * 1024)
	t1.Done()
	g.Close()

	_ = w.Close()
	out, err := io.ReadAll(r)
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	got := string(out)

	if strings.Contains(got, "\033[") {
		t.Fatalf("plain output must not contain ANSI sequences, got: %q", got)
	}
	for _, want := range []string{
		"==> Downloading components\n",
		"Downloading TiDB v7.5.0 (1.0MiB)\n",
		"Downloaded  TiDB v7.5.0 (1.0MiB, ",
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("missing %q in output: %q", want, got)
		}
	}
}

func TestPlainOutput_FORCE_COLOR_EmitsANSI(t *testing.T) {
	t.Setenv("FORCE_COLOR", "1")

	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("pipe: %v", err)
	}
	defer r.Close()
	defer w.Close()

	ui := New(Options{Mode: ModePlain, Out: w})
	defer ui.Close()

	g := ui.Group("Waiting for things")
	t1 := g.Task("task-err")
	t1.Start()
	t1.Error("boom")
	g.Close()

	_ = w.Close()
	out, err := io.ReadAll(r)
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	got := string(out)

	if !strings.Contains(got, "\033[") {
		t.Fatalf("expected ANSI sequences with FORCE_COLOR, got: %q", got)
	}

	tokens := colorstr.DefaultTokens
	tokens.Disable = false

	if !strings.Contains(got, tokens.Sprintf("[bold][light_magenta]==> Waiting for things[reset]")) {
		t.Fatalf("expected colored group header, got: %q", got)
	}
	if !strings.Contains(got, tokens.Sprintf("[bold][light_red]ERR[reset]")) {
		t.Fatalf("expected colored ERR label, got: %q", got)
	}
}

func TestGroupElapsed_FreezeWhenAllTasksDone(t *testing.T) {
	start := time.Unix(1_000_000, 0)
	end := start.Add(10 * time.Second)

	g := &Group{startedAt: start}
	g.tasks = []*Task{
		{status: taskStatusDone, startAt: start, endAt: end},
	}

	if got := g.elapsedLocked(); got != 10*time.Second {
		t.Fatalf("unexpected elapsed: %v", got)
	}
}
