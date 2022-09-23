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
	"bytes"
	"context"
	"fmt"
	"io"
	"os"
	"path"
	"time"

	cjson "github.com/gibson042/canonicaljson-go"
	"github.com/pingcap/errors"
	"github.com/pingcap/tiup/pkg/localdata"
	"github.com/pingcap/tiup/pkg/repository/v1manifest"
	"github.com/pingcap/tiup/pkg/utils"
)

var (
	// ErrorFsCommitConflict indicates concurrent writing file
	ErrorFsCommitConflict = errors.New("conflict on fs commit")
)

// The localTxn is used to implment a filesystem transaction, the basic principle is to
// record the timestamp before access any manifest file, and when writing them back, check
// if the origin files' timestamp is newer than recorded one, if so, a conflict is detected.
//
// To get current timestamp:
// 1. check if there is timestamp.json in root directory, if so, return it's modify time
// 2. check if there is any file in root directory, if so, return the newest one's modify time
// 3. return the root directory's modify time
// If the timestamp.json is in root directory, we always make sure it's newer than other files
// So current timestamp is the modify time of the newest file in root directory
//
// To read a manifest file:
// 1. get current timestamp and record it
// 2. read the file from root directory
//
// To write a manifest file:
// 1. get current timestamp and record it
// 2. write the manifest file to temporary directory
//
// To commit a localTxn:
//  1. for every accessed file, get the recorded timestamp
//  2. for every accessed file, get the current modify time of their origin file in root directory
//  3. check if the origin file is newer thant recorded timestamp, if so, there must be conflict
//  4. copy every file in temporary directory to root directory, if there is a timestamp.json in
//     temporary directory, it should be the last one to copy
type localTxn struct {
	syncer   Syncer
	store    *localStore
	root     string
	accessed map[string]*time.Time
}

func newLocalTxn(store *localStore) (*localTxn, error) {
	syncer := newFsSyncer(path.Join(store.root, "commits"))
	if script := os.Getenv(localdata.EnvNameMirrorSyncScript); script != "" {
		syncer = combine(syncer, newExternalSyncer(script))
	}
	root, err := os.MkdirTemp(os.Getenv(localdata.EnvNameComponentDataDir), "tiup-commit-*")
	if err != nil {
		return nil, err
	}
	txn := &localTxn{
		syncer:   syncer,
		store:    store,
		root:     root,
		accessed: make(map[string]*time.Time),
	}

	return txn, nil
}

// Write implements FsTxn
func (t *localTxn) Write(filename string, reader io.Reader) error {
	filepath := path.Join(t.root, filename)
	file, err := os.Create(filepath)
	if err != nil {
		return errors.Annotate(err, "create file")
	}
	defer file.Close()

	_, err = io.Copy(file, reader)
	return err
}

// Read implements FsTxn
func (t *localTxn) Read(filename string) (io.ReadCloser, error) {
	filepath := t.store.path(filename)
	if utils.IsExist(path.Join(t.root, filename)) {
		filepath = path.Join(t.root, filename)
	}

	return os.Open(filepath)
}

func (t *localTxn) WriteManifest(filename string, manifest *v1manifest.Manifest) error {
	if err := t.access(filename); err != nil {
		return err
	}
	filepath := path.Join(t.root, filename)
	file, err := os.Create(filepath)
	if err != nil {
		return errors.Annotate(err, "create file")
	}
	defer file.Close()

	bytes, err := cjson.Marshal(manifest)
	if err != nil {
		return errors.Annotate(err, "marshal manifest")
	}

	if _, err = file.Write(bytes); err != nil {
		return errors.Annotate(err, "write file")
	}

	if err = file.Close(); err != nil {
		return errors.Annotate(err, "flush file content")
	}

	fi, err := os.Stat(filepath)
	if err != nil {
		return errors.Annotate(err, "stat file")
	}
	// The modify time must increase
	if !t.first(filename).Before(fi.ModTime()) {
		mt := time.Unix(0, t.first(filename).UnixNano()+1)
		return os.Chtimes(filepath, mt, mt)
	}

	return nil
}

