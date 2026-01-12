package proc

import (
	"context"
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
	if !slices.Contains(cmd.Args, "--path=127.0.0.1:2379") {
		t.Fatalf("unexpected args: %v", cmd.Args)
	}
	if !slices.Contains(cmd.Args, "--enable-binlog=true") {
		t.Fatalf("unexpected args: %v", cmd.Args)
	}
}
