package proc

import (
	"context"
	"path/filepath"
	"slices"
	"testing"

	"github.com/pingcap/tiup/pkg/utils"
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
			PDAddrs:       []string{"127.0.0.1:2379"},
			EnableBinlog:  true,
			TiKVWorkerURL: "",
		},
		TiProxyCertDir: dir,
	}

	if err := inst.Prepare(context.Background()); err != nil {
		t.Fatalf("Prepare: %v", err)
	}
	cmd := inst.Info().Proc.Cmd()
	if cmd == nil {
		t.Fatalf("missing cmd")
	}

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
	if !slices.Equal(cmd.Args, want) {
		t.Fatalf("unexpected args:\n  got:  %v\n  want: %v", cmd.Args, want)
	}
}
