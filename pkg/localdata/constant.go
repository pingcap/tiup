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

package localdata

const (
	// ComponentParentDir represent the parent directory of all downloaded components
	ComponentParentDir = "components"

	// DataParentDir represent the parent directory of all running instances
	DataParentDir = "data"

	// EnvNameInstanceDataDir represents the working directory of specific instance
	EnvNameInstanceDataDir = "TIUP_INSTANCE_DATA_DIR"

	// EnvNameHome represents the environment name of tiup home directory
	EnvNameHome = "TIUP_HOME"

	// MetaFilename represents the process meta file name
	MetaFilename = "tiup_process_meta"
)
