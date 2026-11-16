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
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/pingcap/tiup/pkg/localdata"
	"github.com/pingcap/tiup/pkg/repository/v1manifest"
	"github.com/stretchr/testify/require"
)

func TestTransactionPathsConfined(t *testing.T) {
	base := t.TempDir()
	t.Setenv(localdata.EnvNameComponentDataDir, base)
	root := filepath.Join(base, "mirror")
	require.NoError(t, os.Mkdir(root, 0700))
	target := filepath.Join(base, "outside.json")
	marker, err := json.Marshal(&v1manifest.Manifest{Signed: &v1manifest.Timestamp{}})
	require.NoError(t, err)
	var requests atomic.Int32
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests.Add(1)
		http.NotFound(w, r)
	}))
	defer upstream.Close()

	operations := map[string]func(FsTxn, string) error{
		"write": func(txn FsTxn, name string) error { return txn.Write(name, strings.NewReader("replacement")) },
		"write manifest": func(txn FsTxn, name string) error {
			return txn.WriteManifest(name, &v1manifest.Manifest{Signed: &v1manifest.Timestamp{}})
		},
		"read": func(txn FsTxn, name string) error {
			r, err := txn.Read(name)
			if err == nil {
				require.NoError(t, r.Close())
			}
			return err
		},
		"read manifest": func(txn FsTxn, name string) error {
			_, err := txn.ReadManifest(name, &v1manifest.Timestamp{})
			return err
		},
		"stat": func(txn FsTxn, name string) error { _, err := txn.Stat(name); return err },
	}
	for operation, run := range operations {
		for _, name := range []string{"../outside.json", target, "nested/file.json", `..\outside.json`, "", ".", ".."} {
			t.Run(operation+"/"+name, func(t *testing.T) {
				require.NoError(t, os.WriteFile(target, marker, 0600))
				txn, err := New(root, upstream.URL).Begin()
				require.NoError(t, err)
				defer func() { require.NoError(t, txn.Rollback()) }()
				require.Error(t, run(txn, name))
				contents, err := os.ReadFile(target)
				require.NoError(t, err)
				require.Equal(t, marker, contents)
			})
		}
	}
	require.Zero(t, requests.Load(), "invalid paths must not fall back to upstream")
}

func TestTransactionRejectsSymlinks(t *testing.T) {
	base := t.TempDir()
	t.Setenv(localdata.EnvNameComponentDataDir, base)
	root := filepath.Join(base, "mirror")
	require.NoError(t, os.Mkdir(root, 0700))
	target := filepath.Join(base, "outside.json")
	require.NoError(t, os.WriteFile(target, []byte("inert marker"), 0600))
	for kind, destination := range map[string]string{
		"absolute outside": target,
		"relative outside": "../outside.json",
		"inside":           "regular.json",
		"dangling":         "missing.json",
	} {
		for _, location := range []string{"staging", "repository"} {
			t.Run(kind+"/"+location, func(t *testing.T) {
				txn, err := New(root, "").Begin()
				require.NoError(t, err)
				defer func() { require.NoError(t, txn.Rollback()) }()
				linkRoot := root
				if location == "staging" {
					linkRoot = txn.(*localTxn).root
				}
				link := filepath.Join(linkRoot, "link.json")
				require.NoError(t, os.WriteFile(filepath.Join(linkRoot, "regular.json"), []byte("inside marker"), 0600))
				require.NoError(t, os.Symlink(destination, link))
				defer func() { require.NoError(t, os.Remove(link)) }()
				reader, err := txn.Read("link.json")
				if err == nil {
					require.NoError(t, reader.Close())
				}
				require.Error(t, err)
				_, err = txn.ReadManifest("link.json", &v1manifest.Timestamp{})
				require.Error(t, err)
				_, err = txn.Stat("link.json")
				require.Error(t, err)
				if location == "staging" {
					require.Error(t, txn.Write("link.json", strings.NewReader("replacement")))
					require.Error(t, txn.WriteManifest("link.json", &v1manifest.Manifest{Signed: &v1manifest.Timestamp{}}))
				} else {
					require.NoError(t, txn.Write("link.json", strings.NewReader("replacement")))
				}
				require.Error(t, txn.Commit())
				contents, err := os.ReadFile(target)
				require.NoError(t, err)
				require.Equal(t, "inert marker", string(contents))
			})
		}
	}
}

func TestCommitPreservesFileModeAndMonotonicTime(t *testing.T) {
	base := t.TempDir()
	t.Setenv(localdata.EnvNameComponentDataDir, base)
	root := filepath.Join(base, "mirror")
	require.NoError(t, os.Mkdir(root, 0700))
	txn, err := New(root, "").Begin()
	require.NoError(t, err)
	const name = "package-v1.0.0+build_linux-amd64.tar.gz"
	require.NoError(t, txn.Write(name, strings.NewReader("inert package")))
	staged := filepath.Join(txn.(*localTxn).root, name)
	require.NoError(t, os.Chmod(staged, 0600))
	mt := time.Now().Add(time.Second)
	require.NoError(t, os.Chtimes(staged, mt, mt))
	require.NoError(t, txn.Commit())
	fi, err := os.Stat(filepath.Join(root, name))
	require.NoError(t, err)
	require.Equal(t, os.FileMode(0600), fi.Mode().Perm())
	require.False(t, fi.ModTime().Before(mt))
}
