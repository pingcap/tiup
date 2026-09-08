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
// See the License for the specific language governing permissions and
// limitations under the License.

package operator

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/pingcap/tiup/pkg/cluster/ctxt"
	"github.com/pingcap/tiup/pkg/cluster/spec"
	logprinter "github.com/pingcap/tiup/pkg/logger/printer"
	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"
)

type destroyExecutor struct {
	commands []string
}

func (e *destroyExecutor) Execute(_ context.Context, cmd string, _ bool, _ ...time.Duration) ([]byte, []byte, error) {
	e.commands = append(e.commands, cmd)
	// Empty ss output models stopped ports, so an unexpected exporter cleanup
	// is caught by the assertions without waiting for a timeout.
	return nil, nil, nil
}

func (*destroyExecutor) Transfer(_ context.Context, _, _ string, _ bool, _ int, _ bool) error {
	return nil
}

func TestDestroyIgnoreExporter(t *testing.T) {
	for _, tc := range []struct {
		name              string
		ignoreExporter    bool
		multipleInstances bool
		force             bool
	}{
		{name: "shared", ignoreExporter: true},
		{name: "shared_multiple_instances", ignoreExporter: true, multipleInstances: true},
		{name: "shared_force", ignoreExporter: true, force: true},
		{name: "managed"},
		{name: "managed_multiple_instances", multipleInstances: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			topology := fmt.Sprintf(`
global:
  user: tidb
  deploy_dir: /tidb-deploy
  data_dir: /tidb-data
monitored:
  deploy_dir: /tidb-deploy/monitor-9100
  data_dir: /tidb-data/monitor-9100
  log_dir: /tidb-log/monitor-9100
pd_servers:
  - host: 192.0.2.1
    ignore_exporter: %t
`, tc.ignoreExporter)
			if tc.multipleInstances {
				topology += fmt.Sprintf(`
tidb_servers:
  - host: 192.0.2.1
    ignore_exporter: %t
`, tc.ignoreExporter)
			}
			var topo spec.Specification
			require.NoError(t, yaml.Unmarshal([]byte(topology), &topo))

			exec := &destroyExecutor{}
			ctx := ctxt.New(context.Background(), 1, logprinter.NewLogger(""))
			inner := ctxt.GetInner(ctx)
			inner.SetExecutor("192.0.2.1", exec)
			inner.PublicKeyPath = filepath.Join(t.TempDir(), "id_rsa.pub")
			require.NoError(t, os.WriteFile(inner.PublicKeyPath, []byte("ssh-ed25519 test-key"), 0600))

			require.NoError(t, Destroy(ctx, &topo, Options{Force: tc.force}))

			// Destroy must still remove the cluster's own components.
			require.Contains(t, exec.commands, "rm -rf /etc/systemd/system/pd-2379.service;")
			if tc.multipleInstances {
				require.Contains(t, exec.commands, "rm -rf /etc/systemd/system/tidb-4000.service;")
			}

			wantDeletes := 1
			if tc.ignoreExporter {
				wantDeletes = 0
			}
			for _, path := range []string{
				"/tidb-deploy/monitor-9100",
				"/tidb-data/monitor-9100",
				"/tidb-log/monitor-9100",
				"/etc/systemd/system/node_exporter-9100.service",
				"/etc/systemd/system/blackbox_exporter-9115.service",
			} {
				deletes := 0
				for _, cmd := range exec.commands {
					if strings.HasPrefix(cmd, "rm -rf ") && strings.Contains(cmd, path) {
						deletes++
					}
				}
				require.Equal(t, wantDeletes, deletes, "deletions of %s: %v", path, exec.commands)
			}

			portChecks := 0
			for _, cmd := range exec.commands {
				if cmd == "ss -ltn" {
					portChecks++
				}
			}
			require.Equal(t, 2*wantDeletes, portChecks, "each managed exporter port is checked once")
		})
	}
}
