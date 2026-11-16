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
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/pingcap/tiup/pkg/localdata"
	"github.com/pingcap/tiup/pkg/repository/store"
	"github.com/pingcap/tiup/pkg/repository/v1manifest"
	"github.com/stretchr/testify/require"
)

type recordingTxn struct {
	store.FsTxn
	writes []string
}

func (t *recordingTxn) Write(name string, r io.Reader) error {
	t.writes = append(t.writes, name)
	return t.FsTxn.Write(name, r)
}

func (t *recordingTxn) WriteManifest(name string, manifest *v1manifest.Manifest) error {
	t.writes = append(t.writes, name)
	return t.FsTxn.WriteManifest(name, manifest)
}

func TestPublishValidationBeforeWrites(t *testing.T) {
	base := t.TempDir()
	t.Setenv(localdata.EnvNameComponentDataDir, base)
	root, keyDir := filepath.Join(base, "mirror"), filepath.Join(base, "keys")
	require.NoError(t, os.Mkdir(root, 0700))
	require.NoError(t, v1manifest.Init(root, keyDir, time.Now()))
	keys := make(map[string]*v1manifest.KeyInfo)
	files, err := os.ReadDir(keyDir)
	require.NoError(t, err)
	for _, file := range files {
		contents, err := os.ReadFile(filepath.Join(keyDir, file.Name()))
		require.NoError(t, err)
		var key v1manifest.KeyInfo
		require.NoError(t, json.Unmarshal(contents, &key))
		id, err := key.ID()
		require.NoError(t, err)
		keys[id] = &key
	}
	publisher, err := v1manifest.GenKeyInfo()
	require.NoError(t, err)
	pub, err := publisher.Public()
	require.NoError(t, err)
	st := store.New(root, "")
	txn, err := st.Begin()
	require.NoError(t, err)
	require.NoError(t, New(txn, keys).Grant("publisher", "test publisher", pub))
	keyID, err := publisher.ID()
	require.NoError(t, err)

	publish := func(comp *v1manifest.Component, info *PublishInfo) {
		t.Helper()
		manifest, err := v1manifest.SignManifest(comp, publisher)
		require.NoError(t, err)
		txn, err := st.Begin()
		require.NoError(t, err)
		require.NoError(t, New(txn, keys).Publish(manifest, info))
	}
	publish(v1manifest.NewComponent("existing", "original", time.Now()), &PublishInfo{})

	for _, tc := range []struct {
		name, id string
		version  uint
		sigs     []v1manifest.Signature
		sign     bool
	}{
		{name: "missing owner", id: "new", version: 1},
		{name: "invalid new signature", id: "new", version: 1, sigs: []v1manifest.Signature{{KeyID: keyID, Sig: "invalid"}}},
		{name: "invalid existing signature", id: "existing", version: 2, sigs: []v1manifest.Signature{{KeyID: keyID, Sig: "invalid"}}},
		{name: "traversal", id: "x/../../victim", version: 1, sign: true},
		{name: "reserved root", id: "root", version: 2, sign: true},
		{name: "reserved index", id: "index", version: 4, sign: true},
		{name: "reserved snapshot", id: "snapshot", version: 1, sign: true},
		{name: "reserved timestamp", id: "timestamp", version: 1, sign: true},
		{name: "reserved case alias", id: "ROOT", version: 1, sign: true},
		{name: "empty id", id: "", version: 1, sign: true},
		{name: "absolute id", id: "/victim", version: 1, sign: true},
		{name: "backslash id", id: `x\victim`, version: 1, sign: true},
		{name: "url encoded separators", id: "x%2f..%2fvictim", version: 1, sign: true},
		{name: "dot id", id: "..", version: 1, sign: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			caseRoot := filepath.Join(t.TempDir(), "mirror")
			require.NoError(t, os.CopyFS(caseRoot, os.DirFS(root)))
			txn, err := store.New(caseRoot, "").Begin()
			require.NoError(t, err)
			defer func() { require.NoError(t, txn.Rollback()) }()
			comp := v1manifest.NewComponent(tc.id, "rejected", time.Now())
			comp.Version = tc.version
			recorded := &recordingTxn{FsTxn: txn}
			manifest := &v1manifest.Manifest{Signed: comp, Signatures: tc.sigs}
			if tc.sign {
				manifest, err = v1manifest.SignManifest(comp, publisher)
				require.NoError(t, err)
			}
			require.NotPanics(t, func() { err = New(recorded, keys).Publish(manifest, &PublishInfo{}) })
			require.Error(t, err)
			require.Empty(t, recorded.writes, "invalid publish must not write even inside the staging directory")
		})
	}

	t.Run("signed metadata-only update", func(t *testing.T) {
		comp := v1manifest.NewComponent("existing", "updated", time.Now())
		comp.Version = 2
		hidden := true
		publish(comp, &PublishInfo{Hide: &hidden})
		txn, err := st.Begin()
		require.NoError(t, err)
		defer func() { require.NoError(t, txn.Rollback()) }()
		manifest, err := txn.ReadManifest("2.existing.json", &v1manifest.Component{})
		require.NoError(t, err)
		require.Equal(t, "updated", manifest.Signed.(*v1manifest.Component).Description)
	})

	t.Run("only current owner can authorize a transfer", func(t *testing.T) {
		recipient, err := v1manifest.GenKeyInfo()
		require.NoError(t, err)
		public, err := recipient.Public()
		require.NoError(t, err)
		txn, err := st.Begin()
		require.NoError(t, err)
		require.NoError(t, New(txn, keys).Grant("recipient", "test recipient", public))
		comp := v1manifest.NewComponent("existing", "transfer", time.Now())
		comp.Version = 3
		manifest, err := v1manifest.SignManifest(comp, recipient)
		require.NoError(t, err)
		txn, err = st.Begin()
		require.NoError(t, err)
		recorded := &recordingTxn{FsTxn: txn}
		require.Error(t, New(recorded, keys).Publish(manifest, &PublishInfo{Owner: "recipient"}))
		require.Empty(t, recorded.writes)
		require.NoError(t, txn.Rollback())
		publish(comp, &PublishInfo{Owner: "recipient"})
		comp.Version++
		manifest, err = v1manifest.SignManifest(comp, recipient)
		require.NoError(t, err)
		txn, err = st.Begin()
		require.NoError(t, err)
		require.NoError(t, New(txn, keys).Publish(manifest, &PublishInfo{}))
	})
}
