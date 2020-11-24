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

package v0manifest

import (
	"strings"

	pkgver "github.com/pingcap/tiup/pkg/repository/version"
)

type (
	// ComponentInfo represents the information of component
	ComponentInfo struct {
		Name       string   `json:"name"`
		Desc       string   `json:"desc"`
		Standalone bool     `json:"standalone"`
		Hide       bool     `json:"hide"` // don't show in component list
		Platforms  []string `json:"platforms"`
	}

	// ComponentManifest represents the all components information of tiup supported
	ComponentManifest struct {
		Description string          `json:"description"`
		Modified    string          `json:"modified"`
		TiUPVersion pkgver.Version  `json:"tiup_version"`
		Components  []ComponentInfo `json:"components"`
	}
)

// HasComponent checks whether the manifest contains the specific component
func (m *ComponentManifest) HasComponent(component string) bool {
	for _, c := range m.Components {
		if strings.EqualFold(c.Name, component) {
			return true
		}
	}
	return false
}

// FindComponent find the component by name and returns component info and a bool to indicate whether it exists
func (m *ComponentManifest) FindComponent(component string) (ComponentInfo, bool) {
	for _, c := range m.Components {
		if strings.EqualFold(c.Name, component) {
			return c, true
		}
	}
	return ComponentInfo{}, false
}

// IsSupport returns true if the component can be run in current OS and architecture
func (c *ComponentInfo) IsSupport(platform string) bool {
	for _, p := range c.Platforms {
		if p == platform {
			return true
		}
	}
	return false
}
