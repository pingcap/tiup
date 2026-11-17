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

func TestPublishLegacyTiUPAliasMigration(t *testing.T) {
	t.Setenv(localdata.EnvNameMirrorSyncScript, "")
	const alias = "tiup-linux-amd64.tar.gz"
	const newFilename = "tiup-v2.0.0-linux-amd64.tar.gz"
	for _, tc := range []struct {
		name, filename, generatedAlias, url string
		ownsAlias                           bool
	}{
		{name: "custom", filename: "tiup-custom-linux-amd64.tar.gz", generatedAlias: alias, ownsAlias: true},
		{name: "hyphenated custom", filename: "tiup-custom-build-rc.1-linux-amd64.tar.gz", generatedAlias: alias, ownsAlias: true},
		{name: "different suffix", filename: "tiup-custom-darwin-arm64.tar.gz", generatedAlias: "tiup-darwin-arm64.tar.gz"},
		{name: "different prefix", filename: "custom-tiup-linux-amd64.tar.gz", generatedAlias: "custom-linux-amd64.tar.gz"},
		{name: "escaped prefix", filename: "%74iup-custom-linux-amd64.tar.gz", generatedAlias: "%74iup-linux-amd64.tar.gz"},
		{name: "relative URL", url: "tiup-custom-linux-amd64.tar.gz"},
		{name: "double slash URL", url: "//tiup-custom-linux-amd64.tar.gz"},
		{name: "nested URL", url: "/old/../tiup-custom-linux-amd64.tar.gz"},
		{name: "backslash URL", url: "/tiup-custom\\build-linux-amd64.tar.gz"},
		{name: "escaped suffix", url: "/tiup-custom-linux%2Damd64.tar.gz"},
	} {
		for _, metadataOnly := range []bool{false, true} {
			t.Run(fmt.Sprintf("%s/metadata-only=%t", tc.name, metadataOnly), func(t *testing.T) {
				f := newArchiveFixture(t)
				_, err := f.publish(t, v1manifest.NewComponent("tiup", "legacy TiUP", time.Now()), "", "")
				require.NoError(t, err)
				resource := tc.url
				if resource == "" {
					resource = "/" + tc.filename
				}
				legacy := archiveComponent("tiup", "v1.0.0", resource, "legacy marker")
				manifest, err := v1manifest.SignManifest(legacy, f.publisher)
				require.NoError(t, err)
				// Seed metadata and inert archives from before canonical filenames were
				// required. Old copyTiUP derived its alias from the raw filename, not
				// the manifest platform or a decoded URL.
				txn, err := store.New(f.root, "").Begin()
				require.NoError(t, err)
				require.NoError(t, txn.WriteManifest("1.tiup.json", manifest))
				if tc.filename != "" {
					require.NoError(t, txn.Write(tc.filename, strings.NewReader("legacy marker")))
					require.NoError(t, txn.Write(tc.generatedAlias, strings.NewReader("legacy marker")))
				}
				if !tc.ownsAlias {
					require.NoError(t, txn.Write(alias, strings.NewReader("unrelated alias")))
				}
				require.NoError(t, txn.Commit())
				before := archiveRepositoryFiles(t, f.root)

				legacy.Version = 2
				next := archiveComponent("tiup", "v2.0.0", "/"+newFilename, "new marker")
				legacy.Platforms["linux/amd64"]["v2.0.0"] = next.Platforms["linux/amd64"]["v2.0.0"]
				upload := newFilename
				if metadataOnly {
					upload = ""
				}
				writes, err := f.publish(t, legacy, upload, "new marker")
				if !tc.ownsAlias {
					require.ErrorIs(t, err, ErrorWrongFileName, "historical metadata must not authorize an unrelated alias")
					require.Empty(t, writes)
					require.Equal(t, before, archiveRepositoryFiles(t, f.root))
					return
				}
				require.NoError(t, err, "canonical releases must retain ownership of aliases generated from legacy filenames")
				wantAlias := "new marker"
				if metadataOnly {
					wantAlias = "legacy marker"
					require.NoFileExists(t, filepath.Join(f.root, newFilename))
				} else {
					contents, err := os.ReadFile(filepath.Join(f.root, newFilename))
					require.NoError(t, err)
					require.Equal(t, "new marker", string(contents))
				}
				for filename, want := range map[string]string{tc.filename: "legacy marker", alias: wantAlias} {
					contents, err := os.ReadFile(filepath.Join(f.root, filename))
					require.NoError(t, err)
					require.Equal(t, want, string(contents))
				}
				readTxn, err := store.New(f.root, "").Begin()
				require.NoError(t, err)
				defer func() { require.NoError(t, readTxn.Rollback()) }()
				stored, err := readTxn.ReadManifest("2.tiup.json", &v1manifest.Component{})
				require.NoError(t, err)
				require.Equal(t, legacy.Platforms, stored.Signed.(*v1manifest.Component).Platforms)
			})
		}
	}
}
