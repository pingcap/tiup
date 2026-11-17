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
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"testing/iotest"
	"time"

	"github.com/pingcap/tiup/pkg/localdata"
	"github.com/pingcap/tiup/pkg/repository/store"
	"github.com/pingcap/tiup/pkg/repository/v1manifest"
	"github.com/stretchr/testify/require"
)

func TestPublishMetadataOnlyArchiveReservation(t *testing.T) {
	t.Setenv(localdata.EnvNameMirrorSyncScript, "")
	for _, metadataOnly := range []bool{false, true} {
		t.Run(fmt.Sprintf("claim metadata-only=%t", metadataOnly), func(t *testing.T) {
			f := newArchiveFixture(t)
			const filename = "example-v1.0.0-v2.0.0-linux-amd64.tar.gz"
			first := archiveComponent("example", "v1.0.0-v2.0.0", "/"+filename, "reserved marker")
			_, err := f.publish(t, first, "", "")
			require.NoError(t, err)
			_, err = os.Stat(filepath.Join(f.root, filename))
			require.True(t, os.IsNotExist(err), "the reservation must exist only in signed metadata")

			// A second registered publisher must not acquire the first publisher's
			// reservation merely because the corresponding file is not present yet.
			other, err := v1manifest.GenKeyInfo()
			require.NoError(t, err)
			public, err := other.Public()
			require.NoError(t, err)
			txn, err := store.New(f.root, "").Begin()
			require.NoError(t, err)
			require.NoError(t, New(txn, f.keys).Grant("other", "other publisher", public))
			f.publisher = other
			before, err := os.ReadFile(filepath.Join(f.root, v1manifest.ManifestFilenameSnapshot))
			require.NoError(t, err)

			second := archiveComponent("example-v1.0.0", "v2.0.0", "/"+filename, "replacement marker")
			upload := filename
			if metadataOnly {
				upload = ""
			}
			writes, err := f.publish(t, second, upload, "replacement marker")
			require.ErrorIs(t, err, ErrorWrongFileName)
			require.Empty(t, writes)
			_, err = os.Stat(filepath.Join(f.root, filename))
			require.True(t, os.IsNotExist(err))
			after, err := os.ReadFile(filepath.Join(f.root, v1manifest.ManifestFilenameSnapshot))
			require.NoError(t, err)
			require.Equal(t, before, after, "the rejected claim must not advance repository metadata")
		})
	}
}

func TestPublishMalformedArchiveTuple(t *testing.T) {
	t.Setenv(localdata.EnvNameMirrorSyncScript, "")
	for _, tc := range []struct {
		name, platform, version, filename string
	}{
		{name: "missing OS", platform: "/amd64", version: "v1.0.0", filename: "example-v1.0.0--amd64.tar.gz"},
		{name: "missing architecture", platform: "linux/", version: "v1.0.0", filename: "example-v1.0.0-linux-.tar.gz"},
		{name: "extra platform segment", platform: "linux/amd64/extra", version: "v1.0.0", filename: "example-v1.0.0-linux-amd64-extra.tar.gz"},
		{name: "ambiguous OS separator", platform: "linux-extra/amd64", version: "v1.0.0", filename: "example-v1.0.0-linux-extra-amd64.tar.gz"},
		{name: "missing version", platform: "linux/amd64", filename: "example--linux-amd64.tar.gz"},
		{name: "version traversal", platform: "linux/amd64", version: "../v1.0.0", filename: "example-../v1.0.0-linux-amd64.tar.gz"},
		{name: "encoded separator", platform: "linux/amd64", version: "v1.0.0%2fextra", filename: "example-v1.0.0%2fextra-linux-amd64.tar.gz"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			for _, metadataOnly := range []bool{false, true} {
				t.Run(fmt.Sprintf("metadata-only=%t", metadataOnly), func(t *testing.T) {
					f := newArchiveFixture(t)
					component := v1manifest.NewComponent("example", "malformed tuple", time.Now())
					component.Platforms[tc.platform] = archiveComponent("example", tc.version, "/"+tc.filename, "inert marker").Platforms["linux/amd64"]
					upload := tc.filename
					if metadataOnly {
						upload = ""
					}
					var writes []string
					var err error
					require.NotPanics(t, func() { writes, err = f.publish(t, component, upload, "inert marker") })
					require.ErrorIs(t, err, ErrorWrongFileName)
					require.Empty(t, writes, "tuple validation must precede all staged writes")
				})
			}
		})
	}
}

