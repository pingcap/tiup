package main

import (
	"context"

	"github.com/pingcap/tiup/components/playground/proc"
	pgservice "github.com/pingcap/tiup/components/playground/service"
	"github.com/pingcap/tiup/pkg/tui/colorstr"
	"github.com/pingcap/tiup/pkg/utils"
)

func (p *Playground) startTiFlashAfterTiDB(ctx context.Context, preloader *binaryPreloader) error {
	if p == nil {
		return nil
	}

	var readyChs []<-chan error
	for _, flash := range pgservice.ProcsOf[*proc.TiFlashInstance](p, proc.ServiceTiFlash, proc.ServiceTiFlashWrite, proc.ServiceTiFlashCompute) {
		info := flash.Info()
		if info == nil {
			continue
		}

		if bin := info.UserBinPath; bin != "" {
			info.BinPath = bin
			info.Version = utils.Version("")
		} else {
			_, binPath, version, err := preloader.resolve(info.RepoComponentID.String())
			if err != nil {
				return err
			}
			info.BinPath = binPath
			info.Version = version
		}

		readyCh, err := p.startProc(ctx, flash)
		if err != nil {
			colorstr.Fprintf(p.termWriter(), "[red][bold]TiFlash %s failed to start:[reset] %v\n", flash.Addr(), err)
			p.removeProc(info.Service, flash)
			continue
		}
		if readyCh != nil {
			readyChs = append(readyChs, readyCh)
		}
	}

	for _, ch := range readyChs {
		if ch == nil {
			continue
		}
		_ = <-ch
	}

	return nil
}

func (p *Playground) skipTiFlashStartingTasks(reason string) {
	if p == nil || reason == "" {
		return
	}

	p.progressMu.Lock()
	startingTasks := p.startingTasks
	p.progressMu.Unlock()
	if startingTasks == nil {
		return
	}

	for _, flash := range pgservice.ProcsOf[*proc.TiFlashInstance](p, proc.ServiceTiFlash, proc.ServiceTiFlashWrite, proc.ServiceTiFlashCompute) {
		if flash == nil || flash.Info() == nil {
			continue
		}
		if t := startingTasks[flash.Info().Name()]; t != nil {
			t.Skip(reason)
		}
	}
}
