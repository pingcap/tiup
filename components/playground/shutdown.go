package main

import (
	"fmt"
	"strings"
	"sync"
	"syscall"
	"time"

	"slices"

	"github.com/pingcap/tiup/components/playground/proc"
	progressv2 "github.com/pingcap/tiup/pkg/tuiv2/progress"
)

// The duration process need to quit gracefully, or we kill the process.
const forceKillAfterDuration = time.Second * 10

type stopCause int

const (
	stopCauseInternal stopCause = iota
	stopCauseSignal
)

func (p *Playground) expectExitPID(pid int) {
	if p == nil || pid <= 0 {
		return
	}
	if p.expectedExit == nil {
		p.expectedExit = make(map[int]struct{})
	}
	p.expectedExit[pid] = struct{}{}
}

func (p *Playground) startShutdown(cause stopCause, sig syscall.Signal) {
	if p == nil {
		return
	}
	p.shutdownOnce.Do(func() {
		if p.stoppingCh != nil {
			close(p.stoppingCh)
		}
		if cause == stopCauseSignal && p.interruptedCh != nil {
			close(p.interruptedCh)
		}

		// Freeze any in-progress boot UI so it stops redrawing while we terminate.
		p.abandonActiveGroups()

		if cause == stopCauseSignal && sig != 0 {
			printInterrupt(p.ui, sig)
		}

		p.progressMu.Lock()
		ui := p.ui
		if ui != nil && p.shutdownGroup == nil {
			title := "Shutdown"
			if cause == stopCauseSignal {
				title = "Shutdown gracefully"
			}
			p.shutdownGroup = ui.Group(title)
			p.shutdownGroup.SetShowMeta(false)
			p.shutdownGroup.SetHideDetailsOnSuccess(true)
		}
		p.progressMu.Unlock()

		go func() {
			defer p.terminateDoneOnce.Do(func() {
				close(p.terminateDoneCh)
			})
			p.terminateGracefully()
		}()
	})
}

func (p *Playground) forceKillShutdown() {
	if p == nil {
		return
	}

	p.startShutdown(stopCauseSignal, syscall.SIGINT)

	p.progressMu.Lock()
	shutdownGroup := p.shutdownGroup
	p.progressMu.Unlock()
	if shutdownGroup != nil {
		shutdownGroup.SetTitle("Shutdown (force killed)")
	}

	go p.terminateForceKill()
}

func (p *Playground) addWaitProc(inst proc.Process, exitCh chan error) {
	info := (*proc.ProcessInfo)(nil)
	name := ""
	if inst != nil {
		info = inst.Info()
		name = info.Name()
	}

	p.progressMu.Lock()
	p.startedProcs = append(p.startedProcs, inst)
	p.progressMu.Unlock()

	waiter := func() error {
		var err error
		var osProc proc.OSProcess
		if info != nil {
			osProc = info.Proc
		}
		if osProc == nil {
			err = fmt.Errorf("process not prepared for %s", name)
		} else {
			err = osProc.Wait()
		}

		// Notify any readiness check that might be waiting so it can stop early.
		if exitCh != nil {
			select {
			case exitCh <- err:
			default:
			}
			close(exitCh)
		}

		pid := 0
		if osProc != nil {
			pid = osProc.Pid()
		}

		decCh := make(chan procExitDecision, 1)
		p.emitEvent(procExitedEvent{inst: inst, pid: pid, err: err, respCh: decCh})

		expectedExit := false
		if decCh != nil {
			select {
			case dec := <-decCh:
				expectedExit = dec.expectedExit
			case <-p.controllerDoneCh:
				// If the controller is gone (context canceled / shutdown), treat
				// the exit as expected to avoid leaking errors.
				expectedExit = true
			}
		}

		if expectedExit {
			return nil
		}
		return err
	}

	nameForErr := name
	if nameForErr == "" {
		if info != nil {
			nameForErr = info.Service.String()
		}
	}
	if p != nil && p.processGroup != nil {
		if err := p.processGroup.Add(nameForErr, waiter); err == nil {
			return
		}
	}
	go func() { _ = waiter() }()
}

func (p *Playground) shutdownProcTitle(inst proc.Process) string {
	if inst == nil {
		return "Instance"
	}

	// Only show the instance index when there are multiple instances with the same
	// display title. This keeps common cases (single TiKV worker, etc.) compact.
	if p != nil && p.procTitleCounts == nil {
		p.progressMu.Lock()
		if p.procTitleCounts == nil {
			p.procTitleCounts = p.buildProcTitleCounts()
		}
		p.progressMu.Unlock()
	}
	title := procTitle(inst)
	p.progressMu.Lock()
	counts := p.procTitleCounts
	p.progressMu.Unlock()
	includeID := counts != nil && counts[title] > 1
	return procDisplayName(inst, includeID)
}

// Wait all instance quit and return the first non-nil err.
// including p8s & grafana
func (p *Playground) wait() error {
	var err error
	if p != nil && p.processGroup != nil {
		err = p.processGroup.Wait()
	}
	if err != nil && !p.interrupted() {
		return err
	}
	if p != nil && p.terminateDoneCh != nil {
		<-p.terminateDoneCh
	}

	return nil
}

