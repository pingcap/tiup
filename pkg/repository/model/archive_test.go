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
	"crypto/sha256"
	"encoding/json"
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

type archiveFixture struct {
	root      string
	keys      map[string]*v1manifest.KeyInfo
	publisher *v1manifest.KeyInfo
}

func newArchiveFixture(t *testing.T) *archiveFixture {
	t.Helper()
	base := t.TempDir()
	t.Setenv(localdata.EnvNameComponentDataDir, base)
	f := &archiveFixture{root: filepath.Join(base, "mirror"), keys: make(map[string]*v1manifest.KeyInfo)}
	keyDir := filepath.Join(base, "keys")
	require.NoError(t, os.Mkdir(f.root, 0700))
	require.NoError(t, v1manifest.Init(f.root, keyDir, time.Now()))
	files, err := os.ReadDir(keyDir)
	require.NoError(t, err)
	for _, file := range files {
		contents, err := os.ReadFile(filepath.Join(keyDir, file.Name()))
		require.NoError(t, err)
		var key v1manifest.KeyInfo
		require.NoError(t, json.Unmarshal(contents, &key))
		id, err := key.ID()
		require.NoError(t, err)
		f.keys[id] = &key
	}
	f.publisher, err = v1manifest.GenKeyInfo()
	require.NoError(t, err)
	pub, err := f.publisher.Public()
	require.NoError(t, err)
	txn, err := store.New(f.root, "").Begin()
	require.NoError(t, err)
	require.NoError(t, New(txn, f.keys).Grant("publisher", "test publisher", pub))
	return f
}

func (f *archiveFixture) publish(t *testing.T, component *v1manifest.Component, filename, contents string) ([]string, error) {
	t.Helper()
	manifest, err := v1manifest.SignManifest(component, f.publisher)
	require.NoError(t, err)
	txn, err := store.New(f.root, "").Begin()
	require.NoError(t, err)
	recorded := &recordingTxn{FsTxn: txn}
	info := &PublishInfo{}
	if filename != "" {
		info.ComponentData = &TarInfo{Reader: strings.NewReader(contents), Name: filename}
	}
	err = New(recorded, f.keys).Publish(manifest, info)
	if err != nil {
		require.NoError(t, txn.Rollback())
	}
	return recorded.writes, err
}

func archiveComponent(id, ver, url, contents string) *v1manifest.Component {
	component := v1manifest.NewComponent(id, "test archive", time.Now())
	component.Platforms["linux/amd64"] = map[string]v1manifest.VersionItem{
		ver: {URL: url, FileHash: v1manifest.FileHash{Hashes: map[string]string{"sha256": fmt.Sprintf("%x", sha256.Sum256([]byte(contents)))}}},
	}
	return component
}

func TestPublishArchiveBinding(t *testing.T) {
	for _, tc := range []struct {
		name, url, filename string
	}{
		{name: "root metadata", url: "/root.json", filename: "root.json"},
		{name: "index metadata", url: "/2.index.json", filename: "2.index.json"},
		{name: "snapshot metadata", url: "/snapshot.json", filename: "snapshot.json"},
		{name: "timestamp metadata", url: "/timestamp.json", filename: "timestamp.json"},
		{name: "other component", url: "/other-v1.0.0-linux-amd64.tar.gz", filename: "other-v1.0.0-linux-amd64.tar.gz"},
		{name: "empty URL", filename: "example-v1.0.0-linux-amd64.tar.gz"},
		{name: "missing slash", url: "Xexample-v1.0.0-linux-amd64.tar.gz", filename: "example-v1.0.0-linux-amd64.tar.gz"},
		{name: "metadata-only claim", url: "/other-v1.0.0-linux-amd64.tar.gz"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			f := newArchiveFixture(t)
			component := archiveComponent("example", "v1.0.0", tc.url, "inert test marker")
			var err error
			var writes []string
			require.NotPanics(t, func() { writes, err = f.publish(t, component, tc.filename, "inert test marker") })
			require.Error(t, err)
			require.Empty(t, writes, "invalid archive requests must fail before staging metadata or data")
		})
	}
}

