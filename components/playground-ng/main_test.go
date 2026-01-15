package main

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/pingcap/tiup/pkg/localdata"
	progressv2 "github.com/pingcap/tiup/pkg/tuiv2/progress"
	"github.com/stretchr/testify/require"
)

// To build:
// see build_integration_test in Makefile
// To run:
// tiup-playground-ng.test -test.coverprofile={file} __DEVEL--i-heard-you-like-tests
func TestMain(t *testing.T) {
	var (
		args []string
		run  bool
	)

	for _, arg := range os.Args {
		switch {
		case arg == "__DEVEL--i-heard-you-like-tests":
			run = true
		case strings.HasPrefix(arg, "-test"):
		case strings.HasPrefix(arg, "__DEVEL"):
		default:
			args = append(args, arg)
		}
	}
	os.Args = args

	if run {
		main()
	}
}

func TestGetAbsolutePath(t *testing.T) {
	t.Run("Empty", func(t *testing.T) {
		got, err := getAbsolutePath("")
		require.NoError(t, err)
		require.Equal(t, "", got)
	})

	t.Run("Relative", func(t *testing.T) {
		wd, err := os.Getwd()
		require.NoError(t, err)

		got, err := getAbsolutePath("a/b")
		require.NoError(t, err)

		want, err := filepath.Abs(filepath.Join(wd, "a/b"))
		require.NoError(t, err)
		require.Equal(t, want, got)
	})

	t.Run("HomeExpansion", func(t *testing.T) {
		home, err := os.UserHomeDir()
		if err != nil || home == "" {
			t.Skipf("os.UserHomeDir unavailable: %v", err)
		}

		got, err := getAbsolutePath("~/a/b")
		require.NoError(t, err)

		want, err := filepath.Abs(filepath.Join(home, "a/b"))
		require.NoError(t, err)
		require.Equal(t, want, got)
	})
}

func TestLoadPort_TrimsWhitespace(t *testing.T) {
	base := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(base, "port"), []byte(" 12345 \n"), 0o644))

	port, err := loadPort(base)
	require.NoError(t, err)
	require.Equal(t, 12345, port)
}

func TestShouldIgnoreSubcommandInstanceDataDir(t *testing.T) {
	base := t.TempDir()
	dataParent := filepath.Join(base, "data")
	require.NoError(t, os.MkdirAll(dataParent, 0o755))

	t.Run("EmptyInstanceDir", func(t *testing.T) {
		require.False(t, shouldIgnoreSubcommandInstanceDataDir("", dataParent))
	})

	t.Run("OutsideDataParent", func(t *testing.T) {
		dir := filepath.Join(base, "other", "V8CMwY9")
		require.NoError(t, os.MkdirAll(dir, 0o755))
		require.False(t, shouldIgnoreSubcommandInstanceDataDir(dir, dataParent))
	})

	t.Run("NotBase62Tag", func(t *testing.T) {
		dir := filepath.Join(dataParent, "my-cluster")
		require.NoError(t, os.MkdirAll(dir, 0o755))
		require.False(t, shouldIgnoreSubcommandInstanceDataDir(dir, dataParent))
	})

	t.Run("NonEmptyDir", func(t *testing.T) {
		dir := filepath.Join(dataParent, "V8CMwY8")
		require.NoError(t, os.MkdirAll(dir, 0o755))
		require.NoError(t, os.WriteFile(filepath.Join(dir, "port"), []byte("12345"), 0o644))
		require.False(t, shouldIgnoreSubcommandInstanceDataDir(dir, dataParent))
	})

	t.Run("OnlyDSStore", func(t *testing.T) {
		dir := filepath.Join(dataParent, "V8CMwY9")
		require.NoError(t, os.MkdirAll(dir, 0o755))
		require.NoError(t, os.WriteFile(filepath.Join(dir, ".DS_Store"), []byte(""), 0o644))
		require.True(t, shouldIgnoreSubcommandInstanceDataDir(dir, dataParent))
	})
}

