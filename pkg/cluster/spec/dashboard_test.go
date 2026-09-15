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
	"context"
	"errors"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/pingcap/tiup/pkg/cluster/ctxt"
	logprinter "github.com/pingcap/tiup/pkg/logger/printer"
	"github.com/pingcap/tiup/pkg/meta"
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
	}{
		{
			name:   "uses only the first PD",
			pds:    []*PDSpec{{Host: "10.0.0.1", ClientPort: 2379}, {Host: "10.0.0.2", ClientPort: 2379}},
			wantPD: "http://10.0.0.1:2379",
		},
		{
			name:      "uses https when TLS is enabled",
			pds:       []*PDSpec{{Host: "10.0.0.1", ClientPort: 2379}},
			enableTLS: true,
			wantPD:    "https://10.0.0.1:2379",
		},
		{
			name:   "preserves an explicit advertised client address",
			pds:    []*PDSpec{{Host: "10.0.0.1", ClientPort: 2379, AdvertiseClientAddr: "https://pd.example.com:443"}, {Host: "10.0.0.2", ClientPort: 2379}},
			wantPD: "https://pd.example.com:443",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			topo := &Specification{
				GlobalOptions:    GlobalOptions{User: "tidb", SystemdMode: UserMode, TLSEnabled: tt.enableTLS},
				PDServers:        tt.pds,
				DashboardServers: []*DashboardSpec{{Host: "10.0.0.50", Port: 12333}},
			}
			deployDir := t.TempDir()
			paths := meta.DirPaths{
				Deploy: deployDir,
				Cache:  t.TempDir(),
				Data:   []string{filepath.Join(deployDir, "data")},
				Log:    filepath.Join(deployDir, "log"),
			}
			comp := DashboardComponent{Topology: topo}
			instance := comp.Instances()[0].(*DashboardInstance)
			ctx := ctxt.New(context.Background(), 0, logprinter.NewLogger(""))
			// Stop after the script transfer, before config validation looks up
			// binaries in the global TiUP repository. No mirror or installation
			// is needed to verify the production script-generation path.
			scriptTransferred := errors.New("stop after startup script transfer")
			executor := &mockExecutor{executeFunc: func(_ context.Context, cmd string, _ bool, _ ...time.Duration) ([]byte, []byte, error) {
				if cmd == "chmod +x "+filepath.Join(deployDir, "scripts", "run_tidb-dashboard.sh") {
					return nil, nil, scriptTransferred
				}
				// The mock copies the unit file but does not execute mv.
				// Remove that temporary source instead of leaking it in /tmp.
				if strings.HasPrefix(cmd, "mv /tmp/tidb-dashboard_") {
					require.NoError(t, os.Remove(strings.Fields(cmd)[1]))
				}
				return nil, nil, nil
			}}
			require.ErrorIs(t, instance.InitConfig(ctx, executor, "test-cluster", "v8.5.0", "tidb", paths), scriptTransferred)

			// Inspect the transferred script, so regressions in InitConfig wiring fail.
			body, err := os.ReadFile(filepath.Join(deployDir, "scripts", "run_tidb-dashboard.sh"))
			require.NoError(t, err)
			endpoints := regexp.MustCompile(`--pd="([^"\n]*)"`).FindAllStringSubmatch(string(body), -1)
			require.Len(t, endpoints, 1)
			require.Equal(t, tt.wantPD, endpoints[0][1])
		})
	}
}