func (t *localTxn) ReadManifest(filename string, role v1manifest.ValidManifest) (*v1manifest.Manifest, error) {
	if err := t.access(filename); err != nil {
		return nil, err
	}
	filepath := t.store.path(filename)
	if utils.IsExist(path.Join(t.root, filename)) {
		filepath = path.Join(t.root, filename)
	}
	var wc io.Reader
	file, err := os.Open(filepath)
	switch {
	case err == nil:
		wc = file
		defer file.Close()
	case os.IsNotExist(err) && t.store.upstream != "":
		url := fmt.Sprintf("%s/%s", t.store.upstream, filename)
		client := utils.NewHTTPClient(time.Minute, nil)
		body, err := client.Get(context.TODO(), url)
		if err != nil {
			return nil, errors.Annotatef(err, "fetch %s", url)
		}
		wc = bytes.NewBuffer(body)
	default:
		return nil, errors.Annotatef(err, "error on read manifest: %s, upstream %s", err.Error(), t.store.upstream)
	}

	return v1manifest.ReadNoVerify(wc, role)
}

func (t *localTxn) ResetManifest() error {
	for file := range t.accessed {
		fp := path.Join(t.root, file)
		if utils.IsExist(fp) {
			if err := os.Remove(fp); err != nil {
				return err
			}
		}
	}
	t.accessed = make(map[string]*time.Time)
	return nil
}

func (t *localTxn) Stat(filename string) (os.FileInfo, error) {
	if err := t.access(filename); err != nil {
		return nil, err
	}
	filepath := t.store.path(filename)
	if utils.IsExist(path.Join(t.root, filename)) {
		filepath = path.Join(t.root, filename)
	}
	return os.Stat(filepath)
}

func (t *localTxn) Commit() error {
	if err := t.store.lock(); err != nil {
		return err
	}
	defer t.store.unlock()

	if err := t.checkConflict(); err != nil {
		return err
	}

	files, err := os.ReadDir(t.root)
	if err != nil {
		return err
	}

	hasTimestamp := false
	for _, f := range files {
		// Make sure modify time of the timestamp.json is the newest
		if f.Name() == v1manifest.ManifestFilenameTimestamp {
			hasTimestamp = true
			continue
		}
		if err := utils.Copy(path.Join(t.root, f.Name()), t.store.path(f.Name())); err != nil {
			return err
		}
	}
	if hasTimestamp {
		if err := utils.Copy(path.Join(t.root, v1manifest.ManifestFilenameTimestamp), t.store.path(v1manifest.ManifestFilenameTimestamp)); err != nil {
			return err
		}
	}

	if err := t.syncer.Sync(t.root); err != nil {
		return err
	}

	return t.release()
}

func (t *localTxn) Rollback() error {
	return t.release()
}

func (t *localTxn) checkConflict() error {
	for file := range t.accessed {
		mt, err := t.store.last(file)
		if err != nil {
			return err
		}
		if mt != nil && mt.After(*t.first(file)) {
			return ErrorFsCommitConflict
		}
	}
	return nil
}

func (t *localTxn) access(filename string) error {
	// Use the earliest time
	if t.accessed[filename] != nil {
		return nil
	}

	// Use the modify time of timestamp.json
	timestamp := t.store.path(v1manifest.ManifestFilenameTimestamp)
	fi, err := os.Stat(timestamp)
	if err == nil {
		mt := fi.ModTime()
		t.accessed[filename] = &mt
	} else if !os.IsNotExist(err) {
		return errors.Annotatef(err, "read %s: %s", v1manifest.ManifestFilenameTimestamp, timestamp)
	}

	// Use the newest file in t.store.root
	files, err := os.ReadDir(t.store.root)
	if err != nil {
		return errors.Annotatef(err, "read store root: %s", t.store.root)
	}
	for _, f := range files {
		fi, err := f.Info()
		if err != nil {
			return err
		}
		if t.accessed[filename] == nil || t.accessed[filename].Before(fi.ModTime()) {
			mt := fi.ModTime()
			t.accessed[filename] = &mt
		}
	}
	if t.accessed[filename] != nil {
		return nil
	}

	// Use the mod time of t.store.root
	fi, err = os.Stat(t.store.root)
	if err != nil {
		return errors.Annotatef(err, "read store root: %s", t.store.root)
	}
	mt := fi.ModTime()
	t.accessed[filename] = &mt
	return nil
}

// Returns the first access time
func (t *localTxn) first(filename string) *time.Time {
	return t.accessed[filename]
}

func (t *localTxn) release() error {
	return os.RemoveAll(t.root)
}
