package proc

import (
	"context"
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
	if len(cmd.Args) < 3 || cmd.Args[1] != "services" || cmd.Args[2] != "tso" {
		t.Fatalf("unexpected args: %v", cmd.Args)
	}
	if !slices.Contains(cmd.Args, "--backend-endpoints=http://127.0.0.1:2379") {
		t.Fatalf("unexpected args: %v", cmd.Args)
	}
	if slices.Contains(cmd.Args, "--name=pd-tso-0") {
		t.Fatalf("unexpected --name on old version: %v", cmd.Args)
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
	if !slices.Contains(cmd.Args, "--name=pd-tso-0") {
		t.Fatalf("expected --name on new version, got: %v", cmd.Args)
	}
}
