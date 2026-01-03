package main

import "github.com/pingcap/tiup/components/playground/proc"

func buildProcTitleCountsFromProcs(procs map[proc.ServiceID][]proc.Process) map[string]int {
	counts := make(map[string]int)
	for _, list := range procs {
		for _, inst := range list {
			if inst == nil {
				continue
			}
			counts[procTitle(inst)]++
		}
	}
	return counts
}

func (p *Playground) onProcsChangedInController(state *controllerState) {
	if p == nil || state == nil {
		return
	}
	p.progressMu.Lock()
	p.procTitleCounts = buildProcTitleCountsFromProcs(state.procs)
	p.progressMu.Unlock()

	logIfErr(p.renderSDFileInController(state))
}

func (p *Playground) renderSDFileInController(state *controllerState) error {
	if p == nil || state == nil {
		return nil
	}

	proms := state.procs[proc.ServicePrometheus]
	if len(proms) == 0 || proms[0] == nil {
		return nil
	}
	prom, ok := proms[0].(*proc.PrometheusInstance)
	if !ok || prom == nil {
		return nil
	}
	return renderPrometheusSDFile(prom, state.walkProcs)
}
