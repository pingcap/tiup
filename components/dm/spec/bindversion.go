package spec

import "github.com/pingcap/tiup/pkg/cluster/spec"

// DMComponentVersion maps the dm version to the third components binding version
func DMComponentVersion(comp, version string) string {
	switch comp {
	case spec.ComponentAlertManager:
		return "v0.17.0"
	case spec.ComponentGrafana, spec.ComponentPrometheus:
		return "v4.0.3"
	default:
		return version
	}
}
