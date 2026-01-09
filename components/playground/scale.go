package main

import (
	"context"
	"fmt"
	"io"
	"strings"
	"syscall"

	"github.com/pingcap/errors"
	"github.com/pingcap/tiup/components/playground/proc"
	pgservice "github.com/pingcap/tiup/components/playground/service"
	logprinter "github.com/pingcap/tiup/pkg/logger/printer"
)

func (p *Playground) handleScaleIn(state *controllerState, w io.Writer, req *ScaleInRequest) error {
	if req == nil {
		return fmt.Errorf("missing scale_in request")
	}
	if state == nil {
		return fmt.Errorf("playground controller state is nil")
	}

	targetName := strings.TrimSpace(req.Name)
	targetPID := req.PID
	if targetName != "" && targetPID > 0 {
		return fmt.Errorf("scale-in expects exactly one of --name or --pid")
	}
	if targetName == "" && targetPID <= 0 {
		return fmt.Errorf("scale-in requires --name or --pid")
	}

	var (
		serviceID proc.ServiceID
		inst      proc.Process
		pid       int
	)

	switch {
	case targetPID > 0:
		pid = targetPID
		if rec := state.procByPID[pid]; rec != nil && rec.inst != nil {
			if rec.removedFromProcs {
				return fmt.Errorf("instance %d already removed", pid)
			}
			serviceID = rec.serviceID
			inst = rec.inst
		}
		if inst == nil {
			err := state.walkProcs(func(wServiceID proc.ServiceID, winst proc.Process) error {
				if winst == nil {
					return nil
				}
				info := winst.Info()
				if info == nil || info.Proc == nil {
					return nil
				}
				if info.Proc.Pid() != pid {
					return nil
				}
				serviceID = wServiceID
				inst = winst
				return nil
			})
			if err != nil {
				return err
			}
		}
		if inst == nil {
			return fmt.Errorf("no instance found with pid %d", pid)
		}
	default:
		if rec := state.procByName[targetName]; rec != nil && rec.inst != nil {
			if rec.removedFromProcs {
				if rec.pid > 0 {
					return fmt.Errorf("instance %d already removed", rec.pid)
				}
				return fmt.Errorf("instance %q already removed", targetName)
			}
			serviceID = rec.serviceID
			inst = rec.inst
			pid = rec.pid
		}
		if inst == nil {
			err := state.walkProcs(func(wServiceID proc.ServiceID, winst proc.Process) error {
				if winst == nil {
					return nil
				}
				info := winst.Info()
				if info == nil {
					return nil
				}
				if info.Name() != targetName {
					return nil
				}
				serviceID = wServiceID
				inst = winst
				if info.Proc != nil {
					pid = info.Proc.Pid()
				}
				return nil
			})
			if err != nil {
				return err
			}
		}
		if inst == nil {
			return fmt.Errorf("no instance found with name %q", targetName)
		}
	}
	if pid <= 0 {
		if targetPID > 0 {
			return fmt.Errorf("instance %d is not running", targetPID)
		}
		return fmt.Errorf("instance %q is not running", targetName)
	}

	spec, ok := pgservice.SpecFor(serviceID)
	if !ok {
		return fmt.Errorf("unknown service %s", serviceID)
	}

	async, err := spec.ScaleInHook(controllerRuntime{pg: p, state: state}, w, inst, pid)
	if err != nil {
		return err
	}
	if async {
		return nil
	}

	if _, ok := state.removeProcByPID(serviceID, pid); !ok {
		return fmt.Errorf("instance %d already removed", pid)
	}

	controllerRuntime{pg: p, state: state}.ExpectExitPID(pid)
	err = syscall.Kill(pid, syscall.SIGQUIT)
	if err != nil && err != syscall.ESRCH {
		return errors.AddStack(err)
	}

	controllerRuntime{pg: p, state: state}.OnProcsChanged()

	fmt.Fprintf(w, "scale in %s success\n", serviceID)

	return nil
}

func (p *Playground) sanitizeConfig(boot proc.Config, cfg *proc.Config) error {
	if cfg.BinPath == "" {
		cfg.BinPath = boot.BinPath
	}
	if cfg.ConfigPath == "" {
		cfg.ConfigPath = boot.ConfigPath
	}
	if cfg.Host == "" {
		cfg.Host = boot.Host
	}

	path, err := getAbsolutePath(cfg.ConfigPath)
	if err != nil {
		return err
	}
	cfg.ConfigPath = path
	return nil
}

func (p *Playground) sanitizeServiceConfig(serviceID proc.ServiceID, cfg *proc.Config) error {
	if p == nil || p.bootOptions == nil {
		return fmt.Errorf("playground not initialized")
	}
	base, ok := p.BootConfig(serviceID)
	if !ok {
		return fmt.Errorf("service %q does not support scale-out", serviceID)
	}
	return p.sanitizeConfig(base, cfg)
}

func (p *Playground) handleScaleOut(state *controllerState, w io.Writer, req *ScaleOutRequest) error {
	if p == nil {
		return fmt.Errorf("playground is nil")
	}
	if req == nil {
		return fmt.Errorf("missing scale_out request")
	}
	if state == nil {
		return fmt.Errorf("playground controller state is nil")
	}
	if req.Count <= 0 {
		return fmt.Errorf("scale-out count must be greater than 0")
	}

	serviceID := req.ServiceID
	if serviceID == "" {
		return fmt.Errorf("missing scale-out service")
	}

	cfg := req.Config
	cfg.Num = 0
	if err := p.sanitizeServiceConfig(serviceID, &cfg); err != nil {
		return err
	}
	spec, ok := pgservice.SpecFor(serviceID)
	if !ok {
		return fmt.Errorf("unknown service %s", serviceID)
	}
	if !spec.Catalog.AllowScaleOut {
		return fmt.Errorf("service %q does not support scale-out", serviceID)
	}

	startCtx := context.WithValue(context.Background(), logprinter.ContextKeyLogger, log)
	for i := 0; i < req.Count; i++ {
		inst, err := p.addProcInController(state, serviceID, cfg)
		if err != nil {
			return err
		}

		if _, err := p.startProc(startCtx, state, inst, nil); err != nil {
			return err
		}
		spec.PostScaleOut(w, inst)
	}

	controllerRuntime{pg: p, state: state}.OnProcsChanged()
	return nil
}
