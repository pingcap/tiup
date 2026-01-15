package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/pingcap/errors"
	tuiv2output "github.com/pingcap/tiup/pkg/tuiv2/output"
	progressv2 "github.com/pingcap/tiup/pkg/tuiv2/progress"
	"github.com/pingcap/tiup/pkg/utils"
	"github.com/spf13/cobra"
)

type playgroundInstanceSummary struct {
	tag      string
	version  string
	tidb     int
	tikv     int
	tiflash  int
	status   string
	port     int
	started  time.Time
	hasStart bool
}

func newPS(state *cliState) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "ps",
		Short: "List running playground-ng instances",
		RunE: func(cmd *cobra.Command, args []string) error {
			return ps(cmd.OutOrStdout(), state)
		},
	}
	return cmd
}

func newStopAll(state *cliState) *cobra.Command {
	var timeoutSec int
	cmd := &cobra.Command{
		Use:   "stop-all",
		Short: "Stop all running playground-ng instances",
		RunE: func(cmd *cobra.Command, args []string) error {
			if timeoutSec <= 0 {
				timeoutSec = 60
			}
			return stopAll(cmd.OutOrStdout(), time.Duration(timeoutSec)*time.Second, state)
		},
	}
	cmd.Flags().IntVar(&timeoutSec, "timeout", 60, "Max wait time in seconds for stopping each instance")
	return cmd
}

func ps(out io.Writer, state *cliState) error {
	if out == nil {
		out = io.Discard
	}
	if state == nil {
		return fmt.Errorf("cli state is nil")
	}

	targets, err := psTargets(state)
	if err != nil {
		return err
	}
	if len(targets) == 0 {
		fmt.Fprint(out, tuiv2output.Callout{
			Style:   tuiv2output.CalloutWarning,
			Content: "No running playground-ng instances found.",
		}.Render(out))
		return nil
	}

	summaries := make([]playgroundInstanceSummary, 0, len(targets))
	for _, target := range targets {
		summary, err := inspectPlaygroundInstance(target)
		if err != nil {
			return err
		}
		summaries = append(summaries, summary)
	}

	td := utils.NewTableDisplayer(out, []string{"TAG", "VERSION", "TIDB", "TIKV", "TIFLASH", "STATUS", "PORT", "START TIME"})
	for _, s := range summaries {
		startText := "-"
		if s.hasStart {
			startText = s.started.Format(time.RFC3339)
		}
		td.AddRow(
			s.tag,
			s.version,
			strconv.Itoa(s.tidb),
			strconv.Itoa(s.tikv),
			strconv.Itoa(s.tiflash),
			s.status,
			strconv.Itoa(s.port),
			startText,
		)
	}
	td.Display()
	return nil
}

func stopAll(out io.Writer, timeout time.Duration, state *cliState) error {
	if out == nil {
		out = io.Discard
	}
	if state == nil {
		return fmt.Errorf("cli state is nil")
	}
	if strings.TrimSpace(state.tag) != "" || strings.TrimSpace(state.tiupDataDir) != "" {
		return fmt.Errorf("stop-all does not accept --tag or TIUP_INSTANCE_DATA_DIR; use 'tiup playground-ng stop' instead")
	}

	targets, err := listPlaygroundTargets(state.dataDir)
	if err != nil {
		return err
	}
	if len(targets) == 0 {
		fmt.Fprint(out, tuiv2output.Callout{
			Style:   tuiv2output.CalloutWarning,
			Content: "No running playground-ng instances found.",
		}.Render(out))
		return nil
	}

	return stopAllWithProgressUI(out, targets, timeout)
}

