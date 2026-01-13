package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

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
	require.NoError(t, dumpPort(filepath.Join(dir, "port"), 12345))

	target, err := resolvePlaygroundTarget("", "", base)
	require.NoError(t, err)
	require.Equal(t, 12345, target.port)
	require.Equal(t, "only", target.tag)
	require.Equal(t, dir, target.dir)
}

func TestTargetTag_MultipleRequireExplicitTag(t *testing.T) {
	base := t.TempDir()

	require.NoError(t, os.MkdirAll(filepath.Join(base, "a"), 0o755))
	require.NoError(t, dumpPort(filepath.Join(base, "a", "port"), 1))
	require.NoError(t, os.MkdirAll(filepath.Join(base, "b"), 0o755))
	require.NoError(t, dumpPort(filepath.Join(base, "b", "port"), 2))

	_, err := resolvePlaygroundTarget("", "", base)
	require.Error(t, err)
	require.False(t, shouldSuggestPlaygroundNotRunning(err))
	require.Contains(t, err.Error(), "multiple playgrounds found")
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

	p := &Playground{
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

	doneCh := make(chan error, 1)
	go func() { doneCh <- p.processGroup.Wait() }()
	p.processGroup.Close()

	select {
	case err := <-doneCh:
		require.NoError(t, err)
	case <-time.After(time.Second):
		require.FailNow(t, "timeout waiting for process group to stop")
	}
}
