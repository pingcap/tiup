package main

import (
	"fmt"
	"path/filepath"
	"strings"

	"slices"

	"github.com/pingcap/tiup/components/playground/proc"
	pgservice "github.com/pingcap/tiup/components/playground/service"
	"github.com/pingcap/tiup/pkg/utils"
)

func (p *Playground) allocID(serviceID proc.ServiceID) int {
	id := p.idAlloc[serviceID]
	p.idAlloc[serviceID] = id + 1
	return id
}

func (p *Playground) appendProc(serviceID proc.ServiceID, inst proc.Process) {
	if p == nil || inst == nil || serviceID == "" {
		return
	}
	info := inst.Info()
	if info == nil {
		panic(fmt.Sprintf("append proc %T has nil info", inst))
	}
	if info.Service != serviceID {
		panic(fmt.Sprintf("append proc service mismatch: expect %s, got %s", serviceID, info.Service))
	}
	if p.procs == nil {
		p.procs = make(map[proc.ServiceID][]proc.Process)
	}
	p.procs[serviceID] = append(p.procs[serviceID], inst)
}

func (p *Playground) removeProcByPID(serviceID proc.ServiceID, pid int) (proc.Process, bool) {
	if p == nil || pid <= 0 || serviceID == "" {
		return nil, false
	}

	list := p.procs[serviceID]
	for i := 0; i < len(list); i++ {
		inst := list[i]
		if inst == nil {
			continue
		}
		info := inst.Info()
		if info == nil || info.Proc == nil || info.Proc.Pid() != pid {
			continue
		}
		p.procs[serviceID] = slices.Delete(list, i, i+1)
		return inst, true
	}
	return nil, false
}

func (p *Playground) removeProc(serviceID proc.ServiceID, inst proc.Process) bool {
	if p == nil || inst == nil || serviceID == "" {
		return false
	}

	list := p.procs[serviceID]
	for i := 0; i < len(list); i++ {
		if list[i] != inst {
			continue
		}
		p.procs[serviceID] = slices.Delete(list, i, i+1)
		return true
	}
	return false
}

// RWalkProcs works like WalkProcs, but in reverse order.
func (p *Playground) RWalkProcs(fn func(serviceID proc.ServiceID, ins proc.Process) error) error {
	var ids []proc.ServiceID
	var procs []proc.Process

	_ = p.WalkProcs(func(id proc.ServiceID, ins proc.Process) error {
		ids = append(ids, id)
		procs = append(procs, ins)
		return nil
	})

	for i := len(ids); i > 0; i-- {
		err := fn(ids[i-1], procs[i-1])
		if err != nil {
			return err
		}
	}
	return nil
}

// WalkProcs calls fn for every process and stops if it returns a non-nil error.
func (p *Playground) WalkProcs(fn func(serviceID proc.ServiceID, ins proc.Process) error) error {
	if p == nil || fn == nil {
		return nil
	}

	serviceIDs := make([]proc.ServiceID, 0, len(p.procs))
	for serviceID := range p.procs {
		serviceIDs = append(serviceIDs, serviceID)
	}
	slices.SortFunc(serviceIDs, func(a, b proc.ServiceID) int {
		return strings.Compare(a.String(), b.String())
	})

	for _, serviceID := range serviceIDs {
		for _, ins := range p.procs[serviceID] {
			if err := fn(serviceID, ins); err != nil {
				return err
			}
		}
	}
	return nil
}

func (p *Playground) addProc(serviceID proc.ServiceID, cfg proc.Config) (ins proc.Process, err error) {
	if cfg.BinPath != "" {
		cfg.BinPath, err = getAbsolutePath(cfg.BinPath)
		if err != nil {
			return nil, err
		}
	}

	if cfg.ConfigPath != "" {
		cfg.ConfigPath, err = getAbsolutePath(cfg.ConfigPath)
		if err != nil {
			return nil, err
		}
	}

	spec, ok := pgservice.SpecFor(serviceID)
	if !ok {
		return nil, fmt.Errorf("unknown service %s", serviceID)
	}

	id := p.allocID(serviceID)
	dir := filepath.Join(p.dataDir, fmt.Sprintf("%s-%d", serviceID, id))
	if err = utils.MkdirAll(dir, 0755); err != nil {
		return nil, err
	}
	// look more like listen ip?
	host := p.bootOptions.Host
	if cfg.Host != "" {
		host = cfg.Host
	}

	return spec.NewProc(p, pgservice.NewProcParams{Config: cfg, ID: id, Dir: dir, Host: host})
}

func (p *Playground) bindVersion(comp string, version string) (bindVersion string) {
	bindVersion = version
	binder := componentVersionBinders[proc.RepoComponentID(comp)]
	if binder == nil {
		return bindVersion
	}
	return binder(p, version)
}
