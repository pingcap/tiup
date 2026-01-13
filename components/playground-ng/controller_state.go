package main

import (
	"fmt"
	"slices"
	"strings"
	"time"

	"github.com/pingcap/tiup/components/playground-ng/proc"
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

type procRecord struct {
	inst      proc.Process
	serviceID proc.ServiceID
	name      string
	pid       int

	logFile   string
	startedAt time.Time

	removedFromProcs bool
}

type procRecordSnapshot struct {
	ServiceID proc.ServiceID
	Name      string
	PID       int
	Inst      proc.Process
	Removed   bool
}

func (s *controllerState) upsertProcRecord(inst proc.Process) {
	if s == nil || inst == nil {
		return
	}
	info := inst.Info()
	if info == nil || info.Proc == nil {
		return
	}
	pid := info.Proc.Pid()
	if pid <= 0 {
		return
	}
	name := info.Name()
	if name == "" {
		return
	}

	if s.procByPID == nil {
		s.procByPID = make(map[int]*procRecord)
	}
	if s.procByName == nil {
		s.procByName = make(map[string]*procRecord)
	}

	rec := s.procByPID[pid]
	if rec == nil {
		rec = &procRecord{
			inst:      inst,
			serviceID: info.Service,
			name:      name,
			pid:       pid,
			logFile:   inst.LogFile(),
			startedAt: time.Now(),
		}
		s.procByPID[pid] = rec
	} else {
		oldName := rec.name
		rec.inst = inst
		rec.serviceID = info.Service
		rec.name = name
		rec.pid = pid
		if rec.logFile == "" {
			rec.logFile = inst.LogFile()
		}
		if rec.startedAt.IsZero() {
			rec.startedAt = time.Now()
		}
		if oldName != "" && oldName != name {
			if cur := s.procByName[oldName]; cur == rec {
				delete(s.procByName, oldName)
			}
		}
	}
	s.procByName[name] = rec
}

func (s *controllerState) markProcRemoved(inst proc.Process, pid int) {
	if s == nil {
		return
	}

	if pid <= 0 && inst != nil {
		info := inst.Info()
		if info != nil && info.Proc != nil {
			pid = info.Proc.Pid()
		}
	}

	if pid <= 0 {
		return
	}

	if rec := s.procByPID[pid]; rec != nil {
		rec.removedFromProcs = true
	}
}

func (s *controllerState) deleteProcRecord(pid int, name string) {
	if s == nil {
		return
	}
	if pid <= 0 && name == "" {
		return
	}

	rec := (*procRecord)(nil)
	if pid > 0 {
		rec = s.procByPID[pid]
		delete(s.procByPID, pid)
	}

	if name == "" && rec != nil {
		name = rec.name
	}

	if name == "" {
		return
	}

	// Only delete if the record matches. Names are stable but this keeps the
	// index consistent if a future restart ever reuses the same name.
	if rec != nil {
		if cur := s.procByName[name]; cur == rec {
			delete(s.procByName, name)
		}
		return
	}

	rec = s.procByName[name]
	delete(s.procByName, name)
	if rec != nil && rec.pid > 0 {
		if cur := s.procByPID[rec.pid]; cur == rec {
			delete(s.procByPID, rec.pid)
		}
	}
}

func (s *controllerState) snapshotProcRecords() []procRecordSnapshot {
	if s == nil || len(s.procByPID) == 0 {
		return nil
	}

	out := make([]procRecordSnapshot, 0, len(s.procByPID))
	for pid, rec := range s.procByPID {
		if pid <= 0 || rec == nil || rec.inst == nil {
			continue
		}
		out = append(out, procRecordSnapshot{
			ServiceID: rec.serviceID,
			Name:      rec.name,
			PID:       pid,
			Inst:      rec.inst,
			Removed:   rec.removedFromProcs,
		})
	}
	return out
}
