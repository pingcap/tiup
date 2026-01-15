package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	progressv2 "github.com/pingcap/tiup/pkg/tuiv2/progress"
	"github.com/stretchr/testify/require"
)

type blockingWriter struct {
	unblockOnce sync.Once
	unblockCh   chan struct{}
}

func (w *blockingWriter) Unblock() {
	if w == nil {
		return
	}
	w.unblockOnce.Do(func() {
		if w.unblockCh != nil {
			close(w.unblockCh)
		}
	})
}

func (w *blockingWriter) Write(p []byte) (int, error) {
	if w == nil {
		return len(p), nil
	}
	if w.unblockCh != nil {
		<-w.unblockCh
	}
	return len(p), nil
}

func TestSendCommandsAndPrintResult_FailedCommandDoesNotDuplicateErrorOutput(t *testing.T) {
	s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(CommandReply{
			OK:    false,
			Error: "boom",
		})
	}))
	defer s.Close()

	addr := strings.TrimPrefix(s.URL, "http://")

	var buf bytes.Buffer
	err := sendCommandsAndPrintResult(&buf, []Command{{Type: DisplayCommandType}}, addr)
	require.Error(t, err)
	printDisplayFailureWarning(&buf, err)

	out := buf.String()
	got := strings.Count(out, "boom")
	require.Equal(t, 1, got, "output:\n%s", out)
}

func TestTargetTag_SingleAutoSelect(t *testing.T) {
	base := t.TempDir()

	dir := filepath.Join(base, "only")
	require.NoError(t, os.MkdirAll(dir, 0o755))
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
		_ = json.NewEncoder(w).Encode(CommandReply{OK: true})
	}))
	defer s.Close()
	u, err := url.Parse(s.URL)
	require.NoError(t, err)
	port, err := strconv.Atoi(u.Port())
	require.NoError(t, err)
	require.NoError(t, dumpPort(filepath.Join(dir, "port"), port))

	target, err := resolvePlaygroundTarget("", "", base)
	require.NoError(t, err)
	require.Equal(t, port, target.port)
	require.Equal(t, "only", target.tag)
	require.Equal(t, dir, target.dir)
}

func TestTargetTag_MultipleRequireExplicitTag(t *testing.T) {
	base := t.TempDir()

	require.NoError(t, os.MkdirAll(filepath.Join(base, "a"), 0o755))
	s1 := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/command" {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusMethodNotAllowed)
		_ = json.NewEncoder(w).Encode(CommandReply{OK: false, Error: "method not allowed"})
	}))
	defer s1.Close()
	u1, err := url.Parse(s1.URL)
	require.NoError(t, err)
	p1, err := strconv.Atoi(u1.Port())
	require.NoError(t, err)
	require.NoError(t, dumpPort(filepath.Join(base, "a", "port"), p1))

	require.NoError(t, os.MkdirAll(filepath.Join(base, "b"), 0o755))
	s2 := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/command" {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusMethodNotAllowed)
		_ = json.NewEncoder(w).Encode(CommandReply{OK: false, Error: "method not allowed"})
	}))
	defer s2.Close()
	u2, err := url.Parse(s2.URL)
	require.NoError(t, err)
	p2, err := strconv.Atoi(u2.Port())
	require.NoError(t, err)
	require.NoError(t, dumpPort(filepath.Join(base, "b", "port"), p2))

	_, err = resolvePlaygroundTarget("", "", base)
	require.Error(t, err)
	require.False(t, shouldSuggestPlaygroundNotRunning(err))
	require.Contains(t, err.Error(), "multiple playgrounds found")
}

