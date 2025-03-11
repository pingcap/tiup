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

package store

import (
	"io"
	"os"

	"github.com/pingcap/tiup/pkg/repository/v1manifest"
)

// Store represents the storage level
type Store interface {
	Begin() (FsTxn, error)
}

// FsTxn represent the transaction session of file operations
type FsTxn interface {
	Write(filename string, reader io.Reader) error
	Read(filename string) (io.ReadCloser, error)
	WriteManifest(filename string, manifest *v1manifest.Manifest) error
	ReadManifest(filename string, role v1manifest.ValidManifest) (*v1manifest.Manifest, error)
	Stat(filename string) (os.FileInfo, error)
	// ResetManifest should reset the manifest state
	ResetManifest() error
	Commit() error
	Rollback() error
}

// New returns a Store, currently only qcloud supported
func New(root string, upstream string) Store {
	return newLocalStore(root, upstream)
}
