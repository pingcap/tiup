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

	sortDir := "--sort-dir=" + filepath.Join(dir, "data") + "/tmp/sorter"
	want := []string{
		"/bin/ticdc",
		"server",
		"--addr=127.0.0.1:8300",
		"--advertise-addr=127.0.0.1:8300",
		"--pd=http://127.0.0.1:2379",
		"--log-file=" + filepath.Join(dir, "ticdc.log"),
		"--config=" + cfgPath,
		sortDir,
	}
	if !slices.Equal(cmd.Args, want) {
		t.Fatalf("unexpected args:\n  got:  %v\n  want: %v", cmd.Args, want)
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

	dataDir := "--data-dir=" + filepath.Join(dir, "data")
	want := []string{
		"/bin/ticdc",
		"server",
		"--addr=127.0.0.1:8300",
		"--advertise-addr=127.0.0.1:8300",
		"--pd=http://127.0.0.1:2379",
		"--log-file=" + filepath.Join(dir, "ticdc.log"),
		"--config=" + cfgPath,
		dataDir,
	}
	if !slices.Equal(cmd.Args, want) {
		t.Fatalf("unexpected args:\n  got:  %v\n  want: %v", cmd.Args, want)
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

	want := []string{
		"/bin/ticdc",
		"server",
		"--addr=127.0.0.1:8300",
		"--advertise-addr=127.0.0.1:8300",
		"--pd=http://127.0.0.1:2379",
		"--log-file=" + filepath.Join(dir, "ticdc.log"),
	}
	if !slices.Equal(cmd.Args, want) {
		t.Fatalf("unexpected args:\n  got:  %v\n  want: %v", cmd.Args, want)
	}
}
