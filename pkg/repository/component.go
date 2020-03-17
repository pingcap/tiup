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

package repository

import "strings"

type (
	// ComponentInfo represents the information of component
	ComponentInfo struct {
		Name      string   `json:"name"`
		Desc      string   `json:"desc"`
		Platforms []string `json:"platforms"`
	}

	// ComponentManifest represents the all components information of tiup supported
	ComponentManifest struct {
		Description string          `json:"description"`
		Modified    string          `json:"modified"`
		TiUPVersion Version         `json:"tiup_version"`
		Components  []ComponentInfo `json:"components"`
	}
)

// HasComponent checks whether the manifest contains the specific component
func (m *ComponentManifest) HasComponent(component string) bool {
	for _, c := range m.Components {
		if strings.ToLower(c.Name) == strings.ToLower(component) {
			return true
		}
	}
	return false
}
