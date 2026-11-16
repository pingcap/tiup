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
	"path"
	"time"

	"github.com/gofrs/flock"
	"github.com/pingcap/errors"
)

type localStore struct {
	root     string
	upstream string
	flock    *flock.Flock
}

func newLocalStore(root, upstream string) *localStore {
	return &localStore{
		root:     root,
		upstream: upstream,
		flock:    flock.New(path.Join(root, "lock")),
	}
}

// Begin implements the Store
func (s *localStore) Begin() (FsTxn, error) {
	return newLocalTxn(s)
}

// Returns the last modify time
func (s *localStore) last(filename string) (*time.Time, error) {
	fi, err := statLocalFile(s.root, filename)
	if os.IsNotExist(err) {
		return nil, nil
	}
	if err != nil {
		return nil, errors.Annotate(err, "Stat file")
	}
	mt := fi.ModTime()
	return &mt, nil
}

func (s *localStore) lock() error {
	return s.flock.Lock()
}

func (s *localStore) unlock() {
	// The unlock operation must success, otherwise the later operation will stuck
	if err := s.flock.Unlock(); err != nil {
		panic(errors.Annotate(err, "unlock filesystem failed"))
	}
}
