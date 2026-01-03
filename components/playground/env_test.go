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

	oldTag, oldDataDir, oldInstanceDir := tag, dataDir, tiupDataDir
	t.Cleanup(func() {
		tag, dataDir, tiupDataDir = oldTag, oldDataDir, oldInstanceDir
	})

	dir := filepath.Join(base, "only")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := dumpPort(filepath.Join(dir, "port"), 12345); err != nil {
		t.Fatalf("dumpPort: %v", err)
	}

	tag = ""
	tiupDataDir = ""
	dataDir = base

	port, err := targetTag()
	if err != nil {
		t.Fatalf("targetTag: %v", err)
	}
	if port != 12345 {
		t.Fatalf("unexpected port: %d", port)
	}
	if tag != "only" {
		t.Fatalf("unexpected tag: %q", tag)
	}
	if dataDir != dir {
		t.Fatalf("unexpected dataDir: %q", dataDir)
	}
}

func TestTargetTag_MultipleRequireExplicitTag(t *testing.T) {
	base := t.TempDir()

	oldTag, oldDataDir, oldInstanceDir := tag, dataDir, tiupDataDir
	t.Cleanup(func() {
		tag, dataDir, tiupDataDir = oldTag, oldDataDir, oldInstanceDir
	})

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

	tag = ""
	tiupDataDir = ""
	dataDir = base

	_, err := targetTag()
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

	oldTag, oldDataDir, oldInstanceDir := tag, dataDir, tiupDataDir
	t.Cleanup(func() {
		tag, dataDir, tiupDataDir = oldTag, oldDataDir, oldInstanceDir
	})

	tag = "missing"
	tiupDataDir = ""
	dataDir = filepath.Join(base, tag)

	_, err := targetTag()
	if err == nil {
		t.Fatalf("expected error")
	}
	var notRunning playgroundNotRunningError
	if !stdErrors.As(err, &notRunning) {
		t.Fatalf("expected playgroundNotRunningError, got %T: %v", err, err)
	}
}
