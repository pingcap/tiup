package proc

import (
	"context"
	"path/filepath"
	"slices"
	"testing"

	"github.com/pingcap/tiup/pkg/utils"
)

func TestPDInstancePrepare_Microservice_OldVersionOmitsName(t *testing.T) {
	dir := t.TempDir()
	inst := &PDInstance{
		ProcessInfo: ProcessInfo{
			Dir:        dir,
			Host:       "127.0.0.1",
			StatusPort: 1234,
			BinPath:    "/bin/pd-server",
			Version:    utils.Version("v8.2.0"),
			Service:    ServicePDTSO,
		},
		Plan: PDPlan{
			BackendAddrs: []string{"127.0.0.1:2379"},
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
		"/bin/pd-server",
		"services",
		"tso",
		"--listen-addr=http://127.0.0.1:1234",
		"--advertise-listen-addr=http://127.0.0.1:1234",
		"--backend-endpoints=http://127.0.0.1:2379",
		"--log-file=" + inst.LogFile(),
		"--config=" + filepath.Join(dir, "pd-tso.toml"),
	}
	if !slices.Equal(cmd.Args, want) {
		t.Fatalf("unexpected args:\n  got:  %v\n  want: %v", cmd.Args, want)
	}
}

func TestPDInstancePrepare_Microservice_NewVersionAddsName(t *testing.T) {
	dir := t.TempDir()
	inst := &PDInstance{
		ProcessInfo: ProcessInfo{
			Dir:        dir,
			Host:       "127.0.0.1",
			StatusPort: 1234,
			BinPath:    "/bin/pd-server",
			Version:    utils.Version("v8.3.0"),
			Service:    ServicePDTSO,
		},
		Plan: PDPlan{
			BackendAddrs: []string{"127.0.0.1:2379"},
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
		"/bin/pd-server",
		"services",
		"tso",
		"--listen-addr=http://127.0.0.1:1234",
		"--advertise-listen-addr=http://127.0.0.1:1234",
		"--backend-endpoints=http://127.0.0.1:2379",
		"--log-file=" + inst.LogFile(),
		"--config=" + filepath.Join(dir, "pd-tso.toml"),
		"--name=pd-tso-0",
	}
	if !slices.Equal(cmd.Args, want) {
		t.Fatalf("unexpected args:\n  got:  %v\n  want: %v", cmd.Args, want)
	}
}