func TestShouldDestroyDataAfterExit(t *testing.T) {
	tiupHome := t.TempDir()
	dataParent := filepath.Join(tiupHome, localdata.DataParentDir)
	require.NoError(t, os.MkdirAll(dataParent, 0o755))

	t.Run("RootNoTiupDataDir", func(t *testing.T) {
		state := &cliState{}
		require.True(t, shouldDestroyDataAfterExit(true, state, false, tiupHome))
	})

	t.Run("RootTiupDataDirUnderDataParent", func(t *testing.T) {
		state := &cliState{tiupDataDir: filepath.Join(dataParent, "V8CMwY9")}
		require.True(t, shouldDestroyDataAfterExit(true, state, false, tiupHome))
	})

	t.Run("RootTiupDataDirEqualsDataParent", func(t *testing.T) {
		state := &cliState{tiupDataDir: dataParent}
		require.False(t, shouldDestroyDataAfterExit(true, state, false, tiupHome))
	})

	t.Run("RootTiupDataDirOutsideDataParent", func(t *testing.T) {
		state := &cliState{tiupDataDir: filepath.Join(tiupHome, "custom", "V8CMwY9")}
		require.False(t, shouldDestroyDataAfterExit(true, state, false, tiupHome))
	})

	t.Run("NotRoot", func(t *testing.T) {
		state := &cliState{}
		require.False(t, shouldDestroyDataAfterExit(false, state, false, tiupHome))
	})

	t.Run("DryRun", func(t *testing.T) {
		state := &cliState{dryRun: true}
		require.False(t, shouldDestroyDataAfterExit(true, state, false, tiupHome))
	})

	t.Run("Background", func(t *testing.T) {
		state := &cliState{background: true}
		require.False(t, shouldDestroyDataAfterExit(true, state, false, tiupHome))
	})

	t.Run("RunAsDaemon", func(t *testing.T) {
		state := &cliState{runAsDaemon: true}
		require.False(t, shouldDestroyDataAfterExit(true, state, false, tiupHome))
	})

	t.Run("TagExplicit", func(t *testing.T) {
		state := &cliState{}
		require.False(t, shouldDestroyDataAfterExit(true, state, true, tiupHome))
	})
}

func TestDownloadTitle(t *testing.T) {
	require.Equal(t,
		"tidb-v7.1.0-linux-amd64.tar.gz",
		downloadTitle("https://example.com/dir/tidb-v7.1.0-linux-amd64.tar.gz?foo=bar"),
	)
	require.Equal(t,
		"tidb-v7.1.0-linux-amd64.tar.gz",
		downloadTitle("/tmp/tidb-v7.1.0-linux-amd64.tar.gz"),
	)
	require.Equal(t,
		"tidb-v7.1.0-linux-amd64.tar.gz",
		downloadTitle("tidb-v7.1.0-linux-amd64.tar.gz"),
	)
}

func TestParseComponentVersionFromTarball(t *testing.T) {
	component, version, ok := parseComponentVersionFromTarball("tidb-v7.1.0-linux-amd64.tar.gz")
	require.True(t, ok)
	require.Equal(t, "tidb", component)
	require.Equal(t, "v7.1.0", version)

	component, version, ok = parseComponentVersionFromTarball("ng-monitoring-server-v1.0.0-linux-amd64.tar.gz")
	require.True(t, ok)
	require.Equal(t, "ng-monitoring-server", component)
	require.Equal(t, "v1.0.0", version)

	component, version, ok = parseComponentVersionFromTarball("tikv-v8.0.0-alpha-linux-amd64.tar.gz")
	require.True(t, ok)
	require.Equal(t, "tikv", component)
	require.Equal(t, "v8.0.0-alpha", version)

	component, version, ok = parseComponentVersionFromTarball("tidb-nightly-linux-amd64.tar.gz")
	require.True(t, ok)
	require.Equal(t, "tidb", component)
	require.Equal(t, "nightly", version)

	component, version, ok = parseComponentVersionFromTarball("tidb-linux-amd64.tar.gz")
	require.False(t, ok)
	require.Equal(t, "", component)
	require.Equal(t, "", version)

	component, version, ok = parseComponentVersionFromTarball("tidb-v7.1.0-linux-amd64.zip")
	require.False(t, ok)
	require.Equal(t, "", component)
	require.Equal(t, "", version)
}

func TestDownloadDisplay(t *testing.T) {
	name, version := downloadDisplay("https://example.com/tidb-v7.1.0-linux-amd64.tar.gz")
	require.Equal(t, "TiDB", name)
	require.Equal(t, "v7.1.0", version)

	name, version = downloadDisplay("https://example.com/unknown_component-v1.0.0-linux-amd64.tar.gz")
	require.Equal(t, "Unknown Component", name)
	require.Equal(t, "v1.0.0", version)

	name, version = downloadDisplay("https://example.com/randomfile.bin")
	require.Equal(t, "randomfile.bin", name)
	require.Equal(t, "", version)
}

