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

package model

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/pingcap/tiup/pkg/localdata"
	"github.com/pingcap/tiup/pkg/repository/store"
	"github.com/pingcap/tiup/pkg/repository/v1manifest"
	"github.com/stretchr/testify/require"
)

func TestPublishLegacyArchiveOwnership(t *testing.T) {
	t.Setenv(localdata.EnvNameMirrorSyncScript, "")
	const filename = "example-v2.0.0-linux-amd64.tar.gz"
	for _, tc := range []struct {
		name, id, resource, conflict string
		legacyFiles                  []string
	}{
		{
			name: "alias prefix case", id: "tiup", resource: "/TiUP-custom-linux-amd64.tar.gz", conflict: "tiup-linux-amd64.tar.gz",
			legacyFiles: []string{"TiUP-custom-linux-amd64.tar.gz", "TiUP-linux-amd64.tar.gz"},
		},
		{
			name: "alias suffix case", id: "tiup", resource: "/tiup-custom-Linux-amd64.tar.gz", conflict: "tiup-linux-amd64.tar.gz",
			legacyFiles: []string{"tiup-custom-Linux-amd64.tar.gz", "tiup-Linux-amd64.tar.gz"},
		},
		{
			name: "archive case", id: "example", resource: "/Example-v2.0.0-linux-amd64.tar.gz",
			legacyFiles: []string{"Example-v2.0.0-linux-amd64.tar.gz"},
		},
		{
			name: "escaped archive", id: "example", resource: "/%65xample-v2.0.0-linux-amd64.tar.gz",
			legacyFiles: []string{"%65xample-v2.0.0-linux-amd64.tar.gz"},
		},
		{name: "relative archive", id: "example", resource: filename},
		{name: "double slash archive", id: "example", resource: "//" + filename},
		{name: "nested archive", id: "example", resource: "/old/../" + filename},
		{
			name: "archive query", id: "example", resource: "/" + filename + "?download=1",
			legacyFiles: []string{filename + "?download=1"},
		},
		{
			name: "archive fragment", id: "example", resource: "/" + filename + "#download",
			legacyFiles: []string{filename + "#download"},
		},
	} {
		for _, metadataOnly := range []bool{false, true} {
			t.Run(fmt.Sprintf("%s/metadata-only=%t", tc.name, metadataOnly), func(t *testing.T) {
				f := newArchiveFixture(t)
				_, err := f.publish(t, v1manifest.NewComponent(tc.id, "legacy component", time.Now()), "", "")
				require.NoError(t, err)
				legacy := archiveComponent(tc.id, "v1.0.0", tc.resource, "legacy marker")
				manifest, err := v1manifest.SignManifest(legacy, f.publisher)
				require.NoError(t, err)
				txn, err := store.New(f.root, "").Begin()
				require.NoError(t, err)
				require.NoError(t, txn.WriteManifest("1."+tc.id+".json", manifest))
				for _, name := range tc.legacyFiles {
					require.NoError(t, txn.Write(name, strings.NewReader("legacy marker")))
				}
				require.NoError(t, txn.Commit())

				newFilename := tc.id + "-v2.0.0-linux-amd64.tar.gz"
				conflict := tc.conflict
				if conflict == "" {
					conflict = newFilename
				}
				for _, name := range tc.legacyFiles {
					if strings.EqualFold(name, conflict) {
						_, err := os.Stat(filepath.Join(f.root, conflict))
						if err == nil {
							t.Skip("requires a case-sensitive filesystem to retain both filename variants")
						}
						require.ErrorIs(t, err, os.ErrNotExist)
					}
				}
				// A retired, unreferenced file must remain independent of historical
				// URLs that normalize to it, including case-distinct TiUP aliases.
				require.NoError(t, os.WriteFile(filepath.Join(f.root, conflict), []byte("unrelated marker"), 0600))
				before := archiveRepositoryFiles(t, f.root)
				for _, name := range tc.legacyFiles {
					require.Equal(t, []byte("legacy marker"), before[filepath.Join(f.root, name)])
				}
				require.Equal(t, []byte("unrelated marker"), before[filepath.Join(f.root, conflict)])

				legacy.Version = 2
				next := archiveComponent(tc.id, "v2.0.0", "/"+newFilename, "new marker")
				legacy.Platforms["linux/amd64"]["v2.0.0"] = next.Platforms["linux/amd64"]["v2.0.0"]
				upload := newFilename
				if metadataOnly {
					upload = ""
				}
				writes, err := f.publish(t, legacy, upload, "new marker")
				require.ErrorIs(t, err, ErrorWrongFileName, "normalized legacy URLs must not grant ownership of distinct files")
				require.Empty(t, writes, "rejection must precede staging metadata, archives, or aliases")
				require.Equal(t, before, archiveRepositoryFiles(t, f.root), "rejection must preserve every committed file")
			})
		}
	}
}
