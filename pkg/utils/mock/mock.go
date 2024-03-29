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

package mock

import (
	"path"
	"reflect"
	"sync"

	"github.com/pingcap/failpoint"
)

// Finalizer represent the function that clean a mock point
type Finalizer func()

type mockPoints struct {
	m map[string]any
	l sync.Mutex
}

func (p *mockPoints) set(fpname string, value any) {
	p.l.Lock()
	defer p.l.Unlock()

	p.m[fpname] = value
}

func (p *mockPoints) get(fpname string) any {
	p.l.Lock()
	defer p.l.Unlock()

	return p.m[fpname]
}

func (p *mockPoints) clr(fpname string) {
	p.l.Lock()
	defer p.l.Unlock()

	delete(p.m, fpname)
}

var points = mockPoints{m: make(map[string]any)}

// On inject a failpoint
func On(fpname string) any {
	var ret any
	failpoint.Inject(fpname, func() {
		ret = points.get(fpname)
	})
	return ret
}

// With enable failpoint and provide a value
func With(fpname string, value any) Finalizer {
	if err := failpoint.Enable(failpath(fpname), "return(true)"); err != nil {
		panic(err)
	}
	points.set(fpname, value)
	return func() {
		if err := Reset(fpname); err != nil {
			panic(err)
		}
	}
}

// Reset disable failpoint and remove mock value
func Reset(fpname string) error {
	if err := failpoint.Disable(failpath(fpname)); err != nil {
		return err
	}
	points.clr(fpname)
	return nil
}

func failpath(fpname string) string {
	type em struct{}
	return path.Join(reflect.TypeOf(em{}).PkgPath(), fpname)
}
