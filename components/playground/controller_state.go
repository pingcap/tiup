package main

import (
	"fmt"
	"slices"
	"strings"

	"github.com/pingcap/tiup/components/playground/proc"
)

func (s *controllerState) allocID(serviceID proc.ServiceID) int {
	if s == nil || serviceID == "" {
		return 0
	}
	if s.idAlloc == nil {
		s.idAlloc = make(map[proc.ServiceID]int)
	}
	id := s.idAlloc[serviceID]
	s.idAlloc[serviceID] = id + 1
	return id
}

func (s *controllerState) appendProc(serviceID proc.ServiceID, inst proc.Process) {
	if s == nil || inst == nil || serviceID == "" {
		return
	}
	info := inst.Info()
	if info == nil {
		panic(fmt.Sprintf("append proc %T has nil info", inst))
	}
	if info.Service != serviceID {
		panic(fmt.Sprintf("append proc service mismatch: expect %s, got %s", serviceID, info.Service))
	}
	if s.procs == nil {
		s.procs = make(map[proc.ServiceID][]proc.Process)
	}
	s.procs[serviceID] = append(s.procs[serviceID], inst)
}

func (s *controllerState) removeProcByPID(serviceID proc.ServiceID, pid int) (proc.Process, bool) {
	if s == nil || pid <= 0 || serviceID == "" {
		return nil, false
	}

	list := s.procs[serviceID]
	for i := 0; i < len(list); i++ {
		inst := list[i]
		if inst == nil {
			continue
		}
		info := inst.Info()
		if info == nil || info.Proc == nil || info.Proc.Pid() != pid {
			continue
		}
		s.procs[serviceID] = slices.Delete(list, i, i+1)
		s.markProcRemoved(inst, pid)
		return inst, true
	}
	return nil, false
}

func (s *controllerState) removeProc(serviceID proc.ServiceID, inst proc.Process) bool {
	if s == nil || inst == nil || serviceID == "" {
		return false
	}

	list := s.procs[serviceID]
	for i := 0; i < len(list); i++ {
		if list[i] != inst {
			continue
		}
		s.procs[serviceID] = slices.Delete(list, i, i+1)
		s.markProcRemoved(inst, 0)
		return true
	}
	return false
}

func (s *controllerState) walkProcs(fn func(serviceID proc.ServiceID, inst proc.Process) error) error {
	if s == nil || fn == nil {
		return nil
	}

	serviceIDs := make([]proc.ServiceID, 0, len(s.procs))
	for serviceID := range s.procs {
		serviceIDs = append(serviceIDs, serviceID)
	}
	slices.SortFunc(serviceIDs, func(a, b proc.ServiceID) int {
		return strings.Compare(a.String(), b.String())
	})

	for _, serviceID := range serviceIDs {
		for _, inst := range s.procs[serviceID] {
			if err := fn(serviceID, inst); err != nil {
				return err
			}
		}
	}
	return nil
}

func (s *controllerState) rwalkProcs(fn func(serviceID proc.ServiceID, inst proc.Process) error) error {
	if s == nil || fn == nil {
		return nil
	}

	var ids []proc.ServiceID
	var procs []proc.Process
	_ = s.walkProcs(func(id proc.ServiceID, inst proc.Process) error {
		ids = append(ids, id)
		procs = append(procs, inst)
		return nil
	})

	for i := len(ids); i > 0; i-- {
		if err := fn(ids[i-1], procs[i-1]); err != nil {
			return err
		}
	}
	return nil
}
