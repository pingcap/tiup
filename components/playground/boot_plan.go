package main

import (
	"github.com/pingcap/tiup/components/playground/proc"
	pgservice "github.com/pingcap/tiup/components/playground/service"
)

type bootPlan struct {
	Plans []plannedProc

	// BaseConfigs holds the per-service config snapshot decided during planning.
	// It is used for boot-time defaults and scale-out request sanitization.
	BaseConfigs map[proc.ServiceID]proc.Config

	// RequiredServices is the minimum running instance count for "critical"
	// services. Controller uses it to trigger auto shutdown if critical services
	// exit unexpectedly.
	RequiredServices map[proc.ServiceID]int
}

func buildBootPlan(options *BootOptions) (bootPlan, error) {
	plans, err := planProcs(options)
	if err != nil {
		return bootPlan{}, err
	}

	baseConfigs := make(map[proc.ServiceID]proc.Config, len(plans))
	for _, plan := range plans {
		baseConfigs[plan.serviceID] = plan.cfg
	}

	required := make(map[proc.ServiceID]int)
	for _, plan := range plans {
		if plan.serviceID == "" || plan.cfg.Num <= 0 || options == nil {
			continue
		}
		spec, ok := pgservice.SpecFor(plan.serviceID)
		if !ok {
			continue
		}
		if spec.Catalog.IsCritical != nil && spec.Catalog.IsCritical(options) {
			required[plan.serviceID] = 1
		}
	}

	return bootPlan{
		Plans:            plans,
		BaseConfigs:      baseConfigs,
		RequiredServices: required,
	}, nil
}
