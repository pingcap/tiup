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

package model

import "io"

// ComponentData is used to represent the tarbal
type ComponentData interface {
	io.Reader

	// Filename is the name of tarbal
	Filename() string
}

// ComponentInfo is used to update component
type ComponentInfo interface {
	ComponentData

	Standalone() *bool
	Yanked() *bool
	Hidden() *bool
	OwnerID() string
}

// PublishInfo implements ComponentInfo
type PublishInfo struct {
	ComponentData
	Stand *bool
	Yank  *bool
	Hide  *bool
	Owner string
}

// TarInfo implements ComponentData
type TarInfo struct {
	io.Reader
	Name string
}

// Filename implements ComponentData
func (ti *TarInfo) Filename() string {
	return ti.Name
}

// Filename implements ComponentData
func (i *PublishInfo) Filename() string {
	if i.ComponentData == nil {
		return ""
	}
	return i.ComponentData.Filename()
}

// Standalone implements ComponentInfo
func (i *PublishInfo) Standalone() *bool {
	return i.Stand
}

// Yanked implements ComponentInfo
func (i *PublishInfo) Yanked() *bool {
	return i.Yank
}

// Hidden implements ComponentInfo
func (i *PublishInfo) Hidden() *bool {
	return i.Hide
}

// OwnerID implements ComponentInfo
func (i *PublishInfo) OwnerID() string {
	return i.Owner
}
