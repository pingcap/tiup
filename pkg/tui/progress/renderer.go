// Copyright 2020 PingCAP, Inc.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// See the License for the specific language governing permissions and
// limitations under the License.

package progress

import (
	"time"

	"go.uber.org/atomic"
)

type renderer struct {
	isUpdaterRunning atomic.Bool
	stopChan         chan struct{}
	stopFinishedChan chan struct{}
	renderFn         func()
}

func newRenderer() *renderer {
	return &renderer{
		isUpdaterRunning: atomic.Bool{},
		stopChan:         nil,
		stopFinishedChan: nil,
		renderFn:         nil,
	}
}

func (r *renderer) startRenderLoop() {
	if r.renderFn == nil {
		panic("renderFn must be set")
	}
	if !r.isUpdaterRunning.CompareAndSwap(false, true) {
		return
	}
	r.stopChan = make(chan struct{})
	r.stopFinishedChan = make(chan struct{})
	go r.renderLoopFn()
}

func (r *renderer) stopRenderLoop() {
	if !r.isUpdaterRunning.CompareAndSwap(true, false) {
		return
	}
	r.stopChan <- struct{}{}
	close(r.stopChan)
	r.stopChan = nil

	<-r.stopFinishedChan
	close(r.stopFinishedChan)
	r.stopFinishedChan = nil
}

func (r *renderer) renderLoopFn() {
	ticker := time.NewTicker(refreshRate)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			r.renderFn()
		case <-r.stopChan:
			r.renderFn()
			r.stopFinishedChan <- struct{}{}
			return
		}
	}
}
