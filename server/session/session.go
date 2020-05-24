package session

import (
	"errors"
	"sync"
	"time"

	"github.com/pingcap-incubator/tiup/server/store"
)

// Max alive time of a session
const maxAliveTime = 3600 * time.Second

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
	txn, err := s.store.Begin()
	if err != nil {
		return err
	}
	s.txns.Store(id, txn)
	go s.gc(id)
	return nil
}

func (s *sessionManager) gc(id string) error {
	time.Sleep(maxAliveTime)

	txn := s.Load(id)
	if txn == nil {
		return nil
	}

	s.Delete(id)
	return txn.Rollback()
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
	s.txns.Delete(id)
}
