package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestPS_ListsRunningPlaygrounds(t *testing.T) {
	base := t.TempDir()

	makePlayground := func(tag, version string, tidb, tikv, tiflash int) {
		dir := filepath.Join(base, tag)
		require.NoError(t, os.MkdirAll(dir, 0o755))

		startedAt := time.Date(2026, 1, 13, 0, 0, 0, 0, time.UTC).Format(time.RFC3339)
		pidBody := fmt.Sprintf("pid=%d\nstarted_at=%s\ntag=%s\n", os.Getpid(), startedAt, tag)
		require.NoError(t, os.WriteFile(filepath.Join(dir, playgroundPIDFileName), []byte(pidBody), 0o644))

		var items []displayItem
		for i := 0; i < tidb; i++ {
			items = append(items, displayItem{Name: fmt.Sprintf("tidb-%d", i), ServiceID: "tidb", Status: "running", Version: version})
		}
		for i := 0; i < tikv; i++ {
			items = append(items, displayItem{Name: fmt.Sprintf("tikv-%d", i), ServiceID: "tikv", Status: "running", Version: version})
		}
		for i := 0; i < tiflash; i++ {
			items = append(items, displayItem{Name: fmt.Sprintf("tiflash-%d", i), ServiceID: "tiflash", Status: "running", Version: version})
		}
		items = append(items, displayItem{Name: "pd-0", ServiceID: "pd", Status: "running", Version: version})

		itemsJSON, err := json.Marshal(items)
		require.NoError(t, err)

		s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.URL.Path != "/command" {
				w.WriteHeader(http.StatusNotFound)
				return
			}
			w.Header().Set("Content-Type", "application/json")
			switch r.Method {
			case http.MethodGet:
				w.WriteHeader(http.StatusMethodNotAllowed)
				_ = json.NewEncoder(w).Encode(CommandReply{OK: false, Error: "method not allowed"})
			case http.MethodPost:
				_ = json.NewEncoder(w).Encode(CommandReply{OK: true, Message: string(itemsJSON)})
			default:
				w.WriteHeader(http.StatusMethodNotAllowed)
				_ = json.NewEncoder(w).Encode(CommandReply{OK: false, Error: "method not allowed"})
			}
		}))
		t.Cleanup(s.Close)

		u, err := url.Parse(s.URL)
		require.NoError(t, err)
		port, err := strconv.Atoi(u.Port())
		require.NoError(t, err)
		require.NoError(t, dumpPort(filepath.Join(dir, playgroundPortFileName), port))
	}

	makePlayground("a", "v8.5.4", 1, 1, 0)
	makePlayground("b", "v8.5.4", 2, 1, 1)

	state := &cliState{dataDir: base}
	var buf bytes.Buffer
	require.NoError(t, ps(&buf, state))

	out := buf.String()
	require.Contains(t, out, "TAG")
	require.Contains(t, out, "a")
	require.Contains(t, out, "b")
	require.Contains(t, out, "v8.5.4")
	require.Contains(t, out, "running")
}

func TestPS_NoInstances_PrintsWarning(t *testing.T) {
	state := &cliState{dataDir: t.TempDir()}

	var buf bytes.Buffer
	require.NoError(t, ps(&buf, state))
	require.Contains(t, buf.String(), "No running playground-ng instances found.")
}

func TestPS_NoDataDir_PrintsWarning(t *testing.T) {
	state := &cliState{dataDir: filepath.Join(t.TempDir(), "missing")}

	var buf bytes.Buffer
	require.NoError(t, ps(&buf, state))
	require.Contains(t, buf.String(), "No running playground-ng instances found.")
}

