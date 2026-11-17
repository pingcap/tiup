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

package main

import (
	"bytes"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"io/fs"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/pingcap/tiup/pkg/localdata"
	"github.com/pingcap/tiup/pkg/repository/model"
	"github.com/pingcap/tiup/pkg/repository/v1manifest"
	"github.com/stretchr/testify/require"
)

func TestPublishHTTPUploadSessionDoesNotAuthorize(t *testing.T) {
	for _, scenario := range []string{"unsigned", "unregistered signer", "different granted owner", "tampered signed body"} {
		t.Run(scenario, func(t *testing.T) {
			t.Setenv(localdata.EnvNameMirrorSyncScript, "")
			s, root, data := newPublishTestServer(t)
			owner, err := v1manifest.GenKeyInfo()
			require.NoError(t, err)
			public, err := owner.Public()
			require.NoError(t, err)
			require.NoError(t, s.mirror.Grant("owner", "original publisher", public))
			other, err := v1manifest.GenKeyInfo()
			require.NoError(t, err)
			if scenario == "different granted owner" {
				public, err = other.Public()
				require.NoError(t, err)
				require.NoError(t, s.mirror.Grant("other", "another publisher", public))
			}
			const filename = "component-nightly-linux-amd64.tar.gz"
			component := func(payload string, revision uint) *v1manifest.Component {
				comp := v1manifest.NewComponent("component", "test component", time.Now())
				comp.Version = revision
				comp.Platforms["linux/amd64"] = map[string]v1manifest.VersionItem{
					"nightly": {URL: "/" + filename, FileHash: v1manifest.FileHash{
						Length: uint(len(payload)), Hashes: map[string]string{"sha256": fmt.Sprintf("%x", sha256.Sum256([]byte(payload)))},
					}},
				}
				return comp
			}
			original, err := v1manifest.SignManifest(component("original package", 1), owner)
			require.NoError(t, err)
			require.NoError(t, s.mirror.Publish(original, &model.PublishInfo{ComponentData: &model.TarInfo{
				Name: filename, Reader: strings.NewReader("original package"),
			}}))
			const sid = "existing-upload-session"
			require.NoError(t, s.sm.Write(sid, filename, strings.NewReader("replacement package")))
			defer s.sm.Delete(sid)
			before := publishRepositoryFiles(t, root)
			comp := component("replacement package", 2)
			manifest := &v1manifest.Manifest{Signed: comp}
			switch scenario {
			case "unregistered signer", "different granted owner":
				manifest, err = v1manifest.SignManifest(comp, other)
				require.NoError(t, err)
			case "tampered signed body":
				manifest, err = v1manifest.SignManifest(comp, owner)
				require.NoError(t, err)
				comp.Description = "changed after signing"
			}
			post := func(manifest *v1manifest.Manifest) int {
				t.Helper()
				body, err := json.Marshal(manifest)
				require.NoError(t, err)
				r := httptest.NewRequest(http.MethodPost, "/api/v1/component/"+sid+"/component?hidden=true&yanked=true&standalone=true", bytes.NewReader(body))
				r.Header.Set("Content-Type", "application/json")
				w := httptest.NewRecorder()
				s.router().ServeHTTP(w, r)
				return w.Code
			}
			require.Equal(t, http.StatusForbidden, post(manifest))
			require.Equal(t, before, publishRepositoryFiles(t, root), "rejection must preserve all committed metadata, package bytes and sync archives")
			staging, err := filepath.Glob(filepath.Join(data, "tiup-commit-*"))
			require.NoError(t, err)
			require.Empty(t, staging, "failed publication must roll back its staging directory")

			// The same session remains usable by the real owner after rejection;
			// possession of it neither authorizes publication nor poisons the revision.
			manifest, err = v1manifest.SignManifest(comp, owner)
			require.NoError(t, err)
			require.Equal(t, http.StatusNoContent, post(manifest))
			contents, err := os.ReadFile(filepath.Join(root, filename))
			require.NoError(t, err)
			require.Equal(t, "replacement package", string(contents))
			require.FileExists(t, filepath.Join(root, "2.component.json"))
		})
	}
}

func publishRepositoryFiles(t *testing.T, root string) map[string]string {
	t.Helper()
	files := make(map[string]string)
	require.NoError(t, filepath.WalkDir(root, func(name string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.IsDir() {
			return nil
		}
		relative, err := filepath.Rel(root, name)
		if err != nil {
			return err
		}
		contents, err := os.ReadFile(name)
		if err != nil {
			return err
		}
		files[relative] = string(contents)
		return nil
	}))
	return files
}