func stopAllWithProgressUI(out io.Writer, targets []playgroundTarget, timeout time.Duration) error {
	if out == nil {
		return fmt.Errorf("output writer is nil")
	}

	ui := progressv2.New(progressv2.Options{
		Mode: progressv2.ModeAuto,
		Out:  out,
	})
	defer ui.Close()

	g := ui.Group("Stop clusters")

	summaries := make([]playgroundInstanceSummary, 0, len(targets))
	for _, target := range targets {
		summary, err := inspectPlaygroundInstance(target)
		if err != nil {
			summary = playgroundInstanceSummary{tag: target.tag, port: target.port, version: "-"}
		}
		summaries = append(summaries, summary)
	}

	tasks := make([]*progressv2.Task, 0, len(targets))
	for i, target := range targets {
		t := g.TaskPending(target.tag)
		if i < len(summaries) {
			if v := strings.TrimSpace(summaries[i].version); v != "" && v != "-" {
				t.SetMeta(fmt.Sprintf("(%s)", v))
			}
		}
		tasks = append(tasks, t)
	}

	type stopResult struct {
		index int
		err   error
	}

	results := make(chan stopResult, len(targets))
	for i, target := range targets {
		target := target
		t := (*progressv2.Task)(nil)
		if i < len(tasks) {
			t = tasks[i]
		}
		if t != nil {
			t.Start()
		}
		go func(index int) {
			results <- stopResult{index: index, err: stopSinglePlayground(target, timeout)}
		}(i)
	}

	var failed []string
	for range targets {
		r := <-results
		t := (*progressv2.Task)(nil)
		if r.index >= 0 && r.index < len(tasks) {
			t = tasks[r.index]
		}
		if r.err != nil {
			failed = append(failed, fmt.Sprintf("%s(%d): %v", targets[r.index].tag, targets[r.index].port, r.err))
			if t != nil {
				t.Error(r.err.Error())
			}
			continue
		}
		if t != nil {
			t.Done()
		}
	}

	g.Close()

	if len(failed) > 0 {
		return renderedError{err: fmt.Errorf("failed to stop %d instance(s)", len(failed))}
	}
	return nil
}

func psTargets(state *cliState) ([]playgroundTarget, error) {
	if state == nil {
		return nil, fmt.Errorf("cli state is nil")
	}
	if strings.TrimSpace(state.tag) != "" || strings.TrimSpace(state.tiupDataDir) != "" {
		target, err := resolvePlaygroundTarget(state.tag, state.tiupDataDir, state.dataDir)
		if err != nil {
			return nil, err
		}
		return []playgroundTarget{target}, nil
	}

	targets, err := listPlaygroundTargets(state.dataDir)
	if err != nil {
		return nil, err
	}
	return targets, nil
}

func inspectPlaygroundInstance(target playgroundTarget) (playgroundInstanceSummary, error) {
	summary := playgroundInstanceSummary{
		tag:    target.tag,
		port:   target.port,
		status: "running",
	}

	start, hasStart := loadStartTime(target.dir)
	summary.started = start
	summary.hasStart = hasStart

	addr := "127.0.0.1:" + strconv.Itoa(target.port)
	items, err := fetchDisplayJSON(addr)
	if err != nil {
		return playgroundInstanceSummary{}, err
	}

	summary.version = pickClusterVersion(items)

	coreDown := false
	for _, item := range items {
		switch item.ServiceID {
		case "tidb":
			summary.tidb++
			if item.Status != "running" {
				coreDown = true
			}
		case "tikv":
			summary.tikv++
			if item.Status != "running" {
				coreDown = true
			}
		case "tiflash":
			summary.tiflash++
			if item.Status != "running" {
				coreDown = true
			}
		case "pd":
			if item.Status != "running" {
				coreDown = true
			}
		}
	}
	if coreDown {
		summary.status = "degraded"
	}

	return summary, nil
}

func loadStartTime(dataDir string) (time.Time, bool) {
	if strings.TrimSpace(dataDir) == "" {
		return time.Time{}, false
	}
	f, err := readPIDFile(filepath.Join(dataDir, playgroundPIDFileName))
	if err != nil {
		return time.Time{}, false
	}
	if f.startedAt.IsZero() {
		return time.Time{}, false
	}
	return f.startedAt, true
}

func fetchDisplayJSON(addr string) ([]displayItem, error) {
	var buf bytes.Buffer
	cmd := Command{
		Type:    DisplayCommandType,
		Display: &DisplayRequest{Verbose: true, JSON: true},
	}
	if err := sendCommandsAndPrintResult(&buf, []Command{cmd}, addr); err != nil {
		return nil, err
	}
	var items []displayItem
	if err := json.Unmarshal(buf.Bytes(), &items); err != nil {
		return nil, errors.Annotate(err, "decode display JSON")
	}
	return items, nil
}

func pickClusterVersion(items []displayItem) string {
	priority := []string{"tidb", "tikv", "pd", "tiflash"}
	for _, serviceID := range priority {
		for _, item := range items {
			if item.ServiceID == serviceID && strings.TrimSpace(item.Version) != "" {
				return item.Version
			}
		}
	}
	for _, item := range items {
		if strings.TrimSpace(item.Version) != "" {
			return item.Version
		}
	}
	return "-"
}

func stopSinglePlayground(target playgroundTarget, timeout time.Duration) error {
	addr := "127.0.0.1:" + strconv.Itoa(target.port)
	if err := sendCommandsAndPrintResult(io.Discard, []Command{{Type: StopCommandType}}, addr); err != nil {
		return err
	}
	return waitPlaygroundStopped(target.dir, timeout)
}