func TestStopAll_StopsAllPlaygrounds(t *testing.T) {
	base := t.TempDir()

	makePlayground := func(tag, version string, tidb, tikv, tiflash int) {
		dir := filepath.Join(base, tag)
		require.NoError(t, os.MkdirAll(dir, 0o755))

		pidPath := filepath.Join(dir, playgroundPIDFileName)
		pidBody := fmt.Sprintf("pid=%d\nstarted_at=%s\ntag=%s\n", os.Getpid(), time.Now().UTC().Format(time.RFC3339), tag)
		require.NoError(t, os.WriteFile(pidPath, []byte(pidBody), 0o644))

		var items []displayItem
		for i := 0; i < tidb; i++ {
			items = append(items, displayItem{Name: fmt.Sprintf("tidb-%d", i), ServiceID: "tidb", Status: "running", Version: version})
		}
		for i := 0; i < tikv; i++ {
			items = append(items, displayItem{Name: fmt.Sprintf("tikv-%d", i), ServiceID: "tikv", Status: "running", Version: version})
		}
		for i := 0; i < tiflash; i++ {
			items = append(items, displayItem{Name: fmt.Sprintf("tiflash-%d", i), ServiceID: "tiflash", Status: "running", Version: version})
		}
		items = append(items, displayItem{Name: "pd-0", ServiceID: "pd", Status: "running", Version: version})

		itemsJSON, err := json.Marshal(items)
		require.NoError(t, err)

		s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.URL.Path != "/command" {
				w.WriteHeader(http.StatusNotFound)
				return
			}
			w.Header().Set("Content-Type", "application/json")
			switch r.Method {
			case http.MethodGet:
				w.WriteHeader(http.StatusMethodNotAllowed)
				_ = json.NewEncoder(w).Encode(CommandReply{OK: false, Error: "method not allowed"})
			case http.MethodPost:
				var cmd Command
				if err := json.NewDecoder(r.Body).Decode(&cmd); err != nil {
					w.WriteHeader(http.StatusBadRequest)
					_ = json.NewEncoder(w).Encode(CommandReply{OK: false, Error: err.Error()})
					return
				}
				switch cmd.Type {
				case StopCommandType:
					_ = json.NewEncoder(w).Encode(CommandReply{OK: true, Message: "Stopping playground...\n"})
					go func() {
						time.Sleep(50 * time.Millisecond)
						_ = os.Remove(pidPath)
						_ = os.Remove(filepath.Join(dir, playgroundPortFileName))
					}()
				case DisplayCommandType:
					_ = json.NewEncoder(w).Encode(CommandReply{OK: true, Message: string(itemsJSON)})
				default:
					w.WriteHeader(http.StatusBadRequest)
					_ = json.NewEncoder(w).Encode(CommandReply{OK: false, Error: "unexpected command"})
				}
			default:
				w.WriteHeader(http.StatusMethodNotAllowed)
				_ = json.NewEncoder(w).Encode(CommandReply{OK: false, Error: "method not allowed"})
			}
		}))
		t.Cleanup(s.Close)

		u, err := url.Parse(s.URL)
		require.NoError(t, err)
		port, err := strconv.Atoi(u.Port())
		require.NoError(t, err)
		require.NoError(t, dumpPort(filepath.Join(dir, playgroundPortFileName), port))
	}

	makePlayground("a", "v8.5.4", 1, 1, 0)
	makePlayground("b", "v8.5.4", 2, 1, 1)

	state := &cliState{dataDir: base}
	var buf bytes.Buffer
	require.NoError(t, stopAll(&buf, time.Second, state))

	_, err := os.Stat(filepath.Join(base, "a", playgroundPIDFileName))
	require.True(t, os.IsNotExist(err))
	_, err = os.Stat(filepath.Join(base, "b", playgroundPIDFileName))
	require.True(t, os.IsNotExist(err))

	out := buf.String()
	require.Contains(t, out, "Stop clusters | a (v8.5.4)")
	require.Contains(t, out, "Stop clusters | b (v8.5.4)")
	require.NotContains(t, out, "TAG")
}

func TestStopAll_StopsAllPlaygroundsInParallel(t *testing.T) {
	base := t.TempDir()
	stopDelay := 500 * time.Millisecond
	stopCalls := make(chan time.Time, 2)

	makePlayground := func(tag string) {
		dir := filepath.Join(base, tag)
		require.NoError(t, os.MkdirAll(dir, 0o755))

		pidPath := filepath.Join(dir, playgroundPIDFileName)
		pidBody := fmt.Sprintf("pid=%d\nstarted_at=%s\ntag=%s\n", os.Getpid(), time.Now().UTC().Format(time.RFC3339), tag)
		require.NoError(t, os.WriteFile(pidPath, []byte(pidBody), 0o644))

		itemsJSON, err := json.Marshal([]displayItem{{Name: "pd-0", ServiceID: "pd", Status: "running", Version: "v8.5.4"}})
		require.NoError(t, err)

		s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.URL.Path != "/command" {
				w.WriteHeader(http.StatusNotFound)
				return
			}
			w.Header().Set("Content-Type", "application/json")
			if r.Method != http.MethodPost {
				w.WriteHeader(http.StatusMethodNotAllowed)
				_ = json.NewEncoder(w).Encode(CommandReply{OK: false, Error: "method not allowed"})
				return
			}

			var cmd Command
			if err := json.NewDecoder(r.Body).Decode(&cmd); err != nil {
				w.WriteHeader(http.StatusBadRequest)
				_ = json.NewEncoder(w).Encode(CommandReply{OK: false, Error: err.Error()})
				return
			}
			switch cmd.Type {
			case StopCommandType:
				stopCalls <- time.Now()
				_ = json.NewEncoder(w).Encode(CommandReply{OK: true, Message: "Stopping playground...\n"})
				go func() {
					time.Sleep(stopDelay)
					_ = os.Remove(pidPath)
					_ = os.Remove(filepath.Join(dir, playgroundPortFileName))
				}()
			case DisplayCommandType:
				_ = json.NewEncoder(w).Encode(CommandReply{OK: true, Message: string(itemsJSON)})
			default:
				w.WriteHeader(http.StatusBadRequest)
				_ = json.NewEncoder(w).Encode(CommandReply{OK: false, Error: "unexpected command"})
			}
		}))
		t.Cleanup(s.Close)

		u, err := url.Parse(s.URL)
		require.NoError(t, err)
		port, err := strconv.Atoi(u.Port())
		require.NoError(t, err)
		require.NoError(t, dumpPort(filepath.Join(dir, playgroundPortFileName), port))
	}

	makePlayground("a")
	makePlayground("b")

	state := &cliState{dataDir: base}
	require.NoError(t, stopAll(io.Discard, 3*time.Second, state))

	times := make([]time.Time, 0, 2)
	for len(times) < 2 {
		select {
		case at := <-stopCalls:
			times = append(times, at)
		case <-time.After(3 * time.Second):
			t.Fatalf("timed out waiting for stop calls, got %d", len(times))
		}
	}

	earliest := times[0]
	latest := times[0]
	if times[1].Before(earliest) {
		earliest = times[1]
	}
	if times[1].After(latest) {
		latest = times[1]
	}
	require.Less(t, latest.Sub(earliest), stopDelay)
}