func TestTargetTag_StalePortIsFiltered(t *testing.T) {
	base := t.TempDir()

	require.NoError(t, os.MkdirAll(filepath.Join(base, "stale"), 0o755))
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	stalePort := ln.Addr().(*net.TCPAddr).Port
	require.NoError(t, ln.Close())
	require.NoError(t, dumpPort(filepath.Join(base, "stale", "port"), stalePort))

	require.NoError(t, os.MkdirAll(filepath.Join(base, "good"), 0o755))
	s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/command" {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusMethodNotAllowed)
		_ = json.NewEncoder(w).Encode(CommandReply{OK: false, Error: "method not allowed"})
	}))
	defer s.Close()
	u, err := url.Parse(s.URL)
	require.NoError(t, err)
	port, err := strconv.Atoi(u.Port())
	require.NoError(t, err)
	require.NoError(t, dumpPort(filepath.Join(base, "good", "port"), port))

	target, err := resolvePlaygroundTarget("", "", base)
	require.NoError(t, err)
	require.Equal(t, "good", target.tag)
	require.Equal(t, port, target.port)
}

func TestTargetTag_MissingBaseDirIsNotRunning(t *testing.T) {
	base := filepath.Join(t.TempDir(), "missing")

	_, err := resolvePlaygroundTarget("", "", base)
	require.Error(t, err)
	var notRunning playgroundNotRunningError
	require.ErrorAs(t, err, &notRunning)
	require.True(t, shouldSuggestPlaygroundNotRunning(err))
}

func TestTargetTag_ExplicitProbeTimeoutIsUnreachable(t *testing.T) {
	base := t.TempDir()

	dir := filepath.Join(base, "slow")
	require.NoError(t, os.MkdirAll(dir, 0o755))

	s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/command" {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		time.Sleep(time.Second)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusMethodNotAllowed)
		_ = json.NewEncoder(w).Encode(CommandReply{OK: false, Error: "method not allowed"})
	}))
	defer s.Close()
	u, err := url.Parse(s.URL)
	require.NoError(t, err)
	port, err := strconv.Atoi(u.Port())
	require.NoError(t, err)
	require.NoError(t, dumpPort(filepath.Join(dir, "port"), port))

	_, err = resolvePlaygroundTarget("slow", "", dir)
	require.Error(t, err)
	var unreachable playgroundUnreachableError
	require.ErrorAs(t, err, &unreachable)
	require.False(t, shouldSuggestPlaygroundNotRunning(err))
	require.Contains(t, err.Error(), "timed out")
}

func TestTargetTag_ExplicitProbeUnexpectedResponseIsUnreachable(t *testing.T) {
	base := t.TempDir()

	dir := filepath.Join(base, "invalid")
	require.NoError(t, os.MkdirAll(dir, 0o755))

	s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/command" {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ok"))
	}))
	defer s.Close()
	u, err := url.Parse(s.URL)
	require.NoError(t, err)
	port, err := strconv.Atoi(u.Port())
	require.NoError(t, err)
	require.NoError(t, dumpPort(filepath.Join(dir, "port"), port))

	_, err = resolvePlaygroundTarget("invalid", "", dir)
	require.Error(t, err)
	var unreachable playgroundUnreachableError
	require.ErrorAs(t, err, &unreachable)
	require.False(t, shouldSuggestPlaygroundNotRunning(err))
	require.Contains(t, err.Error(), "probe playground")
}

func TestTargetTag_ExplicitConnRefusedIsNotRunning(t *testing.T) {
	base := t.TempDir()

	dir := filepath.Join(base, "refused")
	require.NoError(t, os.MkdirAll(dir, 0o755))

	ln, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	port := ln.Addr().(*net.TCPAddr).Port
	require.NoError(t, ln.Close())
	require.NoError(t, dumpPort(filepath.Join(dir, "port"), port))

	_, err = resolvePlaygroundTarget("refused", "", dir)
	require.Error(t, err)
	var notRunning playgroundNotRunningError
	require.ErrorAs(t, err, &notRunning)
	require.True(t, shouldSuggestPlaygroundNotRunning(err))
}

func TestTargetTag_ExplicitMissingTagIsNotRunning(t *testing.T) {
	base := t.TempDir()

	_, err := resolvePlaygroundTarget("missing", "", filepath.Join(base, "missing"))
	require.Error(t, err)
	var notRunning playgroundNotRunningError
	require.ErrorAs(t, err, &notRunning)
}

