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
	"fmt"
	"net/http"

	"github.com/pingcap/tiup/pkg/repository"
	"github.com/pingcap/tiup/server/session"
)

type server struct {
	mirror   repository.Mirror
	sm       session.Manager
	upstream string
}

// NewServer returns a pointer to server
func newServer(rootDir, keyDir, upstream string) (*server, error) {
	mirror := repository.NewMirror(rootDir, keyDir, repository.MirrorOptions{Upstream: upstream})
	if err := mirror.Open(); err != nil {
		return nil, err
	}

	s := &server{
		mirror:   mirror,
		sm:       session.New(),
		upstream: upstream,
	}

	return s, nil
}

func (s *server) run(addr string) error {
	fmt.Println(addr)
	return http.ListenAndServe(addr, s.router())
}