func TestStopAll_NonTTY_UsesTuiv2PlainOutput(t *testing.T) {
	base := t.TempDir()

	makePlayground := func(tag, version string) {
		dir := filepath.Join(base, tag)
		require.NoError(t, os.MkdirAll(dir, 0o755))

		pidPath := filepath.Join(dir, playgroundPIDFileName)
		pidBody := fmt.Sprintf("pid=%d\nstarted_at=%s\ntag=%s\n", os.Getpid(), time.Now().UTC().Format(time.RFC3339), tag)
		require.NoError(t, os.WriteFile(pidPath, []byte(pidBody), 0o644))

		itemsJSON, err := json.Marshal([]displayItem{{Name: "pd-0", ServiceID: "pd", Status: "running", Version: version}})
		require.NoError(t, err)

		s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.URL.Path != "/command" {
				w.WriteHeader(http.StatusNotFound)
				return
			}
			w.Header().Set("Content-Type", "application/json")
			if r.Method != http.MethodPost {
				w.WriteHeader(http.StatusMethodNotAllowed)
				_ = json.NewEncoder(w).Encode(CommandReply{OK: false, Error: "method not allowed"})
				return
			}

			var cmd Command
			if err := json.NewDecoder(r.Body).Decode(&cmd); err != nil {
				w.WriteHeader(http.StatusBadRequest)
				_ = json.NewEncoder(w).Encode(CommandReply{OK: false, Error: err.Error()})
				return
			}
			switch cmd.Type {
			case StopCommandType:
				_ = json.NewEncoder(w).Encode(CommandReply{OK: true, Message: "Stopping playground...\n"})
				go func() {
					time.Sleep(50 * time.Millisecond)
					_ = os.Remove(pidPath)
					_ = os.Remove(filepath.Join(dir, playgroundPortFileName))
				}()
			case DisplayCommandType:
				_ = json.NewEncoder(w).Encode(CommandReply{OK: true, Message: string(itemsJSON)})
			default:
				w.WriteHeader(http.StatusBadRequest)
				_ = json.NewEncoder(w).Encode(CommandReply{OK: false, Error: "unexpected command"})
			}
		}))
		t.Cleanup(s.Close)

		u, err := url.Parse(s.URL)
		require.NoError(t, err)
		port, err := strconv.Atoi(u.Port())
		require.NoError(t, err)
		require.NoError(t, dumpPort(filepath.Join(dir, playgroundPortFileName), port))
	}

	makePlayground("a", "v8.5.4")
	makePlayground("b", "v8.5.4")

	state := &cliState{dataDir: base}
	var buf bytes.Buffer
	require.NoError(t, stopAll(&buf, time.Second, state))
	out := buf.String()

	require.Contains(t, out, "Stop clusters | a (v8.5.4)")
	require.Contains(t, out, "Stop clusters | b (v8.5.4)")
	require.NotContains(t, out, "TAG")
}

func TestStopAll_RejectsTag(t *testing.T) {
	state := &cliState{dataDir: t.TempDir(), tag: "only"}
	err := stopAll(io.Discard, time.Second, state)
	require.Error(t, err)
	require.Contains(t, err.Error(), "does not accept")
}

func TestStopAll_NoDataDir_PrintsWarning(t *testing.T) {
	state := &cliState{dataDir: filepath.Join(t.TempDir(), "missing")}

	var buf bytes.Buffer
	require.NoError(t, stopAll(&buf, time.Second, state))
	require.Contains(t, buf.String(), "No running playground-ng instances found.")
}
