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
		Shared: proc.SharedOptions{
			Mode:               proc.ModeCSE,
			PDMode:             "pd",
			PortOffset:         123,
			HighPerf:           true,
			EnableTiKVColumnar: true,
			ForcePull:          true,
			CSE: proc.CSEOptions{
				S3Endpoint: "https://s3.example.com",
				Bucket:     "my-bucket",
				AccessKey:  "fake-access-key",
				SecretKey:  "fake-secret-key",
			},
		},
		Monitor:     true,
		GrafanaPort: 3000,
		Downloads: []DownloadPlan{
			{ComponentID: "tidb", ResolvedVersion: "v1.0.0", DebugReason: "missing_binary", DebugBinPath: "/home/tidb-server"},
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
			{
				Name:            "tidb-0",
				ServiceID:       proc.ServiceTiDB.String(),
				ComponentID:     proc.ComponentTiDB.String(),
				ResolvedVersion: "v1.0.0",
				BinPath:         "/usr/local/bin/tidb-server",
				Shared:          ServiceSharedPlan{Dir: "/data/tidb-0", Host: "127.0.0.1", Port: 4000, StatusPort: 10080},
			},
		},
	}

	var buf bytes.Buffer
	if err := writeDryRun(&buf, plan, "text"); err != nil {
		t.Fatalf("writeDryRun(text): %v", err)
	}

	out := buf.String()
	for _, want := range []string{
		"==> Download Packages:\n",
		"  + tidb@v1.0.0\n",
		"==> Reuse Packages:\n",
		"  + pd@v1.0.0\n",
		"==> Start Services:\n",
		"  + pd-0@v1.0.0\n",
		"    127.0.0.1:2380,2379(status)\n",
		"    Start after: tikv\n",
		"  + tidb-0@v1.0.0 (use /usr/local/bin/tidb-server)\n",
		"    127.0.0.1:4000,10080(status)\n",
	} {
		if !strings.Contains(out, want) {
			t.Fatalf("dry-run text missing %q, got:\n%s", want, out)
		}
	}

	for _, secret := range []string{"fake-access-key", "fake-secret-key"} {
		if strings.Contains(out, secret) {
			t.Fatalf("dry-run text should not include secret %q, got:\n%s", secret, out)
		}
	}
}

func TestWriteDryRun_Text_ShowsComponentHintWhenDifferent(t *testing.T) {
	plan := BootPlan{
		Services: []ServicePlan{
			{
				Name:            "ng-monitoring-0",
				ServiceID:       proc.ServiceNGMonitoring.String(),
				ComponentID:     proc.ComponentPrometheus.String(),
				ResolvedVersion: "v1.0.0",
				Shared:          ServiceSharedPlan{Host: "127.0.0.1", Port: 12020, StatusPort: 12020},
			},
		},
	}

	var buf bytes.Buffer
	if err := writeDryRun(&buf, plan, "text"); err != nil {
		t.Fatalf("writeDryRun(text): %v", err)
	}

	if got := buf.String(); !strings.Contains(got, "  + ng-monitoring-0@v1.0.0 (prometheus)\n") {
		t.Fatalf("dry-run text missing component hint, got:\n%s", got)
	}
	if got := buf.String(); !strings.Contains(got, "    127.0.0.1:12020\n") {
		t.Fatalf("dry-run text missing address, got:\n%s", got)
	}
	if got := buf.String(); strings.Contains(got, "(status)") {
		t.Fatalf("dry-run text should not include status port suffix when the ports are equal, got:\n%s", got)
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

func TestWriteDryRun_JSON_RedactsSecrets(t *testing.T) {
	plan := BootPlan{
		Shared: proc.SharedOptions{
			Mode: proc.ModeCSE,
			CSE: proc.CSEOptions{
				AccessKey: "access-KEY-123",
				SecretKey: "secret-KEY-456",
			},
		},
	}

	var buf bytes.Buffer
	if err := writeDryRun(&buf, plan, "json"); err != nil {
		t.Fatalf("writeDryRun(json): %v", err)
	}

	out := buf.String()
	for _, secret := range []string{"access-KEY-123", "secret-KEY-456"} {
		if strings.Contains(out, secret) {
			t.Fatalf("dry-run json should redact secret %q, got:\n%s", secret, out)
		}
	}

	var got BootPlan
	if err := json.Unmarshal(buf.Bytes(), &got); err != nil {
		t.Fatalf("unmarshal dry-run json: %v", err)
	}
	if got.Shared.CSE.AccessKey != "***" || got.Shared.CSE.SecretKey != "***" {
		t.Fatalf("unexpected json secret values: %+v", got.Shared.CSE)
	}
}

func TestWriteDryRun_JSON_OmitsNilOneOfFields(t *testing.T) {
	plan := BootPlan{
		Services: []ServicePlan{
			{
				Name:            "pd-0",
				ServiceID:       proc.ServicePD.String(),
				ComponentID:     proc.ComponentPD.String(),
				ResolvedVersion: "v1.0.0",
			},
		},
	}

	var buf bytes.Buffer
	if err := writeDryRun(&buf, plan, "json"); err != nil {
		t.Fatalf("writeDryRun(json): %v", err)
	}

	out := buf.String()
	for _, field := range []string{
		"\"TiKV\": null",
		"\"TiDB\": null",
		"\"TiKVWorker\": null",
		"\"TiFlash\": null",
		"\"TiProxy\": null",
		"\"Grafana\": null",
		"\"NGMonitoring\": null",
		"\"TiCDC\": null",
		"\"TiKVCDC\": null",
		"\"DMMaster\": null",
		"\"DMWorker\": null",
		"\"Pump\": null",
		"\"Drainer\": null",
	} {
		if strings.Contains(out, field) {
			t.Fatalf("dry-run json should omit nil one-of field %q, got:\n%s", field, out)
		}
	}
}

func TestWriteDryRun_JSON_MapOrderIsStable(t *testing.T) {
	plan := BootPlan{
		RequiredServices: map[string]int{
			"zzz": 1,
			"aaa": 2,
		},
		DebugServiceConfigs: map[string]proc.Config{
			"zzz_cfg": {Num: 1},
			"aaa_cfg": {Num: 2},
		},
	}

	var buf bytes.Buffer
	if err := writeDryRun(&buf, plan, "json"); err != nil {
		t.Fatalf("writeDryRun(json): %v", err)
	}

	out := buf.String()
	idxAAA := strings.Index(out, "\"aaa\":")
	idxZZZ := strings.Index(out, "\"zzz\":")
	if idxAAA < 0 || idxZZZ < 0 || idxAAA > idxZZZ {
		t.Fatalf("unexpected RequiredServices json order, got:\n%s", out)
	}

	idxAAACfg := strings.Index(out, "\"aaa_cfg\":")
	idxZZZCfg := strings.Index(out, "\"zzz_cfg\":")
	if idxAAACfg < 0 || idxZZZCfg < 0 || idxAAACfg > idxZZZCfg {
		t.Fatalf("unexpected DebugServiceConfigs json order, got:\n%s", out)
	}
}

func TestWriteDryRun_UnknownFormat(t *testing.T) {
	var buf bytes.Buffer
	if err := writeDryRun(&buf, BootPlan{}, "xml"); err == nil {
		t.Fatalf("expected error for unknown format")
	}
}
