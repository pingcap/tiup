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

package session

import (
	"errors"
	"sync"
	"time"

	"github.com/pingcap/tiup/pkg/logger/log"
	"github.com/pingcap/tiup/server/store"
)

// Max alive time of a session
const maxAliveTime = 600 * time.Second

var (
	// ErrorSessionConflict indicates that a same session existed
	ErrorSessionConflict = errors.New("a session with same identity has been existed")
)

// Manager provide methods to operates on upload sessions
type Manager interface {
	Begin(id string) error
	Load(id string) store.FsTxn
	Delete(id string)
}

type sessionManager struct {
	store store.Store
	txns  *sync.Map
}

// New returns a session manager
func New(store store.Store, txns *sync.Map) Manager {
	return &sessionManager{
		store: store,
		txns:  txns,
	}
}

// Begin start a new session and returns the session id
func (s *sessionManager) Begin(id string) error {
	if s.Load(id) != nil {
		return ErrorSessionConflict
	}
	log.Debugf("Begin new session: %s", id)
	txn, err := s.store.Begin()
	if err != nil {
		return err
	}
	s.txns.Store(id, txn)
	go s.gc(id)
	return nil
}

func (s *sessionManager) gc(id string) {
	time.Sleep(maxAliveTime)

	txn := s.Load(id)
	if txn == nil {
		return
	}

	s.Delete(id)
	if err := txn.Rollback(); err != nil {
		log.Errorf("Rollback: %s", err.Error())
	}
}

// Get returns the txn of given session
func (s *sessionManager) Load(id string) store.FsTxn {
	txn, ok := s.txns.Load(id)
	if ok {
		return txn.(store.FsTxn)
	}
	return nil
}

// Delele delete a session
func (s *sessionManager) Delete(id string) {
	log.Debugf("Delete session: %s", id)
	s.txns.Delete(id)
}
