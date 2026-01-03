package proc

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestPrometheusRenderSDFileStableOrder(t *testing.T) {
	dir := t.TempDir()
	inst := &PrometheusInstance{
		ProcessInfo: ProcessInfo{
			Dir:  dir,
			Host: "127.0.0.1",
			Port: 9090,
		},
	}

	sid2targets := map[ServiceID]MetricAddr{
		ServiceID("z-job"): {Targets: []string{"127.0.0.1:2", "127.0.0.1:1", "127.0.0.1:1"}},
		ServiceID("a-job"): {Targets: []string{"127.0.0.1:3"}},
	}
	if err := inst.RenderSDFile(sid2targets); err != nil {
		t.Fatalf("RenderSDFile: %v", err)
	}

	data, err := os.ReadFile(filepath.Join(dir, "targets.json"))
	if err != nil {
		t.Fatalf("read targets.json: %v", err)
	}

	var got []MetricAddr
	if err := json.Unmarshal(data, &got); err != nil {
		t.Fatalf("unmarshal targets.json: %v", err)
	}

	if len(got) != 3 {
		t.Fatalf("unexpected item count: %d (%+v)", len(got), got)
	}

	if got[0].Labels["job"] != "a-job" {
		t.Fatalf("unexpected job[0]: %q", got[0].Labels["job"])
	}
	if got[1].Labels["job"] != "prometheus" {
		t.Fatalf("unexpected job[1]: %q", got[1].Labels["job"])
	}
	if got[2].Labels["job"] != "z-job" {
		t.Fatalf("unexpected job[2]: %q", got[2].Labels["job"])
	}

	wantZ := []string{"127.0.0.1:1", "127.0.0.1:2"}
	if len(got[2].Targets) != len(wantZ) {
		t.Fatalf("unexpected z-job targets: %v", got[2].Targets)
	}
	for i := range wantZ {
		if got[2].Targets[i] != wantZ[i] {
			t.Fatalf("unexpected z-job targets: %v", got[2].Targets)
		}
	}
}
