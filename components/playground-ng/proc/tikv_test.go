package proc

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestTiKVInstancePrepare_PropagatesEndpointsAndMALLOCConfEnv(t *testing.T) {
	dir := t.TempDir()
	inst := &TiKVInstance{
		ProcessInfo: ProcessInfo{
			Dir:        dir,
			Host:       "127.0.0.1",
			Port:       20160,
			StatusPort: 20180,
			BinPath:    "/bin/tikv-server",
			Service:    ServiceTiKV,
		},
		Plan: TiKVPlan{
			PDAddrs: []string{"127.0.0.1:2379"},
		},
	}

	require.NoError(t, inst.Prepare(context.Background()))
	cmd := inst.Info().Proc.Cmd()
	require.NotNil(t, cmd)

	want := []string{
		"/bin/tikv-server",
		"--addr=127.0.0.1:20160",
		"--advertise-addr=127.0.0.1:20160",
		"--status-addr=127.0.0.1:20180",
		"--pd-endpoints=http://127.0.0.1:2379",
		"--config=" + filepath.Join(dir, "tikv.toml"),
		"--data-dir=" + filepath.Join(dir, "data"),
		"--log-file=" + filepath.Join(dir, "tikv.log"),
	}
	require.Equal(t, want, cmd.Args)
	require.Contains(t, cmd.Env, "MALLOC_CONF=prof:true,prof_active:false")
}
