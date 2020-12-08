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

const (
	// ManifestFileName is the filename of the manifest.
	ManifestFileName = "tiup-manifest.index"

	// DefaultMirror is the location of the mirror to use if none is specified by the user via `EnvMirrors`.
	DefaultMirror = "https://tiup-mirrors.pingcap.com/"

	// EnvMirrors is the name of an env var the user can set to specify a mirror.
	EnvMirrors = "TIUP_MIRRORS"

	// TiUPBinaryName is the name of the tiup binary, both in the repository and locally.
	TiUPBinaryName = "tiup"
)
