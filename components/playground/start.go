package main

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/pingcap/errors"
	"github.com/pingcap/tiup/components/playground/proc"
	pgservice "github.com/pingcap/tiup/components/playground/service"
	"github.com/pingcap/tiup/pkg/environment"
	"github.com/pingcap/tiup/pkg/repository"
	progressv2 "github.com/pingcap/tiup/pkg/tuiv2/progress"
	"github.com/pingcap/tiup/pkg/utils"
)

type progressTask interface {
	SetMeta(meta string)
	Start()
	Done()
	Error(msg string)
	Cancel(reason string)
}

type hideIfFastProgressTask interface {
	SetHideIfFast(revealAfter time.Duration)
}

const hideInProgressRevealAfterStart = 5 * time.Second

func applyHideInProgressPolicy(task progressTask, serviceID proc.ServiceID, revealAfter time.Duration) {
	if task == nil || serviceID == "" {
		return
	}
	spec, ok := pgservice.SpecFor(serviceID)
	if !ok || !spec.Catalog.HideInProgress {
		return
	}
	if t, ok := task.(hideIfFastProgressTask); ok {
		t.SetHideIfFast(revealAfter)
	}
}

var readyOKCh = func() <-chan error {
	ch := make(chan error)
	close(ch)
	return ch
}()

var taskStartedOKCh = func() <-chan struct{} {
	ch := make(chan struct{})
	close(ch)
	return ch
}()

func startProgressTask(task progressTask, meta string) <-chan struct{} {
	if task == nil {
		return taskStartedOKCh
	}
	started := make(chan struct{})
	go func() {
		if meta != "" {
			task.SetMeta(meta)
		}
		task.Start()
		close(started)
	}()
	return started
}

// initBootStartingTasks pre-creates one pending task per planned process so the
// "Starting instances" group can show a stable component list from the
// beginning (including components that start later, like TiFlash).
//
// It returns the created tasks map when the group is active in TTY mode.
func (p *Playground) initBootStartingTasks() map[string]progressTask {
	if p == nil {
		return nil
	}

	counts := p.buildProcTitleCounts()
	p.progressMu.Lock()
	p.procTitleCounts = counts
	startingGroup := p.startingGroup
	ui := p.ui
	p.progressMu.Unlock()

	if startingGroup == nil || ui == nil || ui.Mode() != progressv2.ModeTTY {
		return nil
	}

	bootVer := ""
	if p.bootOptions != nil {
		bootVer = p.bootOptions.Version
	}

	tasks := make(map[string]progressTask)
	_ = p.WalkProcs(func(_ proc.ServiceID, inst proc.Process) error {
		if inst == nil {
			return nil
		}
		info := inst.Info()
		if info == nil {
			return nil
		}

		title := procTitle(inst)
		includeID := counts != nil && counts[title] > 1
		t := startingGroup.TaskPending(procDisplayName(inst, includeID))
		applyHideInProgressPolicy(t, info.Service, hideInProgressRevealAfterStart)

		if bin := info.UserBinPath; bin != "" {
			// User explicitly configured a binary path, keep it as the stable
			// identity in progress output.
			t.SetMeta(prettifyUserPath(bin))
		} else {
			// If the user didn't specify a binary path, show the planned version
			// constraint from the beginning.
			ver := p.versionConstraintForService(info.Service, bootVer)
			if ver == "" {
				ver = utils.LatestVersionAlias
			}
			t.SetMeta(ver)
		}

		tasks[info.Name()] = t
		return nil
	})

	// Store tasks only if the group is still active. It may be abandoned by a
	// concurrent interrupt handler (Ctrl+C).
	p.progressMu.Lock()
	if p.startingGroup == startingGroup {
		p.startingTasks = tasks
	} else {
		tasks = nil
	}
	p.progressMu.Unlock()

	return tasks
}

func (p *Playground) closeStartingGroup() {
	if p == nil {
		return
	}

	p.progressMu.Lock()
	downloadGroup := p.downloadGroup
	startingGroup := p.startingGroup
	p.downloadGroup = nil
	p.startingGroup = nil
	p.startingTasks = nil
	p.progressMu.Unlock()

	// Close the download group first so the history output order matches the
	// actual workflow (download, then start instances).
	if downloadGroup != nil {
		downloadGroup.Close()
	}

	if startingGroup != nil {
		startingGroup.Close()
	}
}