func TestCommandHandler_MethodNotAllowed(t *testing.T) {
	p := &Playground{}
	r := httptest.NewRequest(http.MethodGet, "/command", nil)
	w := httptest.NewRecorder()

	p.commandHandler(w, r)

	require.Equal(t, http.StatusMethodNotAllowed, w.Result().StatusCode)
}

func TestCommandHandler_InvalidJSON(t *testing.T) {
	p := &Playground{}
	r := httptest.NewRequest(http.MethodPost, "/command", strings.NewReader("{"))
	r.Header.Set("Content-Type", "application/json")
	r.Header.Set("Accept", "application/json")
	w := httptest.NewRecorder()

	p.commandHandler(w, r)

	require.Equal(t, http.StatusBadRequest, w.Result().StatusCode, "body=%q", w.Body.String())
	var reply CommandReply
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &reply), "body=%q", w.Body.String())
	require.False(t, reply.OK)
	require.NotEmpty(t, reply.Error)
}

func TestCommandHandler_UnknownFieldsRejected(t *testing.T) {
	p := &Playground{}
	r := httptest.NewRequest(http.MethodPost, "/command", strings.NewReader(`{"type":"display","unknown":1}`))
	r.Header.Set("Content-Type", "application/json")
	r.Header.Set("Accept", "application/json")
	w := httptest.NewRecorder()

	p.commandHandler(w, r)

	require.Equal(t, http.StatusBadRequest, w.Result().StatusCode, "body=%q", w.Body.String())
	var reply CommandReply
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &reply), "body=%q", w.Body.String())
	require.NotEmpty(t, reply.Error)
}

func TestCommandHandler_TrailingDataRejected(t *testing.T) {
	p := &Playground{}
	body := `{"type":"display"}{"type":"display"}`
	r := httptest.NewRequest(http.MethodPost, "/command", strings.NewReader(body))
	r.Header.Set("Content-Type", "application/json")
	r.Header.Set("Accept", "application/json")
	w := httptest.NewRecorder()

	p.commandHandler(w, r)

	require.Equal(t, http.StatusBadRequest, w.Result().StatusCode, "body=%q", w.Body.String())
	var reply CommandReply
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &reply), "body=%q", w.Body.String())
	require.Equal(t, "invalid JSON payload", reply.Error)
}

func TestCommandHandler_MaxBodyBytes(t *testing.T) {
	p := &Playground{}
	tooLarge := bytes.Repeat([]byte{'a'}, 1024*1024+1)
	r := httptest.NewRequest(http.MethodPost, "/command", bytes.NewReader(tooLarge))
	r.Header.Set("Content-Type", "application/json")
	r.Header.Set("Accept", "application/json")
	w := httptest.NewRecorder()

	p.commandHandler(w, r)

	require.Equal(t, http.StatusBadRequest, w.Result().StatusCode, "body=%q", w.Body.String())
	var reply CommandReply
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &reply), "body=%q", w.Body.String())
	require.NotEmpty(t, reply.Error)
}

func TestListenAndServeHTTP_StopsAfterProcessGroupClose(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	port := ln.Addr().(*net.TCPAddr).Port
	require.NoError(t, ln.Close())

	dataDir := t.TempDir()
	p := &Playground{
		dataDir:      dataDir,
		port:         port,
		processGroup: NewProcessGroup(),
	}
	require.NoError(t, p.processGroup.Add("command server", p.listenAndServeHTTP))

	url := fmt.Sprintf("http://127.0.0.1:%d/command", port)
	deadline := time.Now().Add(time.Second)
	for {
		resp, err := http.Get(url)
		if err == nil {
			_ = resp.Body.Close()
			require.Equal(t, http.StatusMethodNotAllowed, resp.StatusCode)
			break
		}
		if time.Now().After(deadline) {
			require.FailNow(t, "timeout waiting for command server to be ready", "lastErr=%v", err)
		}
		time.Sleep(10 * time.Millisecond)
	}

	require.FileExists(t, filepath.Join(dataDir, playgroundPortFileName))

	doneCh := make(chan error, 1)
	go func() { doneCh <- p.processGroup.Wait() }()
	p.processGroup.Close()

	select {
	case err := <-doneCh:
		require.NoError(t, err)
	case <-time.After(time.Second):
		require.FailNow(t, "timeout waiting for process group to stop")
	}

	_, err = os.Stat(filepath.Join(dataDir, playgroundPortFileName))
	require.True(t, os.IsNotExist(err))
}

