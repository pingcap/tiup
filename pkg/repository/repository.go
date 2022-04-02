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

// Repository represents a components repository. All logic concerning manifests and the locations of tarballs
// is contained in the Repository object. Any IO is delegated to mirrorSource, which in turn will delegate fetching
// files to a Mirror.
type Repository struct {
	Options
}

// Options represents options for a repository
type Options struct {
	GOOS              string
	GOARCH            string
	DisableDecompress bool
}