func (p *Playground) terminateGracefully() {
	// Prevent late additions (scale-out waiters, monitors, etc.) from extending
	// the lifetime of the process while we are shutting down.
	if p != nil && p.processGroup != nil {
		p.processGroup.Close()
	}

	type shutdownTarget struct {
		title string
		pid   int
		wait  func() error
		task  *progressv2.Task
	}

	p.progressMu.Lock()
	startedProcs := append([]proc.Process(nil), p.startedProcs...)
	shutdownGroup := p.shutdownGroup
	p.progressMu.Unlock()

	targets := func() []shutdownTarget {
		seenPIDs := make(map[int]struct{})
		byService := make(map[proc.ServiceID][]shutdownTarget)
		var serviceIDs []proc.ServiceID

		add := func(inst proc.Process) {
			if inst == nil {
				return
			}
			info := inst.Info()
			if info == nil {
				return
			}
			osProc := info.Proc
			if osProc == nil || osProc.Cmd() == nil || osProc.Cmd().Process == nil {
				return
			}

			pid := osProc.Pid()
			if pid <= 0 {
				return
			}
			if _, ok := seenPIDs[pid]; ok {
				return
			}
			seenPIDs[pid] = struct{}{}

			serviceID := info.Service
			if _, ok := byService[serviceID]; !ok {
				serviceIDs = append(serviceIDs, serviceID)
			}
			byService[serviceID] = append(byService[serviceID], shutdownTarget{
				title: p.shutdownProcTitle(inst),
				pid:   pid,
				wait:  osProc.Wait,
			})
		}

		for _, inst := range startedProcs {
			add(inst)
		}

		ordered, err := topoSortServiceIDs(serviceIDs)
		if err != nil {
			ordered = append([]proc.ServiceID(nil), serviceIDs...)
			slices.SortStableFunc(ordered, func(a, b proc.ServiceID) int {
				return strings.Compare(a.String(), b.String())
			})
		}
		slices.Reverse(ordered)

		var out []shutdownTarget
		for _, serviceID := range ordered {
			items := byService[serviceID]
			slices.SortStableFunc(items, func(a, b shutdownTarget) int {
				if a.title < b.title {
					return -1
				}
				if a.title > b.title {
					return 1
				}
				return 0
			})
			out = append(out, items...)
		}
		return out
	}()

	if shutdownGroup != nil {
		// Pre-create tasks so the shutdown list is complete from the beginning,
		// without implying everything is already in progress.
		for i := range targets {
			t := shutdownGroup.TaskPending(targets[i].title)
			t.SetMessage(fmt.Sprintf("pid=%d", targets[i].pid))
			targets[i].task = t
		}
	}

	// Send stop signals first (in dependency-derived order) so fast-exiting
	// processes can quit early even if some components take longer.
	for i := range targets {
		t := targets[i]
		task := t.task
		if shutdownGroup != nil {
			if task == nil {
				task = shutdownGroup.TaskPending(t.title)
				task.SetMessage(fmt.Sprintf("pid=%d", t.pid))
			}
			task.Start()
		}

		_ = syscall.Kill(t.pid, syscall.SIGTERM)
	}

	var wg sync.WaitGroup
	for _, t := range targets {
		wg.Add(1)
		go func(t shutdownTarget) {
			defer wg.Done()

			timer := time.AfterFunc(forceKillAfterDuration, func() {
				_ = syscall.Kill(t.pid, syscall.SIGKILL)
			})
			// On shutdown, process may exit with non-zero due to the signal.
			// Consider it a successful shutdown once it quits.
			_ = t.wait()
			timer.Stop()

			if shutdownGroup != nil && t.task != nil {
				t.task.Done()
			}
		}(t)
	}
	wg.Wait()

	if shutdownGroup != nil {
		shutdownGroup.Close()
		p.progressMu.Lock()
		if p.shutdownGroup == shutdownGroup {
			p.shutdownGroup = nil
		}
		p.progressMu.Unlock()
	}
}

func (p *Playground) terminateForceKill() {
	if p == nil {
		return
	}
	if p.processGroup != nil {
		p.processGroup.Close()
	}

	p.progressMu.Lock()
	startedProcs := append([]proc.Process(nil), p.startedProcs...)
	p.progressMu.Unlock()

	seenPIDs := make(map[int]struct{})
	for _, inst := range startedProcs {
		if inst == nil {
			continue
		}
		info := inst.Info()
		if info == nil || info.Proc == nil {
			continue
		}
		pid := info.Proc.Pid()
		if pid <= 0 {
			continue
		}
		if _, ok := seenPIDs[pid]; ok {
			continue
		}
		seenPIDs[pid] = struct{}{}
		_ = syscall.Kill(pid, syscall.SIGKILL)
	}
}

func (p *Playground) interrupted() bool {
	if p == nil {
		return false
	}
	ch := p.interruptedCh
	if ch == nil {
		return false
	}
	select {
	case <-ch:
		return true
	default:
		return false
	}
}
