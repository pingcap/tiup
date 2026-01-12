package proc

import (
	"context"
	"path/filepath"
	"slices"
	"testing"

	"github.com/pingcap/tiup/pkg/utils"
)

func TestTiCDCPrepare_V4013_UsesSortDir(t *testing.T) {
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "ticdc.toml")

	inst := &TiCDC{
		ProcessInfo: ProcessInfo{
			Dir:        dir,
			Host:       "127.0.0.1",
			Port:       8300,
			BinPath:    "/bin/ticdc",
			Version:    utils.Version("v4.0.13"),
			ConfigPath: cfgPath,
			Service:    ServiceTiCDC,
		},
		Plan: TiCDCPlan{
			PDAddrs: []string{"127.0.0.1:2379"},
		},
	}

	if err := inst.Prepare(context.Background()); err != nil {
		t.Fatalf("Prepare: %v", err)
	}
	cmd := inst.Info().Proc.Cmd()
	if cmd == nil {
		t.Fatalf("missing cmd")
	}

	expectedSortDir := "--sort-dir=" + filepath.Join(dir, "data") + "/tmp/sorter"
	expectedDataDir := "--data-dir=" + filepath.Join(dir, "data")

	if !slices.Contains(cmd.Args, "--pd=http://127.0.0.1:2379") {
		t.Fatalf("unexpected args: %v", cmd.Args)
	}
	if !slices.Contains(cmd.Args, "--config="+cfgPath) {
		t.Fatalf("unexpected args: %v", cmd.Args)
	}
	if !slices.Contains(cmd.Args, expectedSortDir) {
		t.Fatalf("unexpected args: %v", cmd.Args)
	}
	if slices.Contains(cmd.Args, expectedDataDir) {
		t.Fatalf("unexpected args: %v", cmd.Args)
	}
}

func TestTiCDCPrepare_V4014_UsesDataDir(t *testing.T) {
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "ticdc.toml")

	inst := &TiCDC{
		ProcessInfo: ProcessInfo{
			Dir:        dir,
			Host:       "127.0.0.1",
			Port:       8300,
			BinPath:    "/bin/ticdc",
			Version:    utils.Version("v4.0.14"),
			ConfigPath: cfgPath,
			Service:    ServiceTiCDC,
		},
		Plan: TiCDCPlan{
			PDAddrs: []string{"127.0.0.1:2379"},
		},
	}

	if err := inst.Prepare(context.Background()); err != nil {
		t.Fatalf("Prepare: %v", err)
	}
	cmd := inst.Info().Proc.Cmd()
	if cmd == nil {
		t.Fatalf("missing cmd")
	}

	expectedSortDir := "--sort-dir=" + filepath.Join(dir, "data") + "/tmp/sorter"
	expectedDataDir := "--data-dir=" + filepath.Join(dir, "data")

	if !slices.Contains(cmd.Args, "--config="+cfgPath) {
		t.Fatalf("unexpected args: %v", cmd.Args)
	}
	if !slices.Contains(cmd.Args, expectedDataDir) {
		t.Fatalf("unexpected args: %v", cmd.Args)
	}
	if slices.Contains(cmd.Args, expectedSortDir) {
		t.Fatalf("unexpected args: %v", cmd.Args)
	}
}

func TestTiCDCPrepare_OldVersion_OmitsConfigAndDirFlags(t *testing.T) {
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "ticdc.toml")

	inst := &TiCDC{
		ProcessInfo: ProcessInfo{
			Dir:        dir,
			Host:       "127.0.0.1",
			Port:       8300,
			BinPath:    "/bin/ticdc",
			Version:    utils.Version("v4.0.12"),
			ConfigPath: cfgPath,
			Service:    ServiceTiCDC,
		},
		Plan: TiCDCPlan{
			PDAddrs: []string{"127.0.0.1:2379"},
		},
	}

	if err := inst.Prepare(context.Background()); err != nil {
		t.Fatalf("Prepare: %v", err)
	}
	cmd := inst.Info().Proc.Cmd()
	if cmd == nil {
		t.Fatalf("missing cmd")
	}

	if slices.Contains(cmd.Args, "--config="+cfgPath) {
		t.Fatalf("unexpected args: %v", cmd.Args)
	}
	if slices.Contains(cmd.Args, "--data-dir="+filepath.Join(dir, "data")) {
		t.Fatalf("unexpected args: %v", cmd.Args)
	}
	if slices.Contains(cmd.Args, "--sort-dir="+filepath.Join(dir, "data")+"/tmp/sorter") {
		t.Fatalf("unexpected args: %v", cmd.Args)
	}
}
