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

// DefaultTiupHome represents the default home directory for this build of tiup
// If this is left empty, the default will be thee combination of the running
// user's home directory and ProfileDirName
var DefaultTiupHome string

// ProfileDirName is the name of the profile directory to be used
var ProfileDirName = ".tiup"

const (
	// ComponentParentDir represent the parent directory of all downloaded components
	ComponentParentDir = "components"

	// DataParentDir represent the parent directory of all running instances
	DataParentDir = "data"

	// EnvNameInstanceDataDir represents the working directory of specific instance
	EnvNameInstanceDataDir = "TIUP_INSTANCE_DATA_DIR"

	// EnvNameComponentInstallDir represents the install directory of specific component
	EnvNameComponentInstallDir = "TIUP_COMPONENT_INSTALL_DIR"

	// EnvNameWorkDir represents the work directory of TiUP where user type the command `tiup xxx`
	EnvNameWorkDir = "TIUP_WORK_DIR"

	// EnvNameHome represents the environment name of tiup home directory
	EnvNameHome = "TIUP_HOME"

	// MetaFilename represents the process meta file name
	MetaFilename = "tiup_process_meta"
)
