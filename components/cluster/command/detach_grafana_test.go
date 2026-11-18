// Copyright 2026 PingCAP, Inc.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package command

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/pingcap/tiup/pkg/cluster/spec"
	"github.com/pingcap/tiup/pkg/utils"
	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"
)

func TestDetachGrafanaCLI(t *testing.T) {
	// Each CLI invocation needs a fresh set of root flags, profile globals, and
	// repository state. A subprocess exercises the real hooks without test-only
	// injection into the command implementation.
	if os.Getenv("TIUP_TEST_DETACH_GRAFANA_CLI") == "1" {
		rootCmd.SetArgs(os.Args[3:])
		Execute()
		return
	}

	trustedRoot, err := os.ReadFile("../../../tests/tiup-cluster/root.json")
	require.NoError(t, err)
	for _, test := range []struct {
		name             string
		args             []string
		input            string
		firstGrafanaHost string
		wantIDs          []string
		wantError        bool
		errorText        string
	}{
		{
			name: "repeated short node flag",
			args: []string{"detach-grafana", "test", "-N", "192.0.2.2:3000", "-N", "192.0.2.3:3000", "-y"},
		},
		{
			name: "comma separated long node flag",
			args: []string{"detach-grafana", "test", "--node", "192.0.2.2:3000,192.0.2.3:3000", "--yes"},
		},
		{
			name:    "subset",
			args:    []string{"detach-grafana", "test", "-N", "192.0.2.2:3000", "-y"},
			wantIDs: []string{"192.0.2.3:3000"},
		},
		{
			name:             "shared host with default sibling directories",
			args:             []string{"detach-grafana", "test", "-N", "192.0.2.1:3000", "-y"},
			firstGrafanaHost: "192.0.2.1",
			wantIDs:          []string{"192.0.2.3:3000"},
		},
		{
			name:      "missing node flag",
			args:      []string{"detach-grafana", "test", "-y"},
			wantError: true, errorText: "required flag(s)",
		},
		{
			name:      "missing cluster",
			args:      []string{"detach-grafana", "-N", "192.0.2.2:3000", "-y"},
			wantError: true, errorText: "accepts 1 arg(s)",
		},
		{
			name:      "extra cluster",
			args:      []string{"detach-grafana", "test", "another", "-N", "192.0.2.2:3000", "-y"},
			wantError: true, errorText: "accepts 1 arg(s)",
		},
		{
			name:      "empty node list",
			args:      []string{"detach-grafana", "test", "--node=", "-y"},
			wantError: true,
		},
		{
			name:      "mixed component IDs",
			args:      []string{"detach-grafana", "test", "-N", "192.0.2.2:3000,192.0.2.1:2379", "-y"},
			wantError: true,
		},
		{
			name:  "confirmation declined",
			args:  []string{"detach-grafana", "test", "-N", "192.0.2.2:3000"},
			input: "n\n", wantError: true,
		},
		{
			name: "other command still initializes proxy",
			args: []string{"list"}, wantError: true, errorText: "start http-proxy",
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			profile := t.TempDir()
			require.NoError(t, os.MkdirAll(filepath.Join(profile, "bin"), 0o755))
			require.NoError(t, os.WriteFile(filepath.Join(profile, "bin", "root.json"), trustedRoot, 0o600))
			// A configured but unreachable mirror also detects accidental downloads.
			require.NoError(t, os.WriteFile(filepath.Join(profile, "tiup.toml"), []byte("mirror = 'http://127.0.0.1:1'\n"), 0o600))
			metaPath := filepath.Join(profile, "storage", "cluster", "clusters", "test", "meta.yaml")
			require.NoError(t, os.MkdirAll(filepath.Dir(metaPath), 0o755))
			firstGrafanaHost := test.firstGrafanaHost
			if firstGrafanaHost == "" {
				firstGrafanaHost = "192.0.2.2"
			}
			metadata := []byte(fmt.Sprintf(`user: tidb
tidb_version: v8.5.6
topology:
  global:
    user: tidb
    deploy_dir: /srv/tiup-detach-test
    data_dir: /srv/tiup-detach-data
  pd_servers:
    - host: 192.0.2.1
  tidb_servers:
    - host: 192.0.2.1
  grafana_servers:
    - host: %s
    - host: 192.0.2.3
`, firstGrafanaHost))
			require.NoError(t, os.WriteFile(metaPath, metadata, 0o600))
			invalidKey := filepath.Join(profile, "invalid-proxy-key")
			require.NoError(t, os.WriteFile(invalidKey, []byte("not an SSH private key"), 0o600))

			ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			defer cancel()
			args := append([]string{"-test.run=^TestDetachGrafanaCLI$", "--"}, test.args...)
			args = append(args, "--ssh-proxy-host=192.0.2.254", "--ssh-proxy-identity-file="+invalidKey)
			command := exec.CommandContext(ctx, os.Args[0], args...)
			command.Env = append(os.Environ(),
				"TIUP_TEST_DETACH_GRAFANA_CLI=1", "TIUP_HOME="+profile,
				"TIUP_COMPONENT_DATA_DIR=", "TIUP_MIRRORS=http://127.0.0.1:1", "SSH_AUTH_SOCK=",
			)
			command.Stdin = strings.NewReader(test.input)
			output, commandErr := command.CombinedOutput()
			require.NoError(t, ctx.Err(), "CLI did not finish: %s", output)
			if test.wantError {
				require.Error(t, commandErr, "%s", output)
				if test.errorText != "" {
					require.Contains(t, string(output), test.errorText)
				}
			} else {
				require.NoError(t, commandErr, "%s", output)
			}
			require.NotContains(t, string(output), "Input SSH password:")
			require.NotContains(t, string(output), "Input its passphrase:")
			current, err := os.ReadFile(metaPath)
			require.NoError(t, err)
			if test.wantError {
				require.Equal(t, metadata, current)
				return
			}
			var result spec.ClusterMeta
			require.NoError(t, yaml.Unmarshal(current, &result))
			var ids []string
			for _, grafana := range result.Topology.Grafanas {
				ids = append(ids, utils.JoinHostPort(grafana.Host, grafana.Port))
			}
			require.Equal(t, test.wantIDs, ids)
			require.Equal(t, "v8.5.6", result.Version)
			require.Len(t, result.Topology.PDServers, 1)
			require.Len(t, result.Topology.TiDBServers, 1)
		})
	}
}