// abandonActiveGroups freezes in-progress boot stages and moves them into the
// immutable History area in TTY mode.
//
// After calling it, the abandoned groups will no longer be updated or redrawn.
// This is primarily used when the user interrupts booting (Ctrl+C) so shutdown
// can be rendered in a separate group without interleaving progress output.
func (p *Playground) abandonActiveGroups() {
	p.abandonActiveGroupsWithStartedRecords(nil)
}

func (p *Playground) abandonActiveGroupsWithStartedRecords(procRecords []procRecordSnapshot) {
	if p == nil {
		return
	}

	p.progressMu.Lock()
	startingGroup := p.startingGroup
	startingTasks := p.startingTasks
	downloadGroup := p.downloadGroup

	p.startingGroup = nil
	p.startingTasks = nil
	p.downloadGroup = nil
	p.progressMu.Unlock()

	// Mark instances that have already been started as canceled, while keeping
	// not-yet-started ones (e.g. TiFlash) as pending in the snapshot.
	if startingTasks != nil {
		if len(procRecords) == 0 {
			procRecords = p.procRecordsSnapshot()
		}
		for _, rec := range procRecords {
			name := rec.Name
			if name == "" && rec.Inst != nil {
				if info := rec.Inst.Info(); info != nil {
					name = info.Name()
				}
			}
			if name == "" {
				continue
			}
			if t := startingTasks[name]; t != nil {
				t.Cancel("")
			}
		}
	}

	// Preserve workflow order in the history output: download, then start.
	if downloadGroup != nil {
		downloadGroup.Seal()
	}
	if startingGroup != nil {
		startingGroup.Seal()
	}
}

func (p *Playground) getOrCreateStartingTask(inst proc.Process) progressTask {
	if p == nil || inst == nil {
		return nil
	}

	name := inst.Info().Name()
	if name == "" {
		return nil
	}

	p.progressMu.Lock()
	defer p.progressMu.Unlock()

	startingGroup := p.startingGroup
	if startingGroup == nil {
		return nil
	}

	if p.startingTasks == nil {
		p.startingTasks = make(map[string]progressTask)
	}
	if t := p.startingTasks[name]; t != nil {
		return t
	}

	title := procTitle(inst)
	includeID := p.procTitleCounts != nil && p.procTitleCounts[title] > 1

	t := startingGroup.TaskPending(procDisplayName(inst, includeID))
	applyHideInProgressPolicy(t, inst.Info().Service, hideInProgressRevealAfterStart)
	p.startingTasks[name] = t
	return t
}

func (p *Playground) markStartingTaskError(inst proc.Process, meta string, err error) {
	if p == nil || inst == nil || err == nil {
		return
	}

	task := p.getOrCreateStartingTask(inst)
	if task == nil {
		return
	}

	info := inst.Info()
	hasUserBin := info != nil && info.UserBinPath != ""
	errStr := err.Error()

	// When a user provides a binary path (often long), prefer showing the error
	// message instead of repeating the path in the meta column.
	go func() {
		if hasUserBin {
			task.SetMeta("")
		} else if meta != "" {
			task.SetMeta(meta)
		}
		task.Error(errStr)
	}()
}

func (p *Playground) requestStartProc(ctx context.Context, inst proc.Process, preload *binaryPreloader) (<-chan error, error) {
	if p == nil {
		return nil, context.Canceled
	}
	if ctx == nil {
		ctx = context.Background()
	}
	if inst == nil {
		return nil, fmt.Errorf("instance is nil")
	}
	if p.evtCh == nil {
		return nil, fmt.Errorf("controller not started")
	}

	respCh := make(chan startProcResponse, 1)
	p.emitEvent(startProcRequest{ctx: ctx, inst: inst, preload: preload, respCh: respCh})
	select {
	case resp := <-respCh:
		return resp.readyCh, resp.err
	case <-p.controllerDoneCh:
		return nil, fmt.Errorf("playground is stopping")
	case <-ctx.Done():
		return nil, ctx.Err()
	}
}

