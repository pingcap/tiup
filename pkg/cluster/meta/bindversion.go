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

package meta

// ComponentVersion maps the TiDB version to the third components binding version
func ComponentVersion(comp, version string) string {
	switch comp {
	case ComponentAlertManager:
		return "v0.17.0"
	case ComponentBlackboxExporter:
		return "v0.12.0"
	case ComponentNodeExporter:
		return "v0.17.0"
	case ComponentPushwaygate:
		return "v0.7.0"
	case ComponentCheckCollector:
		return "v0.3.1"
	default:
		return version
	}
}
