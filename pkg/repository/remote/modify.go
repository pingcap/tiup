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

package remote

import (
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/juju/errors"
	"github.com/pingcap/tiup/pkg/repository/v0manifest"
	"github.com/pingcap/tiup/pkg/repository/v1manifest"
)

// Editor defines the methods to modify a component attrs
type Editor interface {
	WithVersion(version string) Editor
	WithDesc(desc string) Editor
	Standalone(bool) Editor
	Hide(bool) Editor
	Yank(bool) Editor
	Sign(key *v1manifest.KeyInfo, m *v1manifest.Component) error
}

type editor struct {
	endpoint    string
	component   string
	version     string
	description string
	options     map[string]bool
}

// NewEditor returns a Editor interface
func NewEditor(endpoint, component string) Editor {
	return &editor{
		endpoint:  endpoint,
		component: component,
		options:   make(map[string]bool),
	}
}

// WithVersion set version field
func (e *editor) WithVersion(version string) Editor {
	e.version = version
	return e
}

// WithDesc set description field
func (e *editor) WithDesc(desc string) Editor {
	e.description = desc
	return e
}

// Hide set hidden flag
func (e *editor) Hide(hidden bool) Editor {
	e.options["hidden"] = hidden
	return e
}

// Standalone set standalone flag
func (e *editor) Standalone(standalone bool) Editor {
	e.options["standalone"] = standalone
	return e
}

// Yank set yanked flag
func (e *editor) Yank(yanked bool) Editor {
	e.options["yanked"] = yanked
	return e
}

func (e *editor) Sign(key *v1manifest.KeyInfo, m *v1manifest.Component) error {
	initTime := time.Now()
	v1manifest.RenewManifest(m, initTime)
	m.Version++
	if e.description != "" {
		m.Description = e.description
	}

	sid := uuid.New().String()
	url := fmt.Sprintf("%s/api/v1/component/%s/%s", e.endpoint, sid, e.component)

	if e.version != "" {
		// Only support modify yanked field for specified versiion
		for p := range m.Platforms {
			if v0manifest.Version(e.version).IsNightly() {
				return errors.New("nightly version can't be yanked")
			}
			vi, ok := m.Platforms[p][e.version]
			if !ok {
				continue
			}
			vi.Yanked = e.options["yanked"]
			m.Platforms[p][e.version] = vi
		}
		return signAndSend(url, m, key, nil)
	}
	return signAndSend(url, m, key, e.options)
}
