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

// TiDBSupportTiproxy return if given version of TiDB support tiproxy
func TiDBSupportTiproxy(version string) bool {
	return semver.Compare(version, "v6.4.0") >= 0 || strings.Contains(version, "nightly")
}

// TiDBSupportUpgradeAPI return if given version of TiDB support upgrade API
func TiDBSupportUpgradeAPI(version string) bool {
	return semver.Compare(version, "v7.4.0") >= 0 ||
		(semver.MajorMinor(version) == "v7.1" && semver.Compare(version, "v7.1.2") >= 0) ||
		strings.Contains(version, "nightly")
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

// TiFlashSupportAdvertiseStatusAddr return if given version of  TiFlash support --advertise-status-addr
func TiFlashSupportAdvertiseStatusAddr(version string) bool {
	// TiFlash support --advertise-status-addr since v4.0.5
	return semver.Compare(version, "v4.0.5") >= 0 || strings.Contains(version, "nightly")
}

// TiFlashSupportMultiDisksDeployment return if given version of TiFlash support multi-disks deployment
func TiFlashSupportMultiDisksDeployment(version string) bool {
	// https://github.com/pingcap/tiup/pull/931
	return semver.Compare(version, "v4.0.9") >= 0 || strings.Contains(version, "nightly")
}

// TiFlashRequireCPUFlagAVX2 return if given version of TiFlash requires AVX2 CPU flags
func TiFlashRequireCPUFlagAVX2(version string) bool {
	// https://github.com/pingcap/tiup/pull/2054
	return semver.Compare(version, "v6.3.0") >= 0 || strings.Contains(version, "nightly")
}

// TiFlashDeprecatedUsersConfig return if given version of TiFlash deprecated users.* config
func TiFlashDeprecatedUsersConfig(version string) bool {
	// https://github.com/pingcap/tiup/pull/1211
	return semver.Compare(version, "v4.0.12") >= 0 && version != "v5.0.0-rc" || strings.Contains(version, "nightly")
}

// TiFlashNotNeedHTTPPortConfig return if given version of TiFlash do not need http_port config
func TiFlashNotNeedHTTPPortConfig(version string) bool {
	return semver.Compare(version, "v7.1.0") >= 0 || strings.Contains(version, "nightly")
}

// TiFlashRequiresTCPPortConfig return if given version of TiFlash requires tcp_port config.
// TiFlash 7.1.0 and later versions won't listen to tpc_port if the config is not given, which is recommended.
// However this config is required for pre-7.1.0 versions because TiFlash will listen to it anyway,
// and we must make sure the port is being configured as specified in the topology file,
// otherwise multiple TiFlash instances will conflict.
func TiFlashRequiresTCPPortConfig(version string) bool {
	return semver.Compare(version, "v7.1.0") < 0 && !strings.Contains(version, "nightly")
}

// TiFlashNotNeedSomeConfig return if given version of TiFlash do not need some config like runAsDaemon
func TiFlashNotNeedSomeConfig(version string) bool {
	// https://github.com/pingcap/tiup/pull/1673
	return semver.Compare(version, "v5.4.0") >= 0 || strings.Contains(version, "nightly")
}

// TiFlashPlaygroundNewStartMode return true if the given version of TiFlash could be started
// with the new implementation in TiUP playground.
func TiFlashPlaygroundNewStartMode(version string) bool {
	return semver.Compare(version, "v7.1.0") >= 0 || strings.Contains(version, "nightly")
}

// PDSupportMicroServices returns true if the given version of PD supports micro services.
func PDSupportMicroServices(version string) bool {
	return semver.Compare(version, "v7.3.0") >= 0 || strings.Contains(version, "nightly")
}

// PDSupportMicroServicesWithName return if the given version of PD supports micro services with name.
func PDSupportMicroServicesWithName(version string) bool {
	return semver.Compare(version, "v8.3.0") >= 0 || strings.Contains(version, "nightly")
}

// TiCDCSupportConfigFile return if given version of TiCDC support config file
func TiCDCSupportConfigFile(version string) bool {
	// config support since v4.0.13, ignore v5.0.0-rc
	return semver.Compare(version, "v4.0.13") >= 0 && version != "v5.0.0-rc" || strings.Contains(version, "nightly")
}

// TiCDCSupportSortOrDataDir return if given version of TiCDC support --sort-dir or --data-dir
func TiCDCSupportSortOrDataDir(version string) bool {
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

// TiCDCSupportClusterID return if the given version of TiCDC support --cluster-id param to identify TiCDC cluster
func TiCDCSupportClusterID(version string) bool {
	return semver.Compare(version, "v6.2.0") >= 0 || strings.Contains(version, "nightly")
}

// TiCDCSupportRollingUpgrade return if the given version of TiCDC support rolling upgrade
// TiCDC support graceful rolling upgrade since v6.3.0
func TiCDCSupportRollingUpgrade(version string) bool {
	return semver.Compare(version, "v6.3.0") >= 0 || strings.Contains(version, "nightly")
}

// TiCDCUpgradeBeforePDTiKVTiDB return if the given version of TiCDC should upgrade TiCDC before PD and TiKV
func TiCDCUpgradeBeforePDTiKVTiDB(version string) bool {
	return semver.Compare(version, "v5.1.0") >= 0 || strings.Contains(version, "nightly")
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

// TiKVCDCSupportDeploy return if given version of TiDB/TiKV cluster is supported
func TiKVCDCSupportDeploy(version string) bool {
	// TiKV-CDC only support TiKV version not less than v6.2.0
	return semver.Compare(version, "v6.2.0") >= 0 || strings.Contains(version, "nightly")
}
