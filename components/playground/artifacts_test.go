package main

import (
	"io"
	"strings"
	"testing"

	tuiv2output "github.com/pingcap/tiup/pkg/tuiv2/output"
)

func TestClusterInfoBasicRows(t *testing.T) {
	pg := NewPlayground("/tmp/tiup-playground-test", 0)
	pg.bootOptions = &BootOptions{Version: "v7.5.0"}

	rows := pg.clusterInfoBasicRows()
	if len(rows) < 2 {
		t.Fatalf("expected at least 2 rows, got %d", len(rows))
	}

	if rows[0][0] != "Version:" || rows[0][1] != "v7.5.0" {
		t.Fatalf("unexpected version row: %v", rows[0])
	}
	if rows[1][0] != "Data dir:" || rows[1][1] != "/tmp/tiup-playground-test" {
		t.Fatalf("unexpected data dir row: %v", rows[1])
	}
}

func TestClusterInfoBasicRows_DataDirHintWhenDeleteAfterExit(t *testing.T) {
	pg := NewPlayground("/tmp/tiup-playground-test", 0)
	pg.bootOptions = &BootOptions{Version: "v7.5.0"}
	pg.deleteWhenExit = true

	rows := pg.clusterInfoBasicRows()
	if len(rows) < 2 {
		t.Fatalf("expected at least 2 rows, got %d", len(rows))
	}
	if rows[1][0] != "Data dir:" {
		t.Fatalf("unexpected data dir label: %v", rows[1])
	}
	if !strings.Contains(rows[1][1], "/tmp/tiup-playground-test") {
		t.Fatalf("unexpected data dir value: %q", rows[1][1])
	}
	if !strings.Contains(rows[1][1], "Destroy after exit") {
		t.Fatalf("expected destroy hint in data dir value: %q", rows[1][1])
	}
}

func TestClusterInfoCalloutRows_AlignedWithSeparatorRow(t *testing.T) {
	pg := NewPlayground("/tmp/tiup-playground-test", 0)
	pg.bootOptions = &BootOptions{Version: "v7.5.0"}

	rows := pg.clusterInfoCalloutRows(
		"mysql",
		"http://127.0.0.1:2379/dashboard",
		"http://127.0.0.1:3000",
		[]string{"127.0.0.1:4000"},
		nil,
	)
	if len(rows) < 6 {
		t.Fatalf("expected at least 6 rows, got %d", len(rows))
	}

	if rows[2] != [2]string{"", ""} {
		t.Fatalf("expected empty separator row at index 2, got %v", rows[2])
	}

	lines := tuiv2output.Labels{Rows: rows}.Lines(io.Discard)
	var versionLine, connectLine string
	for _, line := range lines {
		if strings.HasPrefix(line, "Version:") {
			versionLine = line
		}
		if strings.HasPrefix(line, "Connect TiDB:") {
			connectLine = line
		}
	}
	if versionLine == "" || connectLine == "" {
		t.Fatalf("missing version/connect lines: %q / %q", versionLine, connectLine)
	}

	idxVer := strings.Index(versionLine, "v7.5.0")
	idxMySQL := strings.Index(connectLine, "mysql")
	if idxVer < 0 || idxMySQL < 0 {
		t.Fatalf("expected values not found:\n%s\n%s", versionLine, connectLine)
	}
	if idxVer != idxMySQL {
		t.Fatalf("value columns not aligned: version=%d mysql=%d\n%s\n%s", idxVer, idxMySQL, versionLine, connectLine)
	}
}