func TestPublishLegacyArchiveMigration(t *testing.T) {
	t.Setenv(localdata.EnvNameMirrorSyncScript, "")
	f := newArchiveFixture(t)
	_, err := f.publish(t, v1manifest.NewComponent("example", "legacy component", time.Now()), "", "")
	require.NoError(t, err)
	const legacyFilename = "historical-package.tar.gz"
	legacy := archiveComponent("example", "v1.0.0", "/"+legacyFilename, "legacy marker")
	manifest, err := v1manifest.SignManifest(legacy, f.publisher)
	require.NoError(t, err)
	txn, err := store.New(f.root, "").Begin()
	require.NoError(t, err)
	require.NoError(t, txn.WriteManifest("1.example.json", manifest))
	require.NoError(t, txn.Write(legacyFilename, strings.NewReader("legacy marker")))
	require.NoError(t, txn.Commit())

	legacy.Version = 2
	writes, err := f.publish(t, legacy, legacyFilename, "replacement marker")
	require.ErrorIs(t, err, ErrorWrongFileName, "retaining an old URL does not authorize an upload to it")
	require.Empty(t, writes)
	contents, err := os.ReadFile(filepath.Join(f.root, legacyFilename))
	require.NoError(t, err)
	require.Equal(t, "legacy marker", string(contents))

	const newFilename = "example-v2.0.0-linux-amd64.tar.gz"
	newVersion := archiveComponent("example", "v2.0.0", "/"+newFilename, "new marker")
	legacy.Platforms["linux/amd64"]["v2.0.0"] = newVersion.Platforms["linux/amd64"]["v2.0.0"]
	_, err = f.publish(t, legacy, newFilename, "new marker")
	require.NoError(t, err, "a canonical release must coexist with unchanged historical URLs")
	for filename, want := range map[string]string{legacyFilename: "legacy marker", newFilename: "new marker"} {
		contents, err := os.ReadFile(filepath.Join(f.root, filename))
		require.NoError(t, err)
		require.Equal(t, want, string(contents))
	}
	readTxn, err := store.New(f.root, "").Begin()
	require.NoError(t, err)
	defer func() { require.NoError(t, readTxn.Rollback()) }()
	stored, err := readTxn.ReadManifest("2.example.json", &v1manifest.Component{})
	require.NoError(t, err)
	require.Equal(t, legacy.Platforms, stored.Signed.(*v1manifest.Component).Platforms)
}

func TestPublishArchiveFailureRollback(t *testing.T) {
	t.Setenv(localdata.EnvNameMirrorSyncScript, "")
	readErr := errors.New("inert upload read failure")
	for _, tc := range []struct {
		name    string
		reader  func() io.Reader
		wantErr error
	}{
		{name: "checksum mismatch", reader: func() io.Reader { return strings.NewReader("corrupt marker") }, wantErr: ErrorWrongChecksum},
		{name: "partial upload", reader: func() io.Reader {
			return io.MultiReader(strings.NewReader("partial marker"), iotest.ErrReader(readErr))
		}, wantErr: readErr},
	} {
		t.Run(tc.name, func(t *testing.T) {
			f := newArchiveFixture(t)
			const filename = "example-nightly-linux-amd64.tar.gz"
			first := archiveComponent("example", "nightly", "/"+filename, "published marker")
			_, err := f.publish(t, first, filename, "published marker")
			require.NoError(t, err)
			before := archiveRepositoryFiles(t, f.root)

			next := archiveComponent("example", "nightly", "/"+filename, "new marker")
			next.Version = 2
			manifest, err := v1manifest.SignManifest(next, f.publisher)
			require.NoError(t, err)
			txn, err := store.New(f.root, "").Begin()
			require.NoError(t, err)
			err = New(txn, f.keys).Publish(manifest, &PublishInfo{ComponentData: &TarInfo{Name: filename, Reader: tc.reader()}})
			require.ErrorIs(t, err, tc.wantErr)
			require.Equal(t, before, archiveRepositoryFiles(t, f.root), "failed uploads must not expose staged metadata or partial archives")
			require.NoError(t, txn.Rollback())
			require.Equal(t, before, archiveRepositoryFiles(t, f.root), "rollback must preserve every committed repository file")

			// Retrying the same manifest version proves the failed request did not
			// consume its version or prevent a legitimate nightly replacement.
			_, err = f.publish(t, next, filename, "new marker")
			require.NoError(t, err)
			contents, err := os.ReadFile(filepath.Join(f.root, filename))
			require.NoError(t, err)
			require.Equal(t, "new marker", string(contents))
		})
	}
}

func archiveRepositoryFiles(t *testing.T, root string) map[string][]byte {
	t.Helper()
	files := make(map[string][]byte)
	require.NoError(t, filepath.WalkDir(root, func(name string, entry os.DirEntry, err error) error {
		if err != nil || !entry.Type().IsRegular() {
			return err
		}
		files[name], err = os.ReadFile(name)
		return err
	}))
	return files
}
