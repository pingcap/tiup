package main

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"

	"slices"

	"github.com/pingcap/tiup/components/playground/proc"
	pgservice "github.com/pingcap/tiup/components/playground/service"
	"github.com/pingcap/tiup/pkg/utils"
)

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

	snapshot := p.procsSnapshot()
	serviceIDs := make([]proc.ServiceID, 0, len(snapshot))
	for serviceID := range snapshot {
		serviceIDs = append(serviceIDs, serviceID)
	}
	slices.SortFunc(serviceIDs, func(a, b proc.ServiceID) int {
		return strings.Compare(a.String(), b.String())
	})

	for _, serviceID := range serviceIDs {
		for _, ins := range snapshot[serviceID] {
			if err := fn(serviceID, ins); err != nil {
				return err
			}
		}
	}
	return nil
}

func (p *Playground) procsSnapshot() map[proc.ServiceID][]proc.Process {
	if p == nil || p.evtCh == nil {
		return nil
	}
	respCh := make(chan map[proc.ServiceID][]proc.Process, 1)
	p.emitEvent(procsSnapshotRequest{respCh: respCh})
	select {
	case m := <-respCh:
		return m
	case <-p.controllerDoneCh:
		return nil
	}
}

func (p *Playground) procRecordsSnapshot() []procRecordSnapshot {
	if p == nil || p.evtCh == nil {
		return nil
	}
	respCh := make(chan []procRecordSnapshot, 1)
	p.emitEvent(procRecordsSnapshotRequest{respCh: respCh})
	select {
	case records := <-respCh:
		return records
	case <-p.controllerDoneCh:
		return nil
	}
}

func (p *Playground) requestAddProc(ctx context.Context, serviceID proc.ServiceID, cfg proc.Config) (proc.Process, error) {
	if p == nil {
		return nil, context.Canceled
	}
	if ctx == nil {
		ctx = context.Background()
	}
	if p.evtCh == nil {
		return nil, fmt.Errorf("controller not started")
	}

	respCh := make(chan addProcResponse, 1)
	p.emitEvent(addProcRequest{serviceID: serviceID, cfg: cfg, respCh: respCh})
	select {
	case resp := <-respCh:
		return resp.inst, resp.err
	case <-p.controllerDoneCh:
		return nil, fmt.Errorf("playground is stopping")
	case <-ctx.Done():
		return nil, ctx.Err()
	}
}

func (p *Playground) addProcInController(state *controllerState, serviceID proc.ServiceID, cfg proc.Config) (ins proc.Process, err error) {
	if p == nil || state == nil {
		return nil, fmt.Errorf("playground controller state is nil")
	}
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

	id := state.allocID(serviceID)
	dir := filepath.Join(p.dataDir, fmt.Sprintf("%s-%d", serviceID, id))
	if err = utils.MkdirAll(dir, 0755); err != nil {
		return nil, err
	}
	// look more like listen ip?
	host := ""
	if p.bootOptions != nil {
		host = p.bootOptions.Host
	}
	if cfg.Host != "" {
		host = cfg.Host
	}

	return spec.NewProc(controllerRuntime{pg: p, state: state}, pgservice.NewProcParams{Config: cfg, ID: id, Dir: dir, Host: host})
}

func (p *Playground) versionConstraintForService(serviceID proc.ServiceID, bootVersion string) string {
	constraint := bootVersion

	if p == nil {
		return constraint
	}

	spec, ok := pgservice.SpecFor(serviceID)
	if !ok {
		return constraint
	}

	if spec.Catalog.AllowModifyVersion && p.bootOptions != nil {
		if cfg := p.bootOptions.Service(serviceID); cfg != nil && cfg.Version != "" {
			constraint = cfg.Version
		}
	}

	if bind := spec.Catalog.VersionBind; bind != nil {
		constraint = bind(constraint)
	}

	return constraint
}
