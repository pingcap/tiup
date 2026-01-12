package main

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"

	"github.com/pingcap/tiup/components/playground/proc"
)

func TestWriteDryRun_Text(t *testing.T) {
	plan := BootPlan{
		DataDir:     "/data",
		BootVersion: "nightly",
		Host:        "127.0.0.1",
		Shared:      proc.SharedOptions{Mode: proc.ModeNormal, PDMode: "pd", PortOffset: 123},
		Monitor:     true,
		Downloads: []DownloadPlan{
			{ComponentID: "tidb", ResolvedVersion: "v1.0.0"},
		},
		Services: []ServicePlan{
			{
				Name:               "pd-0",
				ServiceID:          proc.ServicePD.String(),
				ComponentID:        proc.ComponentPD.String(),
				ResolvedVersion:    "v1.0.0",
				StartAfterServices: []string{proc.ServiceTiKV.String()},
				Shared:             ServiceSharedPlan{Dir: "/data/pd-0", Host: "127.0.0.1", Port: 2380, StatusPort: 2379},
			},
		},
	}

	var buf bytes.Buffer
	if err := writeDryRun(&buf, plan, "text"); err != nil {
		t.Fatalf("writeDryRun(text): %v", err)
	}

	out := buf.String()
	for _, want := range []string{
		"DataDir: /data\n",
		"Version: nightly\n",
		"Host: 127.0.0.1\n",
		"Downloads:\n",
		"- Install tidb@v1.0.0\n",
		"Services:\n",
		"- Start pd(pd-0): host=127.0.0.1 port=2380 status_port=2379 dir=/data/pd-0",
	} {
		if !strings.Contains(out, want) {
			t.Fatalf("dry-run text missing %q, got:\n%s", want, out)
		}
	}
}

func TestWriteDryRun_JSON(t *testing.T) {
	plan := BootPlan{
		DataDir:     "/data",
		BootVersion: "nightly",
		Host:        "127.0.0.1",
		Shared:      proc.SharedOptions{Mode: proc.ModeNormal, PDMode: "pd"},
	}

	var buf bytes.Buffer
	if err := writeDryRun(&buf, plan, "json"); err != nil {
		t.Fatalf("writeDryRun(json): %v", err)
	}

	var got BootPlan
	if err := json.Unmarshal(buf.Bytes(), &got); err != nil {
		t.Fatalf("unmarshal dry-run json: %v", err)
	}
	if got.DataDir != plan.DataDir || got.BootVersion != plan.BootVersion || got.Host != plan.Host {
		t.Fatalf("unexpected json plan: %+v", got)
	}
}

func TestWriteDryRun_UnknownFormat(t *testing.T) {
	var buf bytes.Buffer
	if err := writeDryRun(&buf, BootPlan{}, "xml"); err == nil {
		t.Fatalf("expected error for unknown format")
	}
}
