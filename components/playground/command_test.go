package main

import (
	"bytes"
	"encoding/json"
	stdErrors "errors"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
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
	if err == nil {
		t.Fatalf("expected error")
	}
	printDisplayFailureWarning(&buf, err)

	out := buf.String()
	if got := strings.Count(out, "boom"); got != 1 {
		t.Fatalf("expected error text printed once, got %d:\n%s", got, out)
	}
}

func TestScaleInHelp_DoesNotUseBackquotedUsageAsPlaceholder(t *testing.T) {
	cmd := newScaleIn(newCLIState())

	var buf bytes.Buffer
	cmd.SetOut(&buf)
	cmd.SetErr(&buf)

	if err := cmd.Help(); err != nil {
		t.Fatalf("help: %v", err)
	}

	out := buf.String()
	if strings.Contains(out, "--name tiup playground display") {
		t.Fatalf("unexpected --name placeholder derived from usage backquotes:\n%s", out)
	}
	if strings.Contains(out, "--pid tiup playground display --verbose") {
		t.Fatalf("unexpected --pid placeholder derived from usage backquotes:\n%s", out)
	}
}

func TestTargetTag_SingleAutoSelect(t *testing.T) {
	base := t.TempDir()

	dir := filepath.Join(base, "only")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := dumpPort(filepath.Join(dir, "port"), 12345); err != nil {
		t.Fatalf("dumpPort: %v", err)
	}

	target, err := resolvePlaygroundTarget("", "", base)
	if err != nil {
		t.Fatalf("resolvePlaygroundTarget: %v", err)
	}
	if target.port != 12345 {
		t.Fatalf("unexpected port: %d", target.port)
	}
	if target.tag != "only" {
		t.Fatalf("unexpected tag: %q", target.tag)
	}
	if target.dir != dir {
		t.Fatalf("unexpected dir: %q", target.dir)
	}
}

func TestTargetTag_MultipleRequireExplicitTag(t *testing.T) {
	base := t.TempDir()

	if err := os.MkdirAll(filepath.Join(base, "a"), 0o755); err != nil {
		t.Fatalf("mkdir a: %v", err)
	}
	if err := dumpPort(filepath.Join(base, "a", "port"), 1); err != nil {
		t.Fatalf("dumpPort a: %v", err)
	}
	if err := os.MkdirAll(filepath.Join(base, "b"), 0o755); err != nil {
		t.Fatalf("mkdir b: %v", err)
	}
	if err := dumpPort(filepath.Join(base, "b", "port"), 2); err != nil {
		t.Fatalf("dumpPort b: %v", err)
	}

	_, err := resolvePlaygroundTarget("", "", base)
	if err == nil {
		t.Fatalf("expected error")
	}
	if shouldSuggestPlaygroundNotRunning(err) {
		t.Fatalf("should not suggest not running: %v", err)
	}
	if !strings.Contains(err.Error(), "multiple playgrounds found") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestTargetTag_ExplicitMissingTagIsNotRunning(t *testing.T) {
	base := t.TempDir()

	_, err := resolvePlaygroundTarget("missing", "", filepath.Join(base, "missing"))
	if err == nil {
		t.Fatalf("expected error")
	}
	var notRunning playgroundNotRunningError
	if !stdErrors.As(err, &notRunning) {
		t.Fatalf("expected playgroundNotRunningError, got %T: %v", err, err)
	}
}

func TestCommandHandler_MethodNotAllowed(t *testing.T) {
	p := &Playground{}
	r := httptest.NewRequest(http.MethodGet, "/command", nil)
	w := httptest.NewRecorder()

	p.commandHandler(w, r)

	if w.Result().StatusCode != http.StatusMethodNotAllowed {
		t.Fatalf("status=%d", w.Result().StatusCode)
	}
}

func TestCommandHandler_InvalidJSON(t *testing.T) {
	p := &Playground{}
	r := httptest.NewRequest(http.MethodPost, "/command", strings.NewReader("{"))
	r.Header.Set("Content-Type", "application/json")
	r.Header.Set("Accept", "application/json")
	w := httptest.NewRecorder()

	p.commandHandler(w, r)

	if w.Result().StatusCode != http.StatusBadRequest {
		t.Fatalf("status=%d body=%q", w.Result().StatusCode, w.Body.String())
	}
	var reply CommandReply
	if err := json.Unmarshal(w.Body.Bytes(), &reply); err != nil {
		t.Fatalf("unmarshal reply: %v (%q)", err, w.Body.String())
	}
	if reply.OK {
		t.Fatalf("unexpected ok reply: %+v", reply)
	}
	if reply.Error == "" {
		t.Fatalf("missing error reply: %+v", reply)
	}
}

func TestCommandHandler_UnknownFieldsRejected(t *testing.T) {
	p := &Playground{}
	r := httptest.NewRequest(http.MethodPost, "/command", strings.NewReader(`{"type":"display","unknown":1}`))
	r.Header.Set("Content-Type", "application/json")
	r.Header.Set("Accept", "application/json")
	w := httptest.NewRecorder()

	p.commandHandler(w, r)

	if w.Result().StatusCode != http.StatusBadRequest {
		t.Fatalf("status=%d body=%q", w.Result().StatusCode, w.Body.String())
	}
	var reply CommandReply
	if err := json.Unmarshal(w.Body.Bytes(), &reply); err != nil {
		t.Fatalf("unmarshal reply: %v (%q)", err, w.Body.String())
	}
	if reply.Error == "" {
		t.Fatalf("missing error reply: %+v", reply)
	}
}

func TestCommandHandler_TrailingDataRejected(t *testing.T) {
	p := &Playground{}
	body := `{"type":"display"}{"type":"display"}`
	r := httptest.NewRequest(http.MethodPost, "/command", strings.NewReader(body))
	r.Header.Set("Content-Type", "application/json")
	r.Header.Set("Accept", "application/json")
	w := httptest.NewRecorder()

	p.commandHandler(w, r)

	if w.Result().StatusCode != http.StatusBadRequest {
		t.Fatalf("status=%d body=%q", w.Result().StatusCode, w.Body.String())
	}
	var reply CommandReply
	if err := json.Unmarshal(w.Body.Bytes(), &reply); err != nil {
		t.Fatalf("unmarshal reply: %v (%q)", err, w.Body.String())
	}
	if reply.Error != "invalid JSON payload" {
		t.Fatalf("unexpected error: %+v", reply)
	}
}

func TestCommandHandler_MaxBodyBytes(t *testing.T) {
	p := &Playground{}
	tooLarge := bytes.Repeat([]byte{'a'}, 1024*1024+1)
	r := httptest.NewRequest(http.MethodPost, "/command", bytes.NewReader(tooLarge))
	r.Header.Set("Content-Type", "application/json")
	r.Header.Set("Accept", "application/json")
	w := httptest.NewRecorder()

	p.commandHandler(w, r)

	if w.Result().StatusCode != http.StatusBadRequest {
		t.Fatalf("status=%d body=%q", w.Result().StatusCode, w.Body.String())
	}
	var reply CommandReply
	if err := json.Unmarshal(w.Body.Bytes(), &reply); err != nil {
		t.Fatalf("unmarshal reply: %v (%q)", err, w.Body.String())
	}
	if reply.Error == "" {
		t.Fatalf("missing error reply: %+v", reply)
	}
}
