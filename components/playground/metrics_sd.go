package main

import "github.com/pingcap/tiup/components/playground/proc"

type procWalker func(fn func(serviceID proc.ServiceID, inst proc.Process) error) error

func renderPrometheusSDFile(prom *proc.PrometheusInstance, walk procWalker) error {
	if prom == nil || walk == nil {
		return nil
	}
	return prom.RenderSDFile(collectMetricTargets(walk))
}

func collectMetricTargets(walk procWalker) map[proc.ServiceID]proc.MetricAddr {
	sid2targets := make(map[proc.ServiceID]proc.MetricAddr)
	if walk == nil {
		return sid2targets
	}

	_ = walk(func(serviceID proc.ServiceID, inst proc.Process) error {
		if inst == nil {
			return nil
		}
		info := inst.Info()
		if serviceID == proc.ServicePrometheus || (info != nil && info.Service == proc.ServicePrometheus) {
			return nil
		}

		var v proc.MetricAddr
		if m, ok := inst.(interface{ MetricAddr() proc.MetricAddr }); ok {
			v = m.MetricAddr()
		} else if info != nil {
			v = info.MetricAddr()
		} else {
			return nil
		}
		if len(v.Targets) == 0 {
			return nil
		}

		if serviceID == "" && info != nil {
			serviceID = info.Service
		}
		if serviceID == "" {
			return nil
		}

		if t, ok := sid2targets[serviceID]; ok {
			v.Targets = append(v.Targets, t.Targets...)
			if v.Labels == nil && len(t.Labels) > 0 {
				v.Labels = t.Labels
			}
		}
		sid2targets[serviceID] = v
		return nil
	})

	return sid2targets
}
