package main

import (
	"context"
	"fmt"
	"strings"

	"slices"

	"github.com/pingcap/tiup/components/playground/proc"
	"github.com/pingcap/tiup/pkg/environment"
	"github.com/pingcap/tiup/pkg/utils"
)

type binaryPreloadKey struct {
	component  string
	constraint string
}

type binaryPreload struct {
	done    chan struct{}
	err     error
	version utils.Version
	binPath string

	// targets are the "Starting instances" tasks for instances that will use
	// this resolved version (i.e. no user-specified BinPath).
	targets []progressTask
}

type binaryPreloader struct {
	ctx       context.Context
	pg        *Playground
	bootVer   string
	forcePull bool

	items     map[binaryPreloadKey]*binaryPreload
	order     []binaryPreloadKey
	allDoneCh chan struct{}
}

func newBinaryPreloader(ctx context.Context, pg *Playground, bootVer string, forcePull bool) *binaryPreloader {
	if ctx == nil {
		ctx = context.Background()
	}
	return &binaryPreloader{
		ctx:       ctx,
		pg:        pg,
		bootVer:   bootVer,
		forcePull: forcePull,
		items:     make(map[binaryPreloadKey]*binaryPreload),
		allDoneCh: make(chan struct{}),
	}
}

func (p *binaryPreloader) constraintFor(serviceID proc.ServiceID) string {
	if p == nil {
		return utils.LatestVersionAlias
	}
	if p.pg == nil {
		return utils.LatestVersionAlias
	}
	constraint := p.pg.versionConstraintForService(serviceID, p.bootVer)
	if constraint == "" {
		constraint = utils.LatestVersionAlias
	}
	return constraint
}

func (p *binaryPreloader) collect(startingTasks map[string]progressTask) {
	if p == nil || p.pg == nil {
		return
	}

	_ = p.pg.WalkProcs(func(serviceID proc.ServiceID, inst proc.Process) error {
		if inst == nil {
			return nil
		}
		info := inst.Info()
		if info == nil || info.UserBinPath != "" {
			return nil
		}

		component := info.RepoComponentID.String()
		constraint := p.constraintFor(serviceID)
		key := binaryPreloadKey{component: component, constraint: constraint}

		item, ok := p.items[key]
		if !ok {
			item = &binaryPreload{done: make(chan struct{})}
			p.items[key] = item
		}
		if startingTasks != nil {
			if t := startingTasks[info.Name()]; t != nil {
				item.targets = append(item.targets, t)
			}
		}
		return nil
	})

	p.order = make([]binaryPreloadKey, 0, len(p.items))
	for key := range p.items {
		p.order = append(p.order, key)
	}
	slices.SortFunc(p.order, func(a, b binaryPreloadKey) int {
		if c := strings.Compare(a.component, b.component); c != 0 {
			return c
		}
		return strings.Compare(a.constraint, b.constraint)
	})
}

func (p *binaryPreloader) start() {
	if p == nil {
		return
	}
	if len(p.order) == 0 {
		close(p.allDoneCh)
		return
	}

	go func() {
		defer close(p.allDoneCh)

		for _, key := range p.order {
			item := p.items[key]
			if item == nil || item.done == nil {
				continue
			}
			func() {
				defer close(item.done)

				var (
					v   utils.Version
					err error
				)

				v, err = environment.GlobalEnv().V1Repository().ResolveComponentVersion(key.component, key.constraint)
				item.version = v
				if err == nil {
					item.binPath, err = prepareComponentBinary(key.component, v, p.forcePull)
				}
				item.err = err

				// Once we have resolved the version constraint, update the starting
				// tasks to show the concrete version (even for components that start
				// later, such as TiFlash).
				if err == nil && !v.IsEmpty() && len(item.targets) > 0 && p.pg != nil {
					p.pg.progressMu.Lock()
					active := p.pg.startingGroup != nil
					p.pg.progressMu.Unlock()
					if active {
						meta := v.String()
						for _, t := range item.targets {
							t.SetMeta(meta)
						}
					}
				}
			}()
		}
	}()
}

func (p *binaryPreloader) wait(component, constraint string) (*binaryPreload, error) {
	if p == nil {
		return nil, nil
	}

	key := binaryPreloadKey{component: component, constraint: constraint}
	item := p.items[key]
	if item == nil {
		return nil, fmt.Errorf("binary not resolved for %s", component)
	}

	select {
	case <-item.done:
		return item, item.err
	case <-p.ctx.Done():
		return item, p.ctx.Err()
	}
}

func (p *binaryPreloader) resolve(serviceID proc.ServiceID, component string) (constraint, binPath string, version utils.Version, err error) {
	if p == nil {
		return "", "", utils.Version(""), fmt.Errorf("binary preloader is nil")
	}

	constraint = p.constraintFor(serviceID)
	item, err := p.wait(component, constraint)
	if err != nil {
		return constraint, "", utils.Version(""), err
	}
	if item == nil || item.binPath == "" {
		return constraint, "", utils.Version(""), fmt.Errorf("binary not resolved for %s", component)
	}
	return constraint, item.binPath, item.version, nil
}

func (p *binaryPreloader) allDone() <-chan struct{} {
	if p == nil || p.allDoneCh == nil {
		ch := make(chan struct{})
		close(ch)
		return ch
	}
	return p.allDoneCh
}
