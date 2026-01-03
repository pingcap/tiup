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

func (p *Playground) handleScaleIn(w io.Writer, req *ScaleInRequest) error {
	if req == nil {
		return fmt.Errorf("missing scale_in request")
	}

	targetName := strings.TrimSpace(req.Name)
	targetPID := req.PID
	if targetName == "" && targetPID <= 0 {
		return fmt.Errorf("scale-in requires --name (or --pid)")
	}

	var (
		serviceID proc.ServiceID
		inst      proc.Process
		pid       int
	)
	err := p.WalkProcs(func(wServiceID proc.ServiceID, winst proc.Process) error {
		if winst == nil {
			return nil
		}
		info := winst.Info()
		if info == nil {
			return nil
		}
		if targetName != "" {
			if info.Name() != targetName {
				return nil
			}
			serviceID = wServiceID
			inst = winst
			if info.Proc != nil {
				pid = info.Proc.Pid()
			}
			return nil
		}
		if info.Proc != nil && info.Proc.Pid() == targetPID {
			serviceID = wServiceID
			inst = winst
			pid = targetPID
		}
		return nil
	})
	if err != nil {
		return err
	}

	if inst == nil {
		if targetName != "" {
			return fmt.Errorf("no instance found with name %q", targetName)
		}
		return fmt.Errorf("no instance found with pid %d", targetPID)
	}
	if pid <= 0 {
		name := targetName
		if name == "" {
			if info := inst.Info(); info != nil {
				name = info.Name()
			}
		}
		if name != "" {
			return fmt.Errorf("instance %q is not running", name)
		}
		return fmt.Errorf("instance pid %d is not running", targetPID)
	}

	spec, ok := pgservice.SpecFor(serviceID)
	if !ok {
		return fmt.Errorf("unknown service %s", serviceID)
	}

	if hook := spec.ScaleInHook; hook != nil {
		async, err := hook(p, w, inst, pid)
		if err != nil {
			return err
		}
		if async {
			return nil
		}
	}

	if _, ok := p.removeProcByPID(serviceID, pid); !ok {
		return fmt.Errorf("instance %d already removed", pid)
	}

	// Refresh title counts so subsequent output (e.g. shutdown) can show indices
	// for components that now have multiple instances (or dropped back to one).
	counts := p.buildProcTitleCounts()
	p.progressMu.Lock()
	p.procTitleCounts = counts
	p.progressMu.Unlock()

	p.expectExitPID(pid)
	err = syscall.Kill(pid, syscall.SIGQUIT)
	if err != nil && err != syscall.ESRCH {
		return errors.AddStack(err)
	}

	logIfErr(p.renderSDFile())

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

func (p *Playground) handleScaleOut(w io.Writer, req *ScaleOutRequest) error {
	if p == nil {
		return fmt.Errorf("playground is nil")
	}
	if req == nil {
		return fmt.Errorf("missing scale_out request")
	}
	if req.Count <= 0 {
		return fmt.Errorf("scale-out count must be greater than 0")
	}

	serviceID := req.ServiceID
	if serviceID == "" {
		return fmt.Errorf("missing scale-out service")
	}
	if def, ok := serviceDefFor(serviceID); !ok {
		return fmt.Errorf("unknown service %s", serviceID)
	} else if !def.ScaleOut {
		return fmt.Errorf("service %q does not support scale-out", serviceID)
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

	startCtx := context.WithValue(context.Background(), logprinter.ContextKeyLogger, log)
	for i := 0; i < req.Count; i++ {
		inst, err := p.addProc(serviceID, cfg)
		if err != nil {
			return err
		}

		if _, err := p.startProcInController(
			startCtx,
			inst,
		); err != nil {
			return err
		}
		if post := spec.PostScaleOut; post != nil {
			post(w, inst)
		}
	}

	// Refresh title counts so subsequent output (e.g. shutdown) can show indices
	// for components that now have multiple instances (or dropped back to one).
	counts := p.buildProcTitleCounts()
	p.progressMu.Lock()
	p.procTitleCounts = counts
	p.progressMu.Unlock()

	logIfErr(p.renderSDFile())
	return nil
}
