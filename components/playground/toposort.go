package main

import (
	"fmt"
	"strings"

	"slices"

	"github.com/pingcap/tiup/components/playground/proc"
	pgservice "github.com/pingcap/tiup/components/playground/service"
)

func topoSortServiceIDs(serviceIDs []proc.ServiceID) ([]proc.ServiceID, error) {
	if len(serviceIDs) == 0 {
		return nil, nil
	}

	inSet := make(map[proc.ServiceID]struct{}, len(serviceIDs))
	for _, id := range serviceIDs {
		inSet[id] = struct{}{}
	}

	indegree := make(map[proc.ServiceID]int, len(serviceIDs))
	deps := make(map[proc.ServiceID][]proc.ServiceID, len(serviceIDs))
	for _, id := range serviceIDs {
		indegree[id] = 0
	}

	for _, id := range serviceIDs {
		spec, ok := pgservice.SpecFor(id)
		if !ok {
			continue
		}
		for _, dep := range spec.StartAfter {
			if _, ok := inSet[dep]; !ok {
				continue
			}
			deps[dep] = append(deps[dep], id)
			indegree[id]++
		}
	}

	var ready []proc.ServiceID
	for _, id := range serviceIDs {
		if indegree[id] == 0 {
			ready = append(ready, id)
		}
	}
	slices.SortStableFunc(ready, func(a, b proc.ServiceID) int {
		return strings.Compare(a.String(), b.String())
	})

	out := make([]proc.ServiceID, 0, len(serviceIDs))
	for len(ready) > 0 {
		id := ready[0]
		ready = ready[1:]
		out = append(out, id)

		for _, next := range deps[id] {
			indegree[next]--
			if indegree[next] == 0 {
				ready = append(ready, next)
			}
		}
		if len(ready) > 1 {
			slices.SortStableFunc(ready, func(a, b proc.ServiceID) int {
				return strings.Compare(a.String(), b.String())
			})
		}
	}

	if len(out) != len(serviceIDs) {
		var cycle []proc.ServiceID
		for _, id := range serviceIDs {
			if indegree[id] > 0 {
				cycle = append(cycle, id)
			}
		}
		slices.SortStableFunc(cycle, func(a, b proc.ServiceID) int {
			return strings.Compare(a.String(), b.String())
		})
		var names []string
		for _, id := range cycle {
			names = append(names, id.String())
		}
		return nil, fmt.Errorf("service dependency cycle detected: %s", strings.Join(names, ", "))
	}

	return out, nil
}