func TestPublishArchiveReplacement(t *testing.T) {
	for _, ver := range []string{"v1.0.0", "v1.0.0-rc.1+staging", "nightly"} {
		t.Run(ver, func(t *testing.T) {
			f := newArchiveFixture(t)
			filename := "example-" + ver + "-linux-amd64.tar.gz"
			component := archiveComponent("example", ver, "/"+filename, "first marker")
			_, err := f.publish(t, component, filename, "first marker")
			require.NoError(t, err)
			component = archiveComponent("example", ver, "/"+filename, "second marker")
			component.Version = 2
			_, err = f.publish(t, component, filename, "second marker")
			require.NoError(t, err)
			contents, err := os.ReadFile(filepath.Join(f.root, filename))
			require.NoError(t, err)
			require.Equal(t, "second marker", string(contents))
			component.Version++
			_, err = f.publish(t, component, "", "")
			require.NoError(t, err)
		})
	}
}

func TestPublishArchiveClaims(t *testing.T) {
	for _, metadataOnly := range []bool{false, true} {
		t.Run(fmt.Sprintf("ambiguous name metadata-only=%t", metadataOnly), func(t *testing.T) {
			f := newArchiveFixture(t)
			filename := "example-v1.0.0-v2.0.0-linux-amd64.tar.gz"
			first := archiveComponent("example", "v1.0.0-v2.0.0", "/"+filename, "first marker")
			_, err := f.publish(t, first, filename, "first marker")
			require.NoError(t, err)
			second := archiveComponent("example-v1.0.0", "v2.0.0", "/"+filename, "second marker")
			upload := filename
			if metadataOnly {
				upload = ""
			}
			writes, err := f.publish(t, second, upload, "second marker")
			require.Error(t, err)
			require.Empty(t, writes)
			contents, err := os.ReadFile(filepath.Join(f.root, filename))
			require.NoError(t, err)
			require.Equal(t, "first marker", string(contents))
		})
	}
	t.Run("component case alias", func(t *testing.T) {
		f := newArchiveFixture(t)
		_, err := f.publish(t, v1manifest.NewComponent("example", "first", time.Now()), "", "")
		require.NoError(t, err)
		writes, err := f.publish(t, v1manifest.NewComponent("Example", "second", time.Now()), "", "")
		require.Error(t, err)
		require.Empty(t, writes)
	})
}

func TestPublishRetiredArchiveClaims(t *testing.T) {
	for _, metadataOnly := range []bool{false, true} {
		t.Run(fmt.Sprintf("metadata-only=%t", metadataOnly), func(t *testing.T) {
			f := newArchiveFixture(t)
			const filename = "example-v1.0.0-v2.0.0-linux-amd64.tar.gz"
			first := archiveComponent("example", "v1.0.0-v2.0.0", "/"+filename, "retained marker")
			_, err := f.publish(t, first, filename, "retained marker")
			require.NoError(t, err)
			retired := v1manifest.NewComponent("example", "archive reference retired", time.Now())
			retired.Version = 2
			_, err = f.publish(t, retired, "", "")
			require.NoError(t, err)
			second := archiveComponent("example-v1.0.0", "v2.0.0", "/"+filename, "replacement marker")
			upload := filename
			if metadataOnly {
				upload = ""
			}
			writes, err := f.publish(t, second, upload, "replacement marker")
			require.Error(t, err)
			require.Empty(t, writes, "metadata must not establish ownership of an existing unclaimed file")
			contents, err := os.ReadFile(filepath.Join(f.root, filename))
			require.NoError(t, err)
			require.Equal(t, "retained marker", string(contents))
		})
	}
}

func TestPublishUnclaimedTiUPAlias(t *testing.T) {
	for _, metadataOnly := range []bool{false, true} {
		t.Run(fmt.Sprintf("metadata-only=%t", metadataOnly), func(t *testing.T) {
			f := newArchiveFixture(t)
			const filename = "tiup-v1.0.0-linux-amd64.tar.gz"
			const alias = "tiup-linux-amd64.tar.gz"
			require.NoError(t, os.WriteFile(filepath.Join(f.root, alias), []byte("retained alias"), 0600))
			component := archiveComponent("tiup", "v1.0.0", "/"+filename, "replacement marker")
			upload := filename
			if metadataOnly {
				upload = ""
			}
			writes, err := f.publish(t, component, upload, "replacement marker")
			require.Error(t, err)
			require.Empty(t, writes)
			contents, err := os.ReadFile(filepath.Join(f.root, alias))
			require.NoError(t, err)
			require.Equal(t, "retained alias", string(contents))
		})
	}
}

