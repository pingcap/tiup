package main

import (
	stdErrors "errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

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
