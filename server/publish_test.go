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
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/pingcap/tiup/pkg/localdata"
	"github.com/pingcap/tiup/pkg/repository"
	"github.com/pingcap/tiup/pkg/repository/model"
	"github.com/pingcap/tiup/pkg/repository/v1manifest"
	"github.com/stretchr/testify/require"
)

func newPublishTestServer(t *testing.T) (*server, string, string) {
	t.Helper()
	base := t.TempDir()
	data := filepath.Join(base, "staging")
	require.NoError(t, os.Mkdir(data, 0700))
	t.Setenv(localdata.EnvNameComponentDataDir, data)
	root := filepath.Join(base, "mirror")
	keys := filepath.Join(base, "keys")
	require.NoError(t, os.Mkdir(root, 0700))
	require.NoError(t, v1manifest.Init(root, keys, time.Now()))
	s, err := newServer(root, keys, "")
	require.NoError(t, err)
	return s, root, data
}

func TestPublishTraversalRejectedWithoutOutsideWrite(t *testing.T) {
	s, _, data := newPublishTestServer(t)

	for _, existing := range []bool{false, true} {
		t.Run(map[bool]string{false: "create", true: "overwrite"}[existing], func(t *testing.T) {
			target := filepath.Join(data, "victim.json")
			if existing {
				require.NoError(t, os.WriteFile(target, []byte("inert original marker"), 0600))
			}
			comp := v1manifest.NewComponent("x/../../victim", "inert test manifest", time.Now())
			body, err := json.Marshal(&v1manifest.Manifest{Signed: comp})
			require.NoError(t, err)
			r := httptest.NewRequest(http.MethodPost, "/api/v1/component/unknown-session/safe-name", bytes.NewReader(body))
			r.Header.Set("Content-Type", "application/json")
			w := httptest.NewRecorder()
			s.router().ServeHTTP(w, r)
			require.GreaterOrEqual(t, w.Code, http.StatusBadRequest)
			if existing {
				contents, err := os.ReadFile(target)
				require.NoError(t, err)
				require.Equal(t, "inert original marker", string(contents), "a rejected request must not overwrite an outside file")
			} else {
				require.NoFileExists(t, target, "a rejected request must not create an outside file")
			}
		})
	}
}

func TestPublishHTTPBindingAndCompatibility(t *testing.T) {
	s, root, _ := newPublishTestServer(t)
	publisher, err := v1manifest.GenKeyInfo()
	require.NoError(t, err)
	public, err := publisher.Public()
	require.NoError(t, err)
	require.NoError(t, s.mirror.Grant("publisher", "test publisher", public))

	t.Run("route and signed id must agree", func(t *testing.T) {
		comp := v1manifest.NewComponent("actual", "inert", time.Now())
		manifest, err := v1manifest.SignManifest(comp, publisher)
		require.NoError(t, err)
		body, err := json.Marshal(manifest)
		require.NoError(t, err)
		r := httptest.NewRequest(http.MethodPost, "/api/v1/component/unknown-session/different", bytes.NewReader(body))
		r.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()
		s.router().ServeHTTP(w, r)
		require.Equal(t, http.StatusBadRequest, w.Code)
		require.NoFileExists(t, filepath.Join(root, "1.actual.json"))
	})

	local := httptest.NewServer(s.router())
	defer local.Close()
	remote := repository.NewMirror(local.URL, repository.MirrorOptions{})
	t.Run("metadata-only publish needs no upload session", func(t *testing.T) {
		comp := v1manifest.NewComponent("metadata", "inert", time.Now())
		manifest, err := v1manifest.SignManifest(comp, publisher)
		require.NoError(t, err)
		require.NoError(t, remote.Publish(manifest, &model.PublishInfo{}))
		comp.Version++
		manifest, err = v1manifest.SignManifest(comp, publisher)
		require.NoError(t, err)
		hidden := true
		require.NoError(t, remote.Publish(manifest, &model.PublishInfo{Hide: &hidden}))
		require.FileExists(t, filepath.Join(root, "2.metadata.json"))
		contents, err := os.ReadFile(filepath.Join(root, "snapshot.json"))
		require.NoError(t, err)
		var snapshot struct {
			Signed v1manifest.Snapshot `json:"signed"`
		}
		require.NoError(t, json.Unmarshal(contents, &snapshot))
		contents, err = os.ReadFile(filepath.Join(root, fmt.Sprintf("%d.index.json", snapshot.Signed.Meta[v1manifest.ManifestURLIndex].Version)))
		require.NoError(t, err)
		var index struct {
			Signed v1manifest.Index `json:"signed"`
		}
		require.NoError(t, json.Unmarshal(contents, &index))
		require.True(t, index.Signed.Components[comp.ID].Hidden)
	})
	t.Run("signed package upload and publish", func(t *testing.T) {
		const payload = "inert package content"
		const filename = "package-v1.0.0-linux-amd64.tar.gz"
		comp := v1manifest.NewComponent("package", "inert", time.Now())
		comp.Platforms["linux/amd64"] = map[string]v1manifest.VersionItem{
			"v1.0.0": {URL: "/" + filename, FileHash: v1manifest.FileHash{Length: uint(len(payload)), Hashes: map[string]string{"sha256": fmt.Sprintf("%x", sha256.Sum256([]byte(payload)))}}},
		}
		manifest, err := v1manifest.SignManifest(comp, publisher)
		require.NoError(t, err)
		require.NoError(t, remote.Publish(manifest, &model.PublishInfo{ComponentData: &model.TarInfo{Reader: strings.NewReader(payload), Name: filename}}))
		contents, err := os.ReadFile(filepath.Join(root, filename))
		require.NoError(t, err)
		require.Equal(t, payload, string(contents))
	})
}
