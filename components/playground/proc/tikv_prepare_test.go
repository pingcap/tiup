package proc

import (
	"context"
	"slices"
	"testing"
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

	if err := inst.Prepare(context.Background()); err != nil {
		t.Fatalf("Prepare: %v", err)
	}
	cmd := inst.Info().Proc.Cmd()
	if cmd == nil {
		t.Fatalf("missing cmd")
	}
	if !slices.Contains(cmd.Args, "--pd-endpoints=http://127.0.0.1:2379") {
		t.Fatalf("unexpected args: %v", cmd.Args)
	}
	if !slices.Contains(cmd.Env, "MALLOC_CONF=prof:true,prof_active:false") {
		t.Fatalf("unexpected env: %v", cmd.Env)
	}
}
