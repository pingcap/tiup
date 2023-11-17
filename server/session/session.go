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
	"io"
	"os"
	"path"
	"sync"
	"time"

	"github.com/pingcap/errors"
	"github.com/pingcap/tiup/pkg/localdata"
	logprinter "github.com/pingcap/tiup/pkg/logger/printer"
	"github.com/pingcap/tiup/pkg/utils"
)

// Max alive time of a session
const maxAliveTime = 600 * time.Second

var (
	// ErrorSessionConflict indicates that a same session existed
	ErrorSessionConflict = errors.New("a session with same identity has been existed")
)

// Manager provide methods to operates on upload sessions
type Manager interface {
	Write(id string, name string, reader io.Reader) error
	Read(id string) (string, io.ReadCloser, error)
	Delete(id string)
}

type sessionManager struct {
	m *sync.Map
}

// New returns a session manager
func New() Manager {
	return &sessionManager{
		m: &sync.Map{},
	}
}

// Write start a new session
func (s *sessionManager) Write(id, name string, reader io.Reader) error {
	if _, ok := s.m.Load(id); ok {
		return ErrorSessionConflict
	}
	logprinter.Debugf("Begin new session: %s", id)
	s.m.Store(id, name)
	go s.gc(id)

	dataDir := os.Getenv(localdata.EnvNameComponentDataDir)
	if dataDir == "" {
		return errors.Errorf("cannot read environment variable %s", localdata.EnvNameComponentDataDir)
	}

	pkgDir := path.Join(dataDir, "packages")
	if err := utils.MkdirAll(pkgDir, 0755); err != nil {
		return errors.Annotate(err, "create package dir")
	}

	filePath := path.Join(pkgDir, id+"_"+name)
	file, err := os.Create(filePath)
	if err != nil {
		return errors.Annotate(err, "create tar file")
	}
	defer file.Close()

	if _, err := io.Copy(file, reader); err != nil {
		return errors.Annotate(err, "write tar file")
	}

	return nil
}

// Read returns the tar file of given session
func (s *sessionManager) Read(id string) (string, io.ReadCloser, error) {
	n, ok := s.m.Load(id)
	if !ok {
		return "", nil, nil
	}
	name := n.(string)

	dataDir := os.Getenv(localdata.EnvNameComponentDataDir)
	if dataDir == "" {
		return "", nil, errors.Errorf("cannot read environment variable %s", localdata.EnvNameComponentDataDir)
	}

	pkgDir := path.Join(dataDir, "packages")
	if err := utils.MkdirAll(pkgDir, 0755); err != nil {
		return "", nil, errors.Annotate(err, "create package dir")
	}

	filePath := path.Join(pkgDir, id+"_"+name)

	file, err := os.Open(filePath)
	if err != nil {
		return "", nil, errors.Annotate(err, "open tar file")
	}
	return name, file, nil
}

// Delele delete a session
func (s *sessionManager) Delete(id string) {
	logprinter.Debugf("Delete session: %s", id)
	n, ok := s.m.Load(id)
	if !ok {
		return
	}
	name := n.(string)
	os.Remove(path.Join(os.Getenv(localdata.EnvNameComponentDataDir), "packages", id+"_"+name))
	s.m.Delete(id)
}

func (s *sessionManager) gc(id string) {
	time.Sleep(maxAliveTime)

	if _, ok := s.m.Load(id); !ok {
		return
	}

	s.Delete(id)
}
