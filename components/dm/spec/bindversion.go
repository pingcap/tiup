package spec

import "github.com/pingcap/tiup/pkg/cluster/spec"

// DMComponentVersion maps the dm version to the third components binding version
// Empty version means the latest stable one
func DMComponentVersion(comp, version string) string {
	switch comp {
	case spec.ComponentAlertmanager,
		spec.ComponentGrafana,
		spec.ComponentPrometheus,
		spec.ComponentBlackboxExporter,
		spec.ComponentNodeExporter:
		return ""
	default:
		return version
	}
}