func TestRepoDownloadProgress_SetCurrent_Throttles(t *testing.T) {
	g := &progressv2.Group{}
	progress := newRepoDownloadProgress(context.Background(), g)

	p, ok := progress.(*repoDownloadProgress)
	require.True(t, ok)

	p.Start("https://example.com/tidb-v7.1.0-linux-amd64.tar.gz", 0)

	base := time.Date(2024, 1, 2, 3, 4, 5, 0, time.UTC)
	now := base
	p.now = func() time.Time { return now }

	p.SetCurrent(0)
	require.Equal(t, base, p.lastUpdateAt)
	require.Equal(t, int64(0), p.lastSize)
	require.Equal(t, int64(0), p.latestSize)

	now = base.Add(10 * time.Millisecond)
	p.SetCurrent(1)
	require.Equal(t, base, p.lastUpdateAt)
	require.Equal(t, int64(0), p.lastSize)
	require.Equal(t, int64(1), p.latestSize)

	now = base.Add(100 * time.Millisecond)
	p.SetCurrent(2)
	require.Equal(t, now, p.lastUpdateAt)
	require.Equal(t, int64(2), p.lastSize)
	require.Equal(t, int64(2), p.latestSize)

	lastUpdateAt := p.lastUpdateAt
	now = lastUpdateAt.Add(10 * time.Millisecond)
	p.SetCurrent(3)
	require.Equal(t, lastUpdateAt, p.lastUpdateAt)
	require.Equal(t, int64(2), p.lastSize)
	require.Equal(t, int64(3), p.latestSize)

	// Size decreases should always be emitted immediately (e.g. restarted download).
	now = lastUpdateAt.Add(20 * time.Millisecond)
	p.SetCurrent(1)
	require.Equal(t, now, p.lastUpdateAt)
	require.Equal(t, int64(1), p.lastSize)
	require.Equal(t, int64(1), p.latestSize)
}

func TestRepoDownloadProgress_Finish_FlushesSuppressedCurrent(t *testing.T) {
	g := &progressv2.Group{}
	progress := newRepoDownloadProgress(context.Background(), g)

	p, ok := progress.(*repoDownloadProgress)
	require.True(t, ok)

	p.Start("https://example.com/tidb-v7.1.0-linux-amd64.tar.gz", 0)

	base := time.Date(2024, 1, 2, 3, 4, 5, 0, time.UTC)
	now := base
	p.now = func() time.Time { return now }

	p.SetCurrent(0)
	require.Equal(t, int64(0), p.lastSize)
	require.Equal(t, int64(0), p.latestSize)

	now = base.Add(10 * time.Millisecond)
	p.SetCurrent(1)
	require.Equal(t, int64(0), p.lastSize)
	require.Equal(t, int64(1), p.latestSize)

	now = base.Add(20 * time.Millisecond)
	p.Finish()
	require.Equal(t, now, p.lastUpdateAt)
	require.Equal(t, int64(1), p.lastSize)
}

func TestRepoDownloadProgress_Start_ReusesExpectedPendingTask(t *testing.T) {
	g := &progressv2.Group{}
	progress := newRepoDownloadProgress(context.Background(), g)

	p, ok := progress.(*repoDownloadProgress)
	require.True(t, ok)

	p.SetExpectedDownloads([]DownloadPlan{
		{ComponentID: "tidb", ResolvedVersion: "v7.1.0"},
	})

	p.mu.Lock()
	expected := p.expected["tidb@v7.1.0"]
	p.mu.Unlock()
	require.NotNil(t, expected)

	p.Start("https://example.com/tidb-v7.1.0-linux-amd64.tar.gz", 123)

	p.mu.Lock()
	got := p.task
	p.mu.Unlock()
	require.Same(t, expected, got)
}

func TestRepoDownloadProgress_Start_UnexpectedDownloadCreatesNewTask(t *testing.T) {
	g := &progressv2.Group{}
	progress := newRepoDownloadProgress(context.Background(), g)

	p, ok := progress.(*repoDownloadProgress)
	require.True(t, ok)

	p.SetExpectedDownloads([]DownloadPlan{
		{ComponentID: "tidb", ResolvedVersion: "v7.1.0"},
	})

	p.mu.Lock()
	expected := p.expected["tidb@v7.1.0"]
	p.mu.Unlock()
	require.NotNil(t, expected)

	p.Start("https://example.com/tikv-v7.1.0-linux-amd64.tar.gz", 123)

	p.mu.Lock()
	got := p.task
	p.mu.Unlock()
	require.NotNil(t, got)
	require.NotSame(t, expected, got)
}

func TestRepoDownloadProgress_Finish_WhenCanceled_MarksCanceled(t *testing.T) {
	f, err := os.CreateTemp("", "tiup-playground-download-progress-*.log")
	require.NoError(t, err)
	defer func() {
		_ = f.Close()
		_ = os.Remove(f.Name())
	}()

	ui := progressv2.New(progressv2.Options{Mode: progressv2.ModePlain, Out: f})
	t.Cleanup(func() { _ = ui.Close() })

	g := ui.Group("Download components")
	ctx, cancel := context.WithCancel(context.Background())
	progress := newRepoDownloadProgress(ctx, g)

	progress.Start("https://example.com/tidb-v7.1.0-linux-amd64.tar.gz", 123)
	cancel()
	progress.Finish()

	require.NoError(t, ui.Close())
	require.NoError(t, f.Close())

	data, err := os.ReadFile(f.Name())
	require.NoError(t, err)
	out := string(data)
	require.Contains(t, out, "CANCEL - TiDB v7.1.0")
	require.NotContains(t, out, "Downloaded  TiDB v7.1.0")
}
