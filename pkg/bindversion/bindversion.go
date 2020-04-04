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

package bindversion

import (
	"github.com/pingcap-incubator/tiup-cluster/pkg/meta"
	"github.com/pingcap-incubator/tiup/pkg/repository"
)

// ComponentVersion maps the TiDB version to the third components binding version
func ComponentVersion(comp, version string) repository.Version {
	switch comp {
	case meta.ComponentPrometheus:
		return "v2.8.1"
	case meta.ComponentGrafana:
		return "v6.1.6"
	case meta.ComponentAlertManager:
		return "v0.17.0"
	case meta.ComponentBlackboxExporter:
		return "v0.12.0"
	case meta.ComponentNodeExporter:
		return "v0.17.0"
	case meta.ComponentPushwaygate:
		return "v0.7.0"
	default:
		return repository.Version(version)
	}
}
