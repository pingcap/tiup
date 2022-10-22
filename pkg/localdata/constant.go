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

// DefaultTiUPHome represents the default home directory for this build of tiup
// If this is left empty, the default will be thee combination of the running
// user's home directory and ProfileDirName
var DefaultTiUPHome string

// ProfileDirName is the name of the profile directory to be used
var ProfileDirName = ".tiup"

// Notice: if you try to add a new env name which is notable by the user, shou should
// add it to cmd/env.go:envList so that the command `tiup env` will show that env.
const (
	// ComponentParentDir represent the parent directory of all downloaded components
	ComponentParentDir = "components"

	// ManifestParentDir represent the parent directory of all manifests
	ManifestParentDir = "manifests"

	// KeyInfoParentDir represent the parent directory of all keys
	KeyInfoParentDir = "keys"

	// DefaultPrivateKeyName represents the default private key file stored in ${TIUP_HOME}/keys
	DefaultPrivateKeyName = "private.json"

	// DataParentDir represent the parent directory of all running instances
	DataParentDir = "data"

	// TelemetryDir represent the parent directory of telemetry info
	TelemetryDir = "telemetry"

	// StorageParentDir represent the parent directory of running component
	StorageParentDir = "storage"

	// TrustedDir represent the parent directory of root.json of mirrors
	TrustedDir = "trusted"

	// EnvNameInstanceDataDir represents the working directory of specific instance
	EnvNameInstanceDataDir = "TIUP_INSTANCE_DATA_DIR"

	// EnvNameComponentDataDir represents the working directory of specific component
	EnvNameComponentDataDir = "TIUP_COMPONENT_DATA_DIR"

	// EnvNameComponentInstallDir represents the install directory of specific component
	EnvNameComponentInstallDir = "TIUP_COMPONENT_INSTALL_DIR"

	// EnvNameWorkDir represents the work directory of TiUP where user type the command `tiup xxx`
	EnvNameWorkDir = "TIUP_WORK_DIR"

	// EnvNameUserInputVersion represents the version user specified when running a component by `tiup component:version`
	EnvNameUserInputVersion = "TIUP_USER_INPUT_VERSION"

	// EnvNameTiUPVersion represents the version of TiUP itself, not the version of component
	EnvNameTiUPVersion = "TIUP_VERSION"

	// EnvNameHome represents the environment name of tiup home directory
	EnvNameHome = "TIUP_HOME"

	// EnvNameTelemetryStatus represents the environment name of tiup telemetry status
	EnvNameTelemetryStatus = "TIUP_TELEMETRY_STATUS"

	// EnvNameTelemetryUUID represents the environment name of tiup telemetry uuid
	EnvNameTelemetryUUID = "TIUP_TELEMETRY_UUID"

	// EnvNameTelemetryEventUUID represents the environment name of tiup telemetry event uuid
	EnvNameTelemetryEventUUID = "TIUP_TELEMETRY_EVENT_UUID"

	// EnvNameTelemetrySecret represents the environment name of tiup telemetry secret
	EnvNameTelemetrySecret = "TIUP_TELEMETRY_SECRET"

	// EnvTag is the tag of the running component
	EnvTag = "TIUP_TAG"

	// EnvNameSSHPassPrompt is the variable name by which user specific the password prompt for sshpass
	EnvNameSSHPassPrompt = "TIUP_SSHPASS_PROMPT"

	// EnvNameNativeSSHClient is the variable name by which user can specific use native ssh client or not
	EnvNameNativeSSHClient = "TIUP_NATIVE_SSH"

	// EnvNameSSHPath is the variable name by which user can specific the executable ssh binary path
	EnvNameSSHPath = "TIUP_SSH_PATH"

	// EnvNameSCPPath is the variable name by which user can specific the executable scp binary path
	EnvNameSCPPath = "TIUP_SCP_PATH"

	// EnvNameKeepSourceTarget is the variable name by which user can keep the source target or not
	EnvNameKeepSourceTarget = "TIUP_KEEP_SOURCE_TARGET"

	// EnvNameMirrorSyncScript make it possible for user to sync mirror commit to other place (eg. CDN)
	EnvNameMirrorSyncScript = "TIUP_MIRROR_SYNC_SCRIPT"

	// EnvNameLogPath is the variable name by which user can write the log files into
	EnvNameLogPath = "TIUP_LOG_PATH"

	// EnvNameDebug is the variable name by which user can set tiup runs in debug mode(eg. print panic logs)
	EnvNameDebug = "TIUP_CLUSTER_DEBUG"

	// MetaFilename represents the process meta file name
	MetaFilename = "tiup_process_meta"
)
