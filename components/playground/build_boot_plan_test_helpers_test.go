package main

import (
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/pingcap/errors"
	"github.com/pingcap/tiup/components/playground/proc"
	"github.com/spf13/pflag"
	"github.com/stretchr/testify/require"
)

func newTestFlagSet() *pflag.FlagSet {
	fs := pflag.NewFlagSet("test", pflag.ContinueOnError)
	fs.SetOutput(io.Discard)
	return fs
}

func applyServiceDefaultsForTest(t *testing.T, opts *BootOptions, args ...string) {
	t.Helper()

	fs := newTestFlagSet()
	registerServiceFlags(fs, opts)
	require.NoError(t, fs.Parse(args))
	require.NoError(t, applyServiceDefaults(fs, opts))
}

func buildBootPlanForTest(t *testing.T, opts *BootOptions, src ComponentSource) BootPlan {
	t.Helper()

	plan, err := BuildBootPlan(opts, bootPlannerConfig{
		dataDir:            t.TempDir(),
		portConflictPolicy: PortConflictNone,
		advertiseHost:      func(listen string) string { return listen },
		componentSource:    src,
	})
	require.NoError(t, err)
	return plan
}

type testComponentSource struct {
	t *testing.T

	root string

	installed map[string]bool // key: component@resolved

	// resolvedByConstraint maps "constraint" -> "resolved". When absent, the
	// constraint itself is treated as resolved.
	resolvedByConstraint map[string]string
}

func newTestComponentSource(t *testing.T, resolvedByConstraint map[string]string) *testComponentSource {
	t.Helper()
	if resolvedByConstraint == nil {
		resolvedByConstraint = make(map[string]string)
	}
	return &testComponentSource{
		t:                    t,
		root:                 t.TempDir(),
		installed:            make(map[string]bool),
		resolvedByConstraint: resolvedByConstraint,
	}
}

func (s *testComponentSource) ResolveVersion(component, constraint string) (string, error) {
	_ = component

	c := strings.TrimSpace(constraint)
	if c == "" {
		return "", nil
	}
	if resolved := s.resolvedByConstraint[c]; resolved != "" {
		return resolved, nil
	}
	return c, nil
}

func (s *testComponentSource) PlanInstall(serviceID proc.ServiceID, component, resolved string, forcePull bool) (*DownloadPlan, error) {
	if component == "" {
		return nil, errors.New("component is empty")
	}
	if resolved == "" {
		return nil, errors.Errorf("component %s resolved version is empty", component)
	}

	baseBinPath, err := s.BinaryPath(component, resolved)
	return planInstallByResolvedBinaryPath(serviceID, component, resolved, baseBinPath, err, forcePull), nil
}

func (s *testComponentSource) EnsureInstalled(component, resolved string) error {
	_ = component
	_ = resolved
	return nil
}

func (s *testComponentSource) BinaryPath(component, resolved string) (string, error) {
	if component == "" {
		return "", errors.New("component is empty")
	}
	if resolved == "" {
		return "", errors.Errorf("component %s resolved version is empty", component)
	}

	if !s.installed[component+"@"+resolved] {
		return "", errors.New("not installed")
	}

	return filepath.Join(s.root, component, resolved, baseBinaryNameForComponent(component)), nil
}

func (s *testComponentSource) InstallComponent(component, resolved string, extraBinaries ...string) {
	if s == nil || s.t == nil {
		return
	}

	s.t.Helper()

	s.installed[component+"@"+resolved] = true

	dir := filepath.Join(s.root, component, resolved)
	require.NoError(s.t, os.MkdirAll(dir, 0o755))

	base := filepath.Join(dir, baseBinaryNameForComponent(component))
	require.NoError(s.t, os.WriteFile(base, []byte("bin"), 0o755))

	for _, name := range extraBinaries {
		require.NoError(s.t, os.WriteFile(filepath.Join(dir, name), []byte("bin"), 0o755))
	}
}

func baseBinaryNameForComponent(component string) string {
	switch strings.TrimSpace(component) {
	case proc.ComponentPD.String():
		return "pd-server"
	case proc.ComponentTiKV.String():
		return "tikv-server"
	case proc.ComponentTiDB.String():
		return "tidb-server"
	case proc.ComponentTiFlash.String():
		return "tiflash"
	case proc.ComponentPrometheus.String():
		return "prometheus"
	case proc.ComponentGrafana.String():
		return "grafana-server"
	case proc.ComponentTiKVWorker.String():
		return "tikv-worker"
	default:
		// Best effort fallback for tests that don't care about the exact binary name.
		return component
	}
}
