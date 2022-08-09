// Copyright 2022 PingCAP, Inc.
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

package tidbver

import (
	"strings"

	"golang.org/x/mod/semver"
)

// warning: invalid semantic version string is considered less than a valid one when using semver.Compare

// TiDBSupportSecureBoot return if given version of TiDB support secure boot
func TiDBSupportSecureBoot(version string) bool {
	return semver.Compare(version, "v5.3.0") >= 0 || strings.Contains(version, "nightly")
}

// TiKVSupportAdvertiseStatusAddr return if given version of TiKV support --advertise-status-addr
func TiKVSupportAdvertiseStatusAddr(version string) bool {
	// TiKV support --advertise-status-addr since v4.0.1
	return semver.Compare(version, "v4.0.1") >= 0 || strings.Contains(version, "nightly")
}

// TiFlashSupportTLS return if given version of TiFlash support TLS
func TiFlashSupportTLS(version string) bool {
	return semver.Compare(version, "v4.0.5") >= 0 || strings.Contains(version, "nightly")
}

//  TiFlashSupportAdvertiseStatusAddr return if given version of  TiFlash support --advertise-status-addr
func TiFlashSupportAdvertiseStatusAddr(version string) bool {
	// TiFlash support --advertise-status-addr since v4.0.5
	return semver.Compare(version, "v4.0.5") >= 0 || strings.Contains(version, "nightly")
}

// TiFlashSupportMultiDisksDeployment return if given version of TiFlash support multi-disks deployment
func TiFlashSupportMultiDisksDeployment(version string) bool {
	// https://github.com/pingcap/tiup/pull/931
	return semver.Compare(version, "v4.0.9") >= 0 || strings.Contains(version, "nightly")
}

// TiFlashDeprecatedUsersConfig return if given version of TiFlash deprecated users.* config
func TiFlashDeprecatedUsersConfig(version string) bool {
	// https://github.com/pingcap/tiup/pull/1211
	return semver.Compare(version, "v4.0.12") >= 0 && version != "v5.0.0-rc" || strings.Contains(version, "nightly")
}

// TiFlashNotNeedSomeConfig return if given version of TiFlash do not need some config like runAsDaemon
func TiFlashNotNeedSomeConfig(version string) bool {
	// https://github.com/pingcap/tiup/pull/1673
	return semver.Compare(version, "v5.4.0") >= 0 || strings.Contains(version, "nightly")
}

// TiCDCSupportConfigFile return if given version of TiCDC support config file
func TiCDCSupportConfigFile(version string) bool {
	// config support since v4.0.13, ignore v5.0.0-rc
	return semver.Compare(version, "v4.0.13") >= 0 && version != "v5.0.0-rc" || strings.Contains(version, "nightly")
}

// TiCDCSupportDataDir return if given version of TiCDC support --data-dir
func TiCDCSupportDataDir(version string) bool {
	// TiCDC support --data-dir since v4.0.14 and v5.0.3
	if semver.Compare(version, "v5.0.3") >= 0 || strings.Contains(version, "nightly") {
		return true
	}
	return semver.Major(version) == "v4" && semver.Compare(version, "v4.0.14") >= 0
}

// NgMonitorDeployByDefault return if given version of TiDB cluster should contain ng-monitoring
func NgMonitorDeployByDefault(version string) bool {
	return semver.Compare(version, "v5.4.0") >= 0 || strings.Contains(version, "nightly")
}

// PrometheusHasTiKVAccelerateRules return if given version of Prometheus has TiKV accelerateRules
func PrometheusHasTiKVAccelerateRules(version string) bool {
	// tikv.accelerate.rules.yml was first introduced in v4.0.0
	return semver.Compare(version, "v4.0.0") >= 0 || strings.Contains(version, "nightly")
}

// DMSupportDeploy return if given version of DM is supported bu tiup-dm
func DMSupportDeploy(version string) bool {
	// tiup-dm only support version not less than v2.0
	return semver.Compare(version, "v2.0.0") >= 0 || strings.Contains(version, "nightly")
}
