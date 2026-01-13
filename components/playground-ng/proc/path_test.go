package proc

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestResolveSiblingBinary(t *testing.T) {
	root := t.TempDir()
	baseDir := filepath.Join(root, "a", "b", "c")
	require.NoError(t, os.MkdirAll(baseDir, 0o755))

	baseBin := filepath.Join(baseDir, "prometheus")
	require.NoError(t, os.WriteFile(baseBin, []byte("bin"), 0o644))

	want := "ng-monitoring-server"
	// Search should find in parent dirs.
	parentBin := filepath.Join(root, want)
	require.NoError(t, os.WriteFile(parentBin, []byte("bin"), 0o644))
	got, ok := ResolveSiblingBinary(baseBin, want)
	require.True(t, ok)
	require.Equal(t, parentBin, got)

	// If not found, it should return the default sibling path and ok=false.
	require.NoError(t, os.Remove(parentBin))
	got, ok = ResolveSiblingBinary(baseBin, want)
	require.False(t, ok)
	require.Equal(t, filepath.Join(baseDir, want), got)
}
