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

package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"sync"

	"github.com/pingcap/tiup/pkg/repository/v1manifest"
	"github.com/pingcap/tiup/server/session"
	"github.com/pingcap/tiup/server/store"
)

type server struct {
	root     string
	upstream string
	keys     map[string]*v1manifest.KeyInfo
	sm       session.Manager
}

// NewServer returns a pointer to server
func newServer(rootDir, upstream, indexKey, snapshotKey, timestampKey string) (*server, error) {
	s := &server{
		root:     rootDir,
		upstream: upstream,
		keys:     make(map[string]*v1manifest.KeyInfo),
		sm:       session.New(store.NewStore(rootDir, upstream), new(sync.Map)),
	}

	kmap := map[string]string{
		v1manifest.ManifestTypeIndex:     indexKey,
		v1manifest.ManifestTypeSnapshot:  snapshotKey,
		v1manifest.ManifestTypeTimestamp: timestampKey,
	}

	for ty, kfile := range kmap {
		if kfile == "" {
			continue
		}
		k, err := loadPrivateKey(kfile)
		if err != nil {
			return nil, err
		}
		s.keys[ty] = k
	}

	return s, nil
}

func (s *server) run(addr string) error {
	fmt.Println(addr)
	return http.ListenAndServe(addr, s.router())
}

func loadPrivateKey(keyFile string) (*v1manifest.KeyInfo, error) {
	var key v1manifest.KeyInfo
	f, err := os.Open(keyFile)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	if err := json.NewDecoder(f).Decode(&key); err != nil {
		return nil, err
	}

	// Check if key is valid
	_, err = key.ID()
	if err != nil {
		return nil, err
	}

	return &key, nil
}
