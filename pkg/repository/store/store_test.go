// Copyright 2020 PingCAP, Inc.
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

package store

import (
	"os"
	"testing"

	"github.com/pingcap/tiup/pkg/repository/v1manifest"
	"github.com/stretchr/testify/assert"
)

func TestEmptyCommit(t *testing.T) {
	root, err := os.MkdirTemp("", "")
	assert.Nil(t, err)
	defer os.RemoveAll(root)

	store := New(root, "")
	txn, err := store.Begin()
	assert.Nil(t, err)
	err = txn.Commit()
	assert.Nil(t, err)
}

func TestSingleWrite(t *testing.T) {
	root, err := os.MkdirTemp("", "")
	assert.Nil(t, err)
	defer os.RemoveAll(root)

	store := New(root, "")
	txn, err := store.Begin()
	assert.Nil(t, err)
	err = txn.WriteManifest("test.json", &v1manifest.Manifest{
		Signed: &v1manifest.Timestamp{
			Meta: map[string]v1manifest.FileHash{
				"test": {
					Length: 9527,
				},
			},
		},
	})
	assert.Nil(t, err)
	m, err := txn.ReadManifest("test.json", &v1manifest.Timestamp{})
	assert.Nil(t, err)
	assert.Equal(t, uint(9527), m.Signed.(*v1manifest.Timestamp).Meta["test"].Length)
}

func TestConflict(t *testing.T) {
	root, err := os.MkdirTemp("", "")
	assert.Nil(t, err)
	defer os.RemoveAll(root)

	store := New(root, "")

	txn1, err := store.Begin()
	assert.Nil(t, err)
	txn2, err := store.Begin()
	assert.Nil(t, err)

	test := &v1manifest.Manifest{
		Signed: &v1manifest.Timestamp{
			Meta: map[string]v1manifest.FileHash{
				"test": {
					Length: 9527,
				},
			},
		},
	}
	err = txn1.WriteManifest("test.json", test)
	assert.Nil(t, err)
	m, err := txn1.ReadManifest("test.json", &v1manifest.Timestamp{})
	assert.Nil(t, err)
	assert.Equal(t, uint(9527), m.Signed.(*v1manifest.Timestamp).Meta["test"].Length)

	err = txn2.WriteManifest("test.json", test)
	assert.Nil(t, err)
	m, err = txn2.ReadManifest("test.json", &v1manifest.Timestamp{})
	assert.Nil(t, err)
	assert.Equal(t, uint(9527), m.Signed.(*v1manifest.Timestamp).Meta["test"].Length)

	err = txn1.Commit()
	assert.Nil(t, err)

	err = txn2.Commit()
	assert.NotNil(t, err)
}

func TestUpstream(t *testing.T) {
	root, err := os.MkdirTemp("", "")
	assert.Nil(t, err)
	defer os.RemoveAll(root)

	txn, err := New(root, "").Begin()
	assert.Nil(t, err)
	_, err = txn.ReadManifest("timestamp.json", &v1manifest.Timestamp{})
	assert.NotNil(t, err)

	txn, err = New(root, "https://tiup-mirrors.pingcap.com").Begin()
	assert.Nil(t, err)
	m, err := txn.ReadManifest("timestamp.json", &v1manifest.Timestamp{})
	assert.Nil(t, err)
	assert.NotEmpty(t, m.Signed.(*v1manifest.Timestamp).Meta["/snapshot.json"].Hashes)
}