func TestListenAndServeHTTP_FlushesProgressBeforeWritingPortFile(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	port := ln.Addr().(*net.TCPAddr).Port
	require.NoError(t, ln.Close())

	dataDir := t.TempDir()

	outFile, err := os.CreateTemp(t.TempDir(), "ui-out")
	require.NoError(t, err)
	t.Cleanup(func() { _ = outFile.Close() })

	bw := &blockingWriter{unblockCh: make(chan struct{})}
	t.Cleanup(bw.Unblock)

	ui := progressv2.New(progressv2.Options{
		Mode:     progressv2.ModePlain,
		Out:      outFile,
		EventLog: bw,
	})
	t.Cleanup(func() { _ = ui.Close() })

	_, _ = io.WriteString(ui.Writer(), "before server\n")

	p := NewPlayground(dataDir, port)
	p.ui = ui

	errCh := make(chan error, 1)
	go func() { errCh <- p.listenAndServeHTTP() }()

	portPath := filepath.Join(dataDir, playgroundPortFileName)
	deadline := time.Now().Add(200 * time.Millisecond)
	for time.Now().Before(deadline) {
		_, err := os.Stat(portPath)
		if err == nil {
			require.FailNow(t, "port file created before progress flush")
		}
		require.True(t, os.IsNotExist(err), "stat=%v", err)
		time.Sleep(10 * time.Millisecond)
	}

	bw.Unblock()

	require.Eventually(t, func() bool {
		_, err := os.Stat(portPath)
		return err == nil
	}, time.Second, 10*time.Millisecond)

	p.processGroup.Close()
	select {
	case err := <-errCh:
		require.NoError(t, err)
	case <-time.After(time.Second):
		require.FailNow(t, "timeout waiting for command server to stop")
	}
}

func TestStop_WaitsForPIDFileRemoval(t *testing.T) {
	base := t.TempDir()
	dir := filepath.Join(base, "only")
	require.NoError(t, os.MkdirAll(dir, 0o755))

	pidPath := filepath.Join(dir, playgroundPIDFileName)
	require.NoError(t, os.WriteFile(pidPath, []byte("pid="+strconv.Itoa(os.Getpid())+"\n"), 0o644))

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
		if cmd.Type != StopCommandType {
			w.WriteHeader(http.StatusBadRequest)
			_ = json.NewEncoder(w).Encode(CommandReply{OK: false, Error: "unexpected command"})
			return
		}

		_ = json.NewEncoder(w).Encode(CommandReply{OK: true, Message: "Stopping playground...\n"})
		if f, ok := w.(http.Flusher); ok {
			f.Flush()
		}

		go func() {
			time.Sleep(200 * time.Millisecond)
			_ = os.Remove(pidPath)
			_ = os.Remove(filepath.Join(dir, playgroundPortFileName))
		}()
	}))
	defer s.Close()

	u, err := url.Parse(s.URL)
	require.NoError(t, err)
	port, err := strconv.Atoi(u.Port())
	require.NoError(t, err)
	require.NoError(t, dumpPort(filepath.Join(dir, playgroundPortFileName), port))

	state := &cliState{
		tag:     "only",
		dataDir: dir,
	}
	require.NoError(t, stop(io.Discard, 2*time.Second, state))
	_, err = os.Stat(pidPath)
	require.True(t, os.IsNotExist(err))
}
