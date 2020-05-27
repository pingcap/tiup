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
	"errors"
	"io"
	"io/ioutil"
	"os"
	"path"
	"sync"
	"time"

	cjson "github.com/gibson042/canonicaljson-go"
	"github.com/google/uuid"
	"github.com/pingcap-incubator/tiup/pkg/utils"
)

var (
	// ErrorFsCommitConflict indicates concurrent writing file
	ErrorFsCommitConflict = errors.New("conflict on fs commit")
)

type qcloudStore struct {
	mux      sync.Mutex
	root     string
	modified map[string]*time.Time
}

func newQCloudStore(root string) *qcloudStore {
	if err := os.MkdirAll(root, 0755); err != nil {
		// TODO: handle error here
	}
	return &qcloudStore{
		root:     root,
		modified: make(map[string]*time.Time),
	}
}

func (s *qcloudStore) Begin() (FsTxn, error) {
	return newQCloudTxn(s)
}

func (s *qcloudStore) path(filename string) string {
	return path.Join(s.root, filename)
}

func (s *qcloudStore) modify(filename string, t *time.Time) {
	s.modified[filename] = t
}

// Returns the last modify time
func (s *qcloudStore) last(filename string) *time.Time {
	return s.modified[filename]
}

func (s *qcloudStore) lock() {
	s.mux.Lock()
}

func (s *qcloudStore) unlock() {
	s.mux.Unlock()
}

type qcloudTxn struct {
	store    *qcloudStore
	root     string
	begin    time.Time
	accessed map[string]*time.Time
}

func newQCloudTxn(store *qcloudStore) (*qcloudTxn, error) {
	txn := &qcloudTxn{
		store:    store,
		root:     path.Join("/tmp", uuid.New().String()),
		begin:    time.Now(),
		accessed: make(map[string]*time.Time),
	}

	if err := txn.require(); err != nil {
		return nil, err
	}
	return txn, nil
}

func (t *qcloudTxn) Write(filename string, reader io.Reader) error {
	filepath := path.Join(t.root, filename)
	file, err := os.Create(filepath)
	if err != nil {
		return err
	}
	defer file.Close()

	_, err = io.Copy(file, reader)
	return err
}

func (t *qcloudTxn) Read(filename string) (io.ReadCloser, error) {
	filepath := t.store.path(filename)
	if utils.IsExist(path.Join(t.root, filename)) {
		filepath = path.Join(t.root, filename)
	}

	return os.Open(filepath)
}

func (t *qcloudTxn) WriteManifest(filename string, manifest interface{}) error {
	t.access(filename)
	filepath := path.Join(t.root, filename)
	file, err := os.Create(filepath)
	if err != nil {
		return err
	}
	defer file.Close()

	return cjson.NewEncoder(file).Encode(manifest)
}

func (t *qcloudTxn) ReadManifest(filename string, manifest interface{}) error {
	t.access(filename)
	filepath := t.store.path(filename)
	if utils.IsExist(path.Join(t.root, filename)) {
		filepath = path.Join(t.root, filename)
	}
	file, err := os.Open(filepath)
	if err != nil {
		return err
	}
	defer file.Close()

	return cjson.NewDecoder(file).Decode(manifest)
}

func (t *qcloudTxn) ResetManifest() error {
	for file := range t.accessed {
		if utils.IsExist(path.Join(t.root, file)) {
			if err := os.Remove(file); err != nil {
				return err
			}
		}
	}
	t.begin = time.Now()
	return nil
}

func (t *qcloudTxn) Stat(filename string) (os.FileInfo, error) {
	t.access(filename)
	filepath := t.store.path(filename)
	if utils.IsExist(path.Join(t.root, filename)) {
		filepath = path.Join(t.root, filename)
	}
	return os.Stat(filepath)
}

func (t *qcloudTxn) access(filename string) {
	// Use the earliest time
	if t.accessed[filename] != nil {
		return
	}

	at := time.Now()
	t.accessed[filename] = &at
}

// Returns the first access time
func (t *qcloudTxn) first(filename string) *time.Time {
	return t.accessed[filename]
}

func (t *qcloudTxn) Commit() error {
	t.store.lock()
	defer t.store.unlock()

	if err := t.checkConflict(); err != nil {
		return err
	}

	files, err := ioutil.ReadDir(t.root)
	if err != nil {
		return err
	}

	at := time.Now()
	for _, f := range files {
		if err := utils.Copy(path.Join(t.root, f.Name()), t.store.path(f.Name())); err != nil {
			return err
		}
		t.store.modify(f.Name(), &at)
	}

	// TODO: qshell upload
	return t.release()
}

func (t *qcloudTxn) checkConflict() error {
	for file := range t.accessed {
		if t.store.last(file) != nil && t.store.last(file).After(*t.first(file)) {
			return ErrorFsCommitConflict
		}
	}
	return nil
}

func (t *qcloudTxn) Rollback() error {
	return t.release()
}

func (t *qcloudTxn) require() (err error) {
	if err := os.MkdirAll(t.root, 0755); err != nil {
		return err
	}

	return nil
}

func (t *qcloudTxn) release() error {
	if err := os.RemoveAll(t.root); err != nil {
		return err
	}
	return nil
}
