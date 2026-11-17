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

package store

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/pingcap/tiup/pkg/localdata"
	"github.com/pingcap/tiup/pkg/repository/v1manifest"
	"github.com/stretchr/testify/require"
)

func TestCommitPreflightRejectsNonFilesBeforeWriting(t *testing.T) {
	for _, location := range []string{"staging", "repository"} {
		for _, kind := range []string{"directory", "symlink", "dangling symlink"} {
			t.Run(location+"/"+kind, func(t *testing.T) {
				base := t.TempDir()
				t.Setenv(localdata.EnvNameComponentDataDir, base)
				t.Setenv(localdata.EnvNameMirrorSyncScript, "")
				root := filepath.Join(base, "mirror")
				require.NoError(t, os.Mkdir(root, 0700))
				outside := filepath.Join(base, "outside")
				require.NoError(t, os.WriteFile(outside, []byte("outside marker"), 0600))
				originals := map[string]string{
					"a-existing.tar.gz":                  "original package",
					v1manifest.ManifestFilenameTimestamp: "original timestamp",
				}
				for name, contents := range originals {
					require.NoError(t, os.WriteFile(filepath.Join(root, name), []byte(contents), 0600))
				}
				txn, err := New(root, "").Begin()
				require.NoError(t, err)
				defer func() { require.NoError(t, txn.Rollback()) }()
				for _, name := range []string{"a-existing.tar.gz", "b-new.tar.gz", v1manifest.ManifestFilenameTimestamp} {
					require.NoError(t, txn.Write(name, strings.NewReader("replacement")))
				}

				// The rejected entry sorts after both package files. Checking each
				// entry only while copying would publish a partial transaction.
				const blocked = "z-blocked.tar.gz"
				blockedRoot := txn.(*localTxn).root
				if location == "repository" {
					require.NoError(t, txn.Write(blocked, strings.NewReader("blocked replacement")))
					blockedRoot = root
				}
				blockedPath := filepath.Join(blockedRoot, blocked)
				switch kind {
				case "directory":
					require.NoError(t, os.Mkdir(blockedPath, 0700))
				case "symlink":
					require.NoError(t, os.Symlink(outside, blockedPath))
				case "dangling symlink":
					require.NoError(t, os.Symlink(filepath.Join(base, "missing"), blockedPath))
				}

				require.Error(t, txn.Commit())
				for name, contents := range originals {
					actual, err := os.ReadFile(filepath.Join(root, name))
					require.NoError(t, err)
					require.Equal(t, contents, string(actual), "rejected commit changed %s", name)
				}
				_, err = os.Lstat(filepath.Join(root, "b-new.tar.gz"))
				require.True(t, os.IsNotExist(err), "rejected commit created a package: %v", err)
				_, err = os.Lstat(filepath.Join(root, "commits"))
				require.True(t, os.IsNotExist(err), "rejected commit reached the sync archive: %v", err)
				actual, err := os.ReadFile(outside)
				require.NoError(t, err)
				require.Equal(t, "outside marker", string(actual))
				_, err = os.Lstat(filepath.Join(base, "missing"))
				require.True(t, os.IsNotExist(err), "rejected commit created a dangling link target: %v", err)
				staged, err := os.ReadFile(filepath.Join(txn.(*localTxn).root, "a-existing.tar.gz"))
				require.NoError(t, err)
				require.Equal(t, "replacement", string(staged), "failed commit must remain available for rollback")
			})
		}
	}
}

func TestReadManifestRejectsLocalNonFilesWithoutUpstreamFallback(t *testing.T) {
	for _, location := range []string{"staging", "repository"} {
		for _, kind := range []string{"directory", "dangling symlink"} {
			t.Run(location+"/"+kind, func(t *testing.T) {
				base := t.TempDir()
				t.Setenv(localdata.EnvNameComponentDataDir, base)
				root := filepath.Join(base, "mirror")
				require.NoError(t, os.Mkdir(root, 0700))
				var requests atomic.Int32
				upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					requests.Add(1)
					http.NotFound(w, r)
				}))
				defer upstream.Close()
				txn, err := New(root, upstream.URL).Begin()
				require.NoError(t, err)
				defer func() { require.NoError(t, txn.Rollback()) }()
				entryRoot := root
				if location == "staging" {
					entryRoot = txn.(*localTxn).root
				}
				const filename = "component.json"
				entry := filepath.Join(entryRoot, filename)
				if kind == "directory" {
					require.NoError(t, os.Mkdir(entry, 0700))
				} else {
					require.NoError(t, os.Symlink("missing.json", entry))
				}
				_, err = txn.ReadManifest(filename, &v1manifest.Timestamp{})
				require.Error(t, err)
				require.Zero(t, requests.Load(), "rejected local entries must not be treated as missing files")
			})
		}
	}
}
