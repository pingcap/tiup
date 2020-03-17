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
	"time"
)

// ManifestIndex represent the object in tiup-manifest.index
type ManifestIndex struct {
	Description string      `json:"description"`
	Modified    time.Time   `json:"modified"`
	TiUPVersion string      `json:"tiup_version"`
	Components  []Component `json:"components"`
}

// Component represent the component object in tiup-manifest.index
type Component struct {
	Name      string   `json:"name"`
	Desc      string   `json:"desc"`
	Platforms []string `json:"platforms"`
}

// ComponentIndex represent the object in tiup-component-xxx.index
type ComponentIndex struct {
	Description string    `json:"description"`
	Modified    time.Time `json:"modified"`
	Nightly     *Version  `json:"nightly"`
	Versions    []Version `json:"versions"`
}

// Version represent the version boject in tiup-component-xxx.index
type Version struct {
	Version   string    `json:"version"`
	Date      time.Time `json:"date"`
	Entry     string    `json:"entry"`
	Platforms []string  `json:"platforms"`
}
