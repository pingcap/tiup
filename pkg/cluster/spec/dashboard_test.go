// Copyright 2020 PingCAP, Inc.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// See the License for the specific language governing permissions and
// limitations under the License.

package spec

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/pingcap/tiup/pkg/cluster/template/scripts"
	"github.com/stretchr/testify/require"
)

func TestDashboardPDEndpoint(t *testing.T) {
	t.Run("selects the first PD endpoint", func(t *testing.T) {
		pds := []*PDSpec{
			{Host: "10.0.0.1", ClientPort: 2379},
			{Host: "10.0.0.2", ClientPort: 2379},
		}
		require.Equal(t, "http://10.0.0.1:2379", dashboardPDEndpoint(pds, false))
	})

	t.Run("returns empty for no PD servers", func(t *testing.T) {
		require.Equal(t, "", dashboardPDEndpoint(nil, false))
		require.Equal(t, "", dashboardPDEndpoint([]*PDSpec{}, false))
	})
}

// TestDashboardScriptSinglePD verifies that the generated startup script passes
// a single --pd endpoint even when the cluster has multiple PD servers, which
// is the standalone tidb-dashboard connectivity fix.
func TestDashboardScriptSinglePD(t *testing.T) {
	tests := []struct {
		name      string
		pds       []*PDSpec
		enableTLS bool
		wantPD    string
		wantNot   string
	}{
		{
			name:    "uses only the first PD",
			pds:     []*PDSpec{{Host: "10.0.0.1", ClientPort: 2379}, {Host: "10.0.0.2", ClientPort: 2379}},
			wantPD:  "http://10.0.0.1:2379",
			wantNot: "10.0.0.2",
		},
		{
			name:      "uses https when TLS is enabled",
			pds:       []*PDSpec{{Host: "10.0.0.1", ClientPort: 2379}},
			enableTLS: true,
			wantPD:    "https://10.0.0.1:2379",
		},
		{
			name:    "preserves an explicit advertised client address",
			pds:     []*PDSpec{{Host: "10.0.0.1", ClientPort: 2379, AdvertiseClientAddr: "https://pd.example.com:443"}, {Host: "10.0.0.2", ClientPort: 2379}},
			wantPD:  "https://pd.example.com:443",
			wantNot: "10.0.0.2",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			script := &scripts.DashboardScript{
				Host:        "0.0.0.0",
				Port:        12333,
				DeployDir:   "/tidb-deploy",
				DataDir:     "/tidb-data",
				LogDir:      "/tidb-log",
				TidbVersion: "v8.5.0",
				PD:          dashboardPDEndpoint(tt.pds, tt.enableTLS),
			}

			file := filepath.Join(t.TempDir(), "run_tidb-dashboard.sh")
			require.NoError(t, script.ConfigToFile(file))

			body, err := os.ReadFile(file)
			require.NoError(t, err)

			require.Contains(t, string(body), `--pd="`+tt.wantPD+`"`)
			if tt.wantNot != "" {
				require.NotContains(t, string(body), tt.wantNot)
			}
		})
	}
}
