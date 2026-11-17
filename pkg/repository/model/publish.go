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

import (
	"fmt"
	"io"
	"net/url"
	"os"
	"path"
	"strings"

	"github.com/pingcap/tiup/pkg/repository/v1manifest"
	"github.com/pingcap/tiup/pkg/version"
)

// archiveFilename binds a portable, flat filename to its manifest tuple. Platform
// tokens cannot contain hyphens, since copyTiUP extracts the final two tokens.
func archiveFilename(id, platform, ver string) string {
	goos, goarch, ok := strings.Cut(platform, "/")
	if !ok || goos == "" || goarch == "" || ver == "" || strings.ContainsAny(platform, "-\\\x00") || strings.Contains(goarch, "/") {
		return ""
	}
	name := fmt.Sprintf("%s-%s-%s-%s.tar.gz", id, ver, goos, goarch)
	for _, c := range name {
		if c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z' || c >= '0' && c <= '9' || strings.ContainsRune("-._+", c) {
			continue
		}
		return ""
	}
	return name
}

// tiupArchiveAlias preserves copyTiUP's historical filename rule, including
// custom version tokens. Derive it from the raw, flat filename, not a manifest
// platform or a normalized URL: those can name a different existing alias.
func tiupArchiveAlias(filename string) string {
	if strings.ContainsAny(filename, "/\\\x00") {
		return ""
	}
	parts := strings.Split(filename, "-")
	if len(parts) < 4 {
		return ""
	}
	return strings.Join([]string{parts[0], parts[len(parts)-2], parts[len(parts)-1]}, "-")
}

// localArchiveClaim reserves legacy relative URLs and aliases normalized by HTTP
// servers too. Normalization only reserves names; it never authorizes a write.
func localArchiveClaim(resource string) string {
	resource, _, _ = strings.Cut(resource, "?")
	resource, _, _ = strings.Cut(resource, "#")
	decoded, err := url.PathUnescape(resource)
	if err != nil {
		return ""
	}
	name := strings.TrimPrefix(path.Clean("/"+decoded), "/")
	if name == "" || name == "." || name == ".." || strings.ContainsAny(name, "/\\\x00") {
		return ""
	}
	return strings.ToLower(name)
}

func (m *model) validateArchiveClaims(index *v1manifest.Index, component *v1manifest.Component, upload string) error {
	for id := range index.Components {
		if id != component.ID && strings.EqualFold(id, component.ID) {
			return ErrorInvalidComponentID
		}
	}
	snapshot, err := m.readSnapshotManifest()
	if err != nil {
		return err
	}
	var previous *v1manifest.Component
	reserved := make(map[string]bool)
	owned := make(map[string]bool)
	for id := range index.Components {
		manifestVersion := snapshot.Signed.(*v1manifest.Snapshot).Meta["/"+id+".json"].Version
		manifest, err := m.txn.ReadManifest(fmt.Sprintf("%d.%s.json", manifestVersion, id), &v1manifest.Component{})
		if err != nil {
			return err
		}
		committed := manifest.Signed.(*v1manifest.Component)
		claims := reserved
		if id == component.ID {
			previous = committed
			claims = owned
		}
		// Include legacy custom filenames: canonical names are ambiguous when IDs
		// and prerelease versions contain hyphens, and older mirrors used custom URLs.
		for _, versions := range committed.Platforms {
			for _, item := range versions {
				if claim := localArchiveClaim(item.URL); claim != "" {
					claims[claim] = true
				}
				if filename, local := strings.CutPrefix(item.URL, "/"); id == version.TiUPVerName && local {
					if alias := tiupArchiveAlias(filename); alias != "" {
						claims[strings.ToLower(alias)] = true
					}
				}
			}
		}
	}
	checkClaim := func(name string) error {
		claim := strings.ToLower(name)
		if reserved[claim] {
			return ErrorWrongFileName
		}
		if !owned[claim] {
			// A retired URL does not make its file available to another publisher.
			// Check metadata-only claims too, before they can establish ownership.
			if _, err := m.txn.Stat(name); err == nil {
				return ErrorWrongFileName
			} else if !os.IsNotExist(err) {
				return err
			}
		}
		return nil
	}
	uploadFound := upload == ""
	for platform, versions := range component.Platforms {
		for ver, item := range versions {
			name := archiveFilename(component.ID, platform, ver)
			canonical := name != "" && item.URL == "/"+name
			if upload != "" && canonical && upload == name {
				uploadFound = true
			}
			if !canonical {
				// Preserve existing metadata, but never let a new reference reserve an
				// arbitrary filename or let a legacy reference authorize a file upload.
				if previous == nil {
					return ErrorWrongFileName
				}
				old, exists := previous.Platforms[platform][ver]
				if !exists || old.URL != item.URL {
					return ErrorWrongFileName
				}
			}
			if claim := localArchiveClaim(item.URL); claim != "" && reserved[claim] {
				return ErrorWrongFileName
			}
			if canonical {
				if err := checkClaim(name); err != nil {
					return err
				}
			}
			if component.ID == version.TiUPVerName && canonical {
				if err := checkClaim(tiupArchiveAlias(name)); err != nil {
					return err
				}
			}
		}
	}
	if !uploadFound {
		return ErrorWrongFileName
	}
	return nil
}

// ComponentData is used to represent the tarball
type ComponentData interface {
	io.Reader

	// Filename is the name of tarball
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