func (p *Playground) startProc(state *controllerState, ctx context.Context, inst proc.Process, preload *binaryPreloader) (readyCh <-chan error, err error) {
	if p == nil || state == nil || inst == nil {
		return nil, fmt.Errorf("startProc: controller state is nil")
	}

	info := inst.Info()
	if info == nil {
		return nil, fmt.Errorf("instance %T has nil info", inst)
	}

	// Resolve binary path and version in the controller to avoid cross-goroutine
	// mutations of ProcessInfo.
	if bin := info.UserBinPath; bin != "" {
		info.BinPath = bin
		bootVer := ""
		if p.bootOptions != nil {
			bootVer = p.bootOptions.Version
		}
		constraint := p.versionConstraintForService(info.Service, bootVer)
		if constraint == "" {
			constraint = utils.LatestVersionAlias
		}
		// Use the version constraint directly for feature gates to avoid blocking
		// local binaries on repository downloads.
		info.Version = utils.Version(constraint)
	} else if info.BinPath == "" {
		component := info.RepoComponentID.String()
		bootVer := ""
		if p.bootOptions != nil {
			bootVer = p.bootOptions.Version
		}
		constraint := p.versionConstraintForService(info.Service, bootVer)
		if constraint == "" {
			constraint = utils.LatestVersionAlias
		}

		if preload != nil {
			if component == "" {
				err := fmt.Errorf("component is empty")
				p.markStartingTaskError(inst, constraint, err)
				return nil, err
			}

			readyCh := make(chan error, 1)
			serviceID := info.Service
			go func() {
				c, binPath, version, err := preload.resolve(serviceID, component)
				if err != nil {
					p.markStartingTaskError(inst, c, err)
					readyCh <- err
					close(readyCh)
					return
				}
				if ok := p.emitEvent(startProcResolvedEvent{
					ctx:     ctx,
					inst:    inst,
					binPath: binPath,
					version: version,
					readyCh: readyCh,
				}); !ok {
					err := fmt.Errorf("playground is stopping")
					readyCh <- err
					close(readyCh)
				}
			}()
			return readyCh, nil
		} else {
			v, err := environment.GlobalEnv().V1Repository().ResolveComponentVersion(component, constraint)
			if err != nil {
				p.markStartingTaskError(inst, constraint, err)
				return nil, err
			}
			forcePull := false
			if p.bootOptions != nil {
				forcePull = p.bootOptions.ShOpt.ForcePull
			}
			binPath, err := prepareComponentBinary(component, v, forcePull)
			if err != nil {
				p.markStartingTaskError(inst, constraint, err)
				return nil, err
			}
			info.BinPath = binPath
			info.Version = v
		}
	}

	return p.startProcWithControllerState(state, ctx, inst)
}

func (p *Playground) startProcWithResolvedBinary(state *controllerState, ctx context.Context, inst proc.Process, binPath string, version utils.Version, readyCh chan error) {
	if binPath == "" {
		err := fmt.Errorf("binary not resolved")
		p.markStartingTaskError(inst, "", err)
		readyCh <- err
		close(readyCh)
		return
	}
	if p.Stopping() {
		err := fmt.Errorf("playground is stopping")
		readyCh <- err
		close(readyCh)
		return
	}

	info := inst.Info()
	info.BinPath = binPath
	if !version.IsEmpty() {
		info.Version = version
	}

	startedReadyCh, err := p.startProcWithControllerState(state, ctx, inst)
	if err != nil {
		readyCh <- err
		close(readyCh)
		return
	}
	go func() {
		readyCh <- <-startedReadyCh
		close(readyCh)
	}()
}

func (p *Playground) startProcWithControllerState(state *controllerState, ctx context.Context, inst proc.Process) (readyCh <-chan error, err error) {
	if inst == nil {
		return nil, fmt.Errorf("instance is nil")
	}

	info := inst.Info()
	if info == nil || info.BinPath == "" {
		err := fmt.Errorf("binary not resolved")
		p.markStartingTaskError(inst, "", err)
		return nil, err
	}

	task := p.getOrCreateStartingTask(inst)
	meta := ""
	if task != nil {
		if bin := info.UserBinPath; bin != "" {
			meta = prettifyUserPath(bin)
		} else if v := info.Version.String(); v != "" {
			meta = v
		}
	}
	// Do not block process startup on progress rendering (e.g. heavy download
	// progress callbacks). UI updates are best-effort.
	taskStarted := startProgressTask(task, meta)

	if err := inst.Prepare(ctx); err != nil {
		p.markStartingTaskError(inst, "", err)
		return nil, err
	}

	proc := info.Proc
	if proc == nil {
		err := fmt.Errorf("process not prepared for %s", info.Name())
		p.markStartingTaskError(inst, "", err)
		return nil, err
	}

	if err := proc.SetOutputFile(inst.LogFile()); err != nil {
		p.markStartingTaskError(inst, "", err)
		return nil, err
	}

	if err := proc.Start(); err != nil {
		p.markStartingTaskError(inst, "", err)
		return nil, err
	}

	p.handleProcStarted(state, inst)

	exitCh := p.addWaitProc(inst)
	readyCh = p.startReadyCheck(ctx, inst, task, taskStarted, exitCh)
	return readyCh, nil
}

