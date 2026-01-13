package main

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"

	"slices"

	"github.com/pingcap/tiup/components/playground-ng/proc"
	pgservice "github.com/pingcap/tiup/components/playground-ng/service"
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

func (p *Playground) requestAddPlannedProc(ctx context.Context, plan ServicePlan, binPath string, version utils.Version, shared proc.SharedOptions, dataDir string) (proc.Process, error) {
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
	p.emitEvent(addPlannedProcRequest{
		plan:    plan,
		shared:  shared,
		dataDir: dataDir,
		binPath: binPath,
		version: version,
		respCh:  respCh,
	})
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

func (p *Playground) addPlannedProcInController(state *controllerState, plan ServicePlan, binPath string, version utils.Version, shOpt proc.SharedOptions, dataDir string) (proc.Process, error) {
	if p == nil || state == nil {
		return nil, fmt.Errorf("playground controller state is nil")
	}

	serviceID := proc.ServiceID(strings.TrimSpace(plan.ServiceID))
	if serviceID == "" {
		return nil, fmt.Errorf("planned service id is empty")
	}

	id := state.allocID(serviceID)
	name := fmt.Sprintf("%s-%d", serviceID, id)
	if plan.Name != "" && plan.Name != name {
		return nil, fmt.Errorf("planned name mismatch: expect %q, got %q", name, plan.Name)
	}

	baseDir := strings.TrimSpace(dataDir)
	if baseDir == "" {
		baseDir = p.dataDir
	}
	if baseDir == "" {
		return nil, fmt.Errorf("planned data dir is empty for %s", name)
	}
	if p.dataDir != "" && strings.TrimSpace(dataDir) != "" && filepath.Clean(p.dataDir) != filepath.Clean(dataDir) {
		return nil, fmt.Errorf("planned data dir mismatch: runtime=%q planned=%q", p.dataDir, dataDir)
	}

	dir := filepath.Join(baseDir, name)
	if plan.Shared.Dir != "" && plan.Shared.Dir != dir {
		return nil, fmt.Errorf("planned dir mismatch: expect %q, got %q", dir, plan.Shared.Dir)
	}
	if err := utils.MkdirAll(dir, 0o755); err != nil {
		return nil, err
	}

	host := strings.TrimSpace(plan.Shared.Host)

	info := proc.ProcessInfo{
		ID:              id,
		Dir:             dir,
		Host:            host,
		Port:            plan.Shared.Port,
		StatusPort:      plan.Shared.StatusPort,
		UpTimeout:       plan.Shared.UpTimeout,
		ConfigPath:      plan.Shared.ConfigPath,
		UserBinPath:     plan.BinPath,
		BinPath:         binPath,
		Version:         version,
		RepoComponentID: proc.RepoComponentID(plan.ComponentID),
		Service:         serviceID,
	}
	if info.BinPath == "" && info.UserBinPath != "" {
		info.BinPath = info.UserBinPath
	}

	inst, err := proc.NewProcessFromPlan(plan, info, shOpt, baseDir)
	if err != nil {
		return nil, err
	}
	if inst == nil {
		return nil, fmt.Errorf("failed to create instance for %s", name)
	}
	state.appendProc(serviceID, inst)
	return inst, nil
}

func (p *Playground) versionConstraintForService(serviceID proc.ServiceID, bootVersion string) string {
	constraint := bootVersion

	if p == nil {
		return constraint
	}
	return serviceVersionConstraint(serviceID, bootVersion, p.bootOptions)
}
