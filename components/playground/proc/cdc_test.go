package proc

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/pingcap/tiup/pkg/utils"
	"github.com/stretchr/testify/require"
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

	require.NoError(t, inst.Prepare(context.Background()))
	cmd := inst.Info().Proc.Cmd()
	require.NotNil(t, cmd)

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
	require.Equal(t, want, cmd.Args)
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

	require.NoError(t, inst.Prepare(context.Background()))
	cmd := inst.Info().Proc.Cmd()
	require.NotNil(t, cmd)

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
	require.Equal(t, want, cmd.Args)
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

	require.NoError(t, inst.Prepare(context.Background()))
	cmd := inst.Info().Proc.Cmd()
	require.NotNil(t, cmd)

	want := []string{
		"/bin/ticdc",
		"server",
		"--addr=127.0.0.1:8300",
		"--advertise-addr=127.0.0.1:8300",
		"--pd=http://127.0.0.1:2379",
		"--log-file=" + filepath.Join(dir, "ticdc.log"),
	}
	require.Equal(t, want, cmd.Args)
}
