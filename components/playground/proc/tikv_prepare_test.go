package proc

import (
	"context"
	"path/filepath"
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
	if !slices.Equal(cmd.Args, want) {
		t.Fatalf("unexpected args:\n  got:  %v\n  want: %v", cmd.Args, want)
	}
	if !slices.Contains(cmd.Env, "MALLOC_CONF=prof:true,prof_active:false") {
		t.Fatalf("unexpected env: %v", cmd.Env)
	}
}
