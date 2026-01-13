package proc

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/pingcap/tiup/pkg/utils"
	"github.com/stretchr/testify/require"
)

func TestTiDBInstancePrepare_PropagatesEndpointsAndBinlogFlag(t *testing.T) {
	dir := t.TempDir()
	inst := &TiDBInstance{
		ProcessInfo: ProcessInfo{
			Dir:        dir,
			Host:       "127.0.0.1",
			Port:       4000,
			StatusPort: 10080,
			BinPath:    "/bin/tidb-server",
			Version:    utils.Version(utils.LatestVersionAlias),
			Service:    ServiceTiDB,
		},
		Plan: TiDBPlan{
			PDAddrs:        []string{"127.0.0.1:2379"},
			EnableBinlog:   true,
			TiKVWorkerURLs: []string{},
		},
		TiProxyCertDir: dir,
	}

	require.NoError(t, inst.Prepare(context.Background()))
	cmd := inst.Info().Proc.Cmd()
	require.NotNil(t, cmd)

	want := []string{
		"/bin/tidb-server",
		"-P", "4000",
		"--store=tikv",
		"--host=127.0.0.1",
		"--status=10080",
		"--path=127.0.0.1:2379",
		"--log-file=" + filepath.Join(dir, "tidb.log"),
		"--config=" + filepath.Join(dir, "tidb.toml"),
		"--enable-binlog=true",
	}
	require.Equal(t, want, cmd.Args)
}