func TestPublishLegacyArchiveClaims(t *testing.T) {
	const filename = "example-v1.0.0-linux-amd64.tar.gz"
	for _, resource := range []string{"/" + filename, filename, "/./" + filename, "/directory/../" + filename, "../" + filename, "/" + strings.ToUpper(filename), "/example%2Dv1.0.0-linux-amd64.tar.gz"} {
		t.Run(resource, func(t *testing.T) {
			f := newArchiveFixture(t)
			_, err := f.publish(t, v1manifest.NewComponent("legacy", "first", time.Now()), "", "")
			require.NoError(t, err)
			legacy := archiveComponent("legacy", "v1.0.0", resource, "legacy marker")
			// Seed an old, already-committed custom URL without routing it through the
			// new publish policy. The fixture contains only inert local marker bytes.
			manifest, err := v1manifest.SignManifest(legacy, f.publisher)
			require.NoError(t, err)
			txn, err := store.New(f.root, "").Begin()
			require.NoError(t, err)
			require.NoError(t, txn.WriteManifest("1.legacy.json", manifest))
			require.NoError(t, txn.Write(filename, strings.NewReader("legacy marker")))
			require.NoError(t, txn.Commit())
			legacy.Version++
			_, err = f.publish(t, legacy, "", "")
			require.NoError(t, err, "unchanged legacy metadata stays publishable")
			other := archiveComponent("example", "v1.0.0", "/"+filename, "new marker")
			writes, err := f.publish(t, other, filename, "new marker")
			require.Error(t, err)
			require.Empty(t, writes)
			contents, err := os.ReadFile(filepath.Join(f.root, filename))
			require.NoError(t, err)
			require.Equal(t, "legacy marker", string(contents))
		})
	}
}

func TestPublishTiUPArchiveAlias(t *testing.T) {
	for _, reserved := range []bool{false, true} {
		t.Run(fmt.Sprintf("reserved=%t", reserved), func(t *testing.T) {
			f := newArchiveFixture(t)
			const alias = "tiup-linux-amd64.tar.gz"
			if reserved {
				_, err := f.publish(t, v1manifest.NewComponent("legacy", "first", time.Now()), "", "")
				require.NoError(t, err)
				legacy := archiveComponent("legacy", "v1.0.0", "/"+alias, "legacy marker")
				manifest, err := v1manifest.SignManifest(legacy, f.publisher)
				require.NoError(t, err)
				txn, err := store.New(f.root, "").Begin()
				require.NoError(t, err)
				require.NoError(t, txn.WriteManifest("1.legacy.json", manifest))
				require.NoError(t, txn.Write(alias, strings.NewReader("legacy marker")))
				require.NoError(t, txn.Commit())
			}
			const filename = "tiup-v1.0.0-rc.1+staging-linux-amd64.tar.gz"
			component := archiveComponent("tiup", "v1.0.0-rc.1+staging", "/"+filename, "tiup marker")
			writes, err := f.publish(t, component, filename, "tiup marker")
			var wantContents string
			if reserved {
				require.Error(t, err)
				require.Empty(t, writes)
				wantContents = "legacy marker"
			} else {
				require.NoError(t, err)
				const nextFilename = "tiup-v1.0.1-linux-amd64.tar.gz"
				next := archiveComponent("tiup", "v1.0.1", "/"+nextFilename, "updated tiup marker")
				next.Version = 2
				_, err = f.publish(t, next, nextFilename, "updated tiup marker")
				require.NoError(t, err, "the existing tiup owner can update its generated alias")
				wantContents = "updated tiup marker"
			}
			contents, err := os.ReadFile(filepath.Join(f.root, alias))
			require.NoError(t, err)
			require.Equal(t, wantContents, string(contents))
		})
	}
}