// prepareComponentBinary ensures the resolved component version is installed and
// returns its executable path.
//
// It intentionally does not print "not installed; downloading..." messages.
// Playground already owns the download UX via the unified progress UI.
func prepareComponentBinary(component string, v utils.Version, forcePull bool) (string, error) {
	env := environment.GlobalEnv()
	if env == nil {
		return "", errors.New("global environment not initialized")
	}
	return prepareComponentBinaryWithInstaller(envComponentBinaryInstaller{env: env}, component, v, forcePull)
}

type componentBinaryInstaller interface {
	BinaryPath(component string, v utils.Version) (string, error)
	UpdateComponents(specs []repository.ComponentSpec) error
}

type envComponentBinaryInstaller struct {
	env *environment.Environment
}

func (e envComponentBinaryInstaller) BinaryPath(component string, v utils.Version) (string, error) {
	if e.env == nil {
		return "", errors.New("global environment not initialized")
	}
	return e.env.BinaryPath(component, v)
}

func (e envComponentBinaryInstaller) UpdateComponents(specs []repository.ComponentSpec) error {
	if e.env == nil {
		return errors.New("global environment not initialized")
	}
	return e.env.V1Repository().UpdateComponents(specs)
}

func prepareComponentBinaryWithInstaller(inst componentBinaryInstaller, component string, v utils.Version, forcePull bool) (string, error) {
	if inst == nil {
		return "", errors.New("component binary installer is nil")
	}
	if component == "" {
		return "", errors.New("component is empty")
	}
	if v.IsEmpty() {
		return "", errors.Errorf("component `%s` version is empty", component)
	}

	binPath, err := inst.BinaryPath(component, v)
	if err == nil && !forcePull && binaryExists(binPath) {
		return binPath, nil
	}

	spec := repository.ComponentSpec{
		ID:      component,
		Version: v.String(),
		Force:   forcePull || !binaryExists(binPath),
	}
	if err := inst.UpdateComponents([]repository.ComponentSpec{spec}); err != nil {
		return "", err
	}

	binPath, err = inst.BinaryPath(component, v)
	if err != nil {
		return "", err
	}
	if !binaryExists(binPath) {
		return "", errors.Errorf("component `%s:%s` installed but binary not found at %s", component, v.String(), binPath)
	}
	return binPath, nil
}

func binaryExists(binPath string) bool {
	if binPath == "" {
		return false
	}
	st, err := os.Stat(binPath)
	if err != nil {
		return false
	}
	return !st.IsDir()
}

func (p *Playground) startReadyCheck(ctx context.Context, inst proc.Process, task progressTask, taskStarted <-chan struct{}, exitCh <-chan error) <-chan error {
	if inst == nil {
		return readyOKCh
	}
	if ctx == nil {
		ctx = context.Background()
	}

	waiter, ok := inst.(proc.ReadyWaiter)
	if !ok {
		if task != nil {
			go func() {
				<-taskStarted
				task.Done()
			}()
		}
		return readyOKCh
	}

	ch := make(chan error, 1)
	go func() {
		readyCtx, cancel := context.WithCancel(ctx)
		defer cancel()

		readyErrCh := make(chan error, 1)
		go func() {
			readyErrCh <- waiter.WaitReady(readyCtx)
		}()

		var err error
		select {
		case err = <-readyErrCh:
		case err = <-exitCh:
			if err == nil {
				err = fmt.Errorf("%s exited before ready", inst.Info().Name())
			}
		case <-ctx.Done():
			err = ctx.Err()
		}
		// Prefer cancellation semantics during shutdown/interrupt.
		if ctx.Err() != nil && errors.Cause(ctx.Err()) == context.Canceled {
			err = ctx.Err()
		}

		// Stop the readiness probe once we decide the result (e.g. if the process
		// exited early).
		cancel()

		ch <- err
		close(ch)

		if task == nil {
			return
		}
		if err == nil {
			go func() {
				<-taskStarted
				task.Done()
			}()
			return
		}
		if errors.Cause(err) == context.Canceled {
			go func() {
				<-taskStarted
				task.Cancel("")
			}()
			return
		}
		p.markStartingTaskError(inst, "", err)
	}()
	return ch
}
