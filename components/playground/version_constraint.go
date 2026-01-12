package main

import (
	"github.com/pingcap/tiup/components/playground/proc"
	pgservice "github.com/pingcap/tiup/components/playground/service"
)

func serviceVersionConstraint(serviceID proc.ServiceID, bootVersion string, options *BootOptions) string {
	constraint := bootVersion

	spec, ok := pgservice.SpecFor(serviceID)
	if !ok {
		return constraint
	}

	if spec.Catalog.AllowModifyVersion && options != nil {
		if cfg := options.Service(serviceID); cfg != nil && cfg.Version != "" {
			constraint = cfg.Version
		}
	}

	if bind := spec.Catalog.VersionBind; bind != nil {
		constraint = bind(constraint)
	}

	return constraint
}
