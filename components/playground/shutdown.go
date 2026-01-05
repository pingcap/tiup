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
const hideInProgressRevealAfterShutdown = 2 * time.Second

type stopCause int

const (
	stopCauseInternal stopCause = iota
	stopCauseSignal
)

func (p *Playground) startShutdown(cause stopCause, sig syscall.Signal) {
	p.startShutdownWithControllerState(nil, cause, sig)
}

func (p *Playground) startShutdownWithControllerState(state *controllerState, cause stopCause, sig syscall.Signal) {
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

		var procRecords []procRecordSnapshot
		if state != nil {
			procRecords = state.snapshotProcRecords()
		} else {
			procRecords = p.procRecordsSnapshot()
		}
		p.shutdownProcRecords = procRecords

		// Stop accepting new commands/events as early as possible to keep
		// lifecycle predictable, after collecting the controller-owned state we
		// need for termination.
		if p.controllerCancel != nil {
			p.controllerCancel()
		}
		if p.processGroup != nil {
			p.processGroup.Close()
		}

		// Freeze any in-progress boot UI so it stops redrawing while we terminate.
		p.abandonActiveGroupsWithStartedRecords(procRecords)

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
			p.terminateGracefully(p.shutdownProcRecords)
		}()
	})
}

func (p *Playground) forceKillShutdown() {
	p.forceKillShutdownWithControllerState(nil)
}

func (p *Playground) forceKillShutdownWithControllerState(state *controllerState) {
	if p == nil {
		return
	}

	p.startShutdownWithControllerState(state, stopCauseSignal, syscall.SIGINT)

	p.progressMu.Lock()
	shutdownGroup := p.shutdownGroup
	p.progressMu.Unlock()
	if shutdownGroup != nil {
		shutdownGroup.SetTitle("Shutdown (force killed)")
	}

	go p.terminateForceKill(p.shutdownProcRecords)
}

func (p *Playground) addWaitProc(inst proc.Process) chan error {
	exitCh := make(chan error, 1)

	info := (*proc.ProcessInfo)(nil)
	name := ""
	if inst != nil {
		info = inst.Info()
		if info != nil {
			name = info.Name()
		}
	}

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
		select {
		case exitCh <- err:
		default:
		}
		close(exitCh)

		pid := 0
		if osProc != nil {
			pid = osProc.Pid()
		}

		decCh := make(chan procExitDecision, 1)
		p.emitEvent(procExitedEvent{inst: inst, pid: pid, err: err, respCh: decCh})

		expectedExit := false
		select {
		case dec := <-decCh:
			expectedExit = dec.expectedExit
		case <-p.controllerDoneCh:
			// If the controller is gone (context canceled / shutdown), treat
			// the exit as expected to avoid leaking errors.
			expectedExit = true
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
			return exitCh
		}
	}
	go func() { _ = waiter() }()
	return exitCh
}

func (p *Playground) shutdownProcTitle(inst proc.Process) string {
	if inst == nil {
		return "Instance"
	}

	title := procTitle(inst)
	p.progressMu.Lock()
	counts := p.procTitleCounts
	p.progressMu.Unlock()
	includeID := true
	if counts != nil {
		includeID = counts[title] > 1
	}
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

func (p *Playground) terminateGracefully(records []procRecordSnapshot) {
	// Prevent late additions (scale-out waiters, monitors, etc.) from extending
	// the lifetime of the process while we are shutting down.
	if p != nil && p.processGroup != nil {
		p.processGroup.Close()
	}

	type shutdownTarget struct {
		serviceID proc.ServiceID
		title     string
		pid       int
		wait      func() error
		task      *progressv2.Task
	}

	p.progressMu.Lock()
	shutdownGroup := p.shutdownGroup
	p.progressMu.Unlock()

	targets := func() []shutdownTarget {
		byService := make(map[proc.ServiceID][]shutdownTarget)
		var serviceIDs []proc.ServiceID

		add := func(serviceID proc.ServiceID, inst proc.Process, pid int) {
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
			if pid <= 0 {
				pid = osProc.Pid()
			}
			if pid <= 0 {
				return
			}
			if serviceID == "" {
				serviceID = info.Service
			}
			if serviceID == "" {
				return
			}

			if _, ok := byService[serviceID]; !ok {
				serviceIDs = append(serviceIDs, serviceID)
			}
			byService[serviceID] = append(byService[serviceID], shutdownTarget{
				serviceID: serviceID,
				title:     p.shutdownProcTitle(inst),
				pid:       pid,
				wait:      osProc.Wait,
			})
		}

		for _, rec := range records {
			add(rec.ServiceID, rec.Inst, rec.PID)
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
			applyHideInProgressPolicy(t, targets[i].serviceID, hideInProgressRevealAfterShutdown)
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

func (p *Playground) terminateForceKill(records []procRecordSnapshot) {
	if p == nil {
		return
	}
	if p.processGroup != nil {
		p.processGroup.Close()
	}

	for _, rec := range records {
		pid := rec.PID
		if pid <= 0 && rec.Inst != nil {
			info := rec.Inst.Info()
			if info != nil && info.Proc != nil {
				pid = info.Proc.Pid()
			}
		}
		if pid <= 0 {
			continue
		}
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
