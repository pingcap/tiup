package proc

import (
	"os"
	"path/filepath"
	"testing"
)

func TestResolveSiblingBinary(t *testing.T) {
	root := t.TempDir()
	baseDir := filepath.Join(root, "a", "b", "c")
	if err := os.MkdirAll(baseDir, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}

	baseBin := filepath.Join(baseDir, "prometheus")
	if err := os.WriteFile(baseBin, []byte("bin"), 0o644); err != nil {
		t.Fatalf("write base: %v", err)
	}

	want := "ng-monitoring-server"
	// Search should find in parent dirs.
	parentBin := filepath.Join(root, want)
	if err := os.WriteFile(parentBin, []byte("bin"), 0o644); err != nil {
		t.Fatalf("write want: %v", err)
	}
	if got, ok := ResolveSiblingBinary(baseBin, want); !ok || got != parentBin {
		t.Fatalf("unexpected resolve: ok=%v path=%q", ok, got)
	}

	// If not found, it should return the default sibling path and ok=false.
	if err := os.Remove(parentBin); err != nil {
		t.Fatalf("remove want: %v", err)
	}
	if got, ok := ResolveSiblingBinary(baseBin, want); ok || got != filepath.Join(baseDir, want) {
		t.Fatalf("unexpected resolve: ok=%v path=%q", ok, got)
	}
}
