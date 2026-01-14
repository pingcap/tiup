package main

import (
	"io"
	"strings"
	"testing"

	tuiv2output "github.com/pingcap/tiup/pkg/tuiv2/output"
	"github.com/stretchr/testify/require"
)

func TestClusterInfoBasicRows(t *testing.T) {
	pg := NewPlayground("/tmp/tiup-playground-test", 0)
	pg.bootOptions = &BootOptions{Version: "v7.5.0"}

	rows := pg.clusterInfoBasicRows()
	require.GreaterOrEqual(t, len(rows), 2)
	require.Equal(t, [2]string{"Version:", "v7.5.0"}, rows[0])
	require.Equal(t, [2]string{"Data dir:", "/tmp/tiup-playground-test"}, rows[1])
}

func TestClusterInfoBasicRows_DataDirHintWhenDestroyAfterExit(t *testing.T) {
	pg := NewPlayground("/tmp/tiup-playground-test", 0)
	pg.bootOptions = &BootOptions{Version: "v7.5.0"}
	pg.destroyDataAfterExit = true

	rows := pg.clusterInfoBasicRows()
	require.GreaterOrEqual(t, len(rows), 2)
	require.Equal(t, "Data dir:", rows[1][0])
	require.Contains(t, rows[1][1], "/tmp/tiup-playground-test")
	require.Contains(t, rows[1][1], "Destroy after exit")
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
	require.GreaterOrEqual(t, len(rows), 6)

	require.Equal(t, [2]string{"", ""}, rows[2])

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
	require.NotEmpty(t, versionLine)
	require.NotEmpty(t, connectLine)

	idxVer := strings.Index(versionLine, "v7.5.0")
	idxMySQL := strings.Index(connectLine, "mysql")
	require.NotEqual(t, -1, idxVer, "expected values not found:\n%s\n%s", versionLine, connectLine)
	require.NotEqual(t, -1, idxMySQL, "expected values not found:\n%s\n%s", versionLine, connectLine)
	require.Equal(t, idxVer, idxMySQL, "value columns not aligned:\n%s\n%s", versionLine, connectLine)
}
