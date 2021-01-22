// Copyright 2021 PingCAP, Inc.
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

package checkpoint

import (
	"bufio"
	"context"
	"encoding/json"
	"io"
	"os"
	"strings"
	"sync/atomic"

	"github.com/pingcap/errors"
	"github.com/pingcap/tiup/pkg/queue"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"golang.org/x/sync/semaphore"
)

type contextKey string

const (
	semKey       = contextKey("CHECKPOINT_SEMAPHORE")
	goroutineKey = contextKey("CHECKPOINT_GOROUTINE")
)

var (
	checkpoint  *CheckPoint
	checkfields []FieldSet
)

// SetCheckPoint set global checkpoint for executor
func SetCheckPoint(file string) error {
	pointReader, err := os.Open(file)
	if err != nil {
		return errors.AddStack(err)
	}
	defer pointReader.Close()

	checkpoint, err = NewCheckPoint(pointReader)
	if err != nil {
		return err
	}

	return nil
}

// Acquire wraps CheckPoint.Acquire
func Acquire(ctx context.Context, point map[string]interface{}) *Point {
	// Check goroutine if we are in test
	gptr := ctx.Value(goroutineKey).(*goroutineLock)
	g := atomic.LoadUint64((*uint64)(gptr))
	if g == 0 {
		atomic.StoreUint64((*uint64)(gptr), uint64(newGoroutineLock()))
	} else {
		goroutineLock(g).check()
	}

	// If checkpoint is disabled, return a mock point
	if checkpoint == nil {
		return &Point{nil, nil, true}
	}

	return checkpoint.Acquire(ctx, point)
}

// NewContext wraps given context with value needed by checkpoint
func NewContext(ctx context.Context) context.Context {
	switch {
	case ctx.Value(semKey) == nil:
		ctx = context.WithValue(ctx, semKey, semaphore.NewWeighted(1))
	case ctx.Value(semKey).(*semaphore.Weighted).TryAcquire(1):
		defer ctx.Value(semKey).(*semaphore.Weighted).Release(1)
		ctx = context.WithValue(ctx, semKey, semaphore.NewWeighted(1))
	default:
		ctx = context.WithValue(ctx, semKey, semaphore.NewWeighted(0))
	}

	return context.WithValue(ctx, goroutineKey, new(goroutineLock))
}

// CheckPoint provides the ability to recover from a failed command at the failpoint
type CheckPoint struct {
	points *queue.AnyQueue
}

// NewCheckPoint returns a CheckPoint by given audit file
func NewCheckPoint(r io.Reader) (*CheckPoint, error) {
	cp := CheckPoint{points: queue.NewAnyQueue(comparePoint)}

	scanner := bufio.NewScanner(r)
	for scanner.Scan() {
		line := scanner.Text()
		m, err := checkLine(line)
		if err != nil {
			return nil, errors.Annotate(err, "initial checkpoint failed")
		}
		if m == nil {
			continue
		}

		cp.points.Put(m)
	}

	if err := scanner.Err(); err != nil {
		return nil, errors.Annotate(err, "failed to parse audit file %s")
	}

	return &cp, nil
}

// Acquire get point from checkpoints
func (c *CheckPoint) Acquire(ctx context.Context, point map[string]interface{}) *Point {
	if ctx.Value(semKey) == nil {
		panic("the value of the key " + semKey + " not found in context, use the ctx from checkpoint.NewContext please")
	}

	acquired := ctx.Value(semKey).(*semaphore.Weighted).TryAcquire(1)
	if p := c.points.Get(point); p != nil {
		return &Point{ctx, p.(map[string]interface{}), acquired}
	}
	return &Point{ctx, nil, acquired}
}

// Point is a point of checkpoint
type Point struct {
	ctx      context.Context
	point    map[string]interface{}
	acquired bool
}

// Hit returns value of the point, it will be nil if not hit.
func (p *Point) Hit() map[string]interface{} {
	return p.point
}

// Release write checkpoint into log file
func (p *Point) Release(err error, fields ...zapcore.Field) {
	logfn := zap.L().Info
	if err != nil {
		logfn = zap.L().Error
		fields = append(fields, zap.Error(err))
	}
	fields = append(fields, zap.Bool("hit", p.Hit() != nil))

	if p.acquired {
		logfn("CheckPoint", fields...)
		// If checkpoint is disabled, the p.ctx will be nil
		if p.ctx != nil {
			p.ctx.Value(semKey).(*semaphore.Weighted).Release(1)
		}
	}
}

func checkLine(line string) (map[string]interface{}, error) {
	// target log format:
	//	2021-01-13T14:11:02.987+0800    INFO    SCPCommand      {k:v...}
	//	2021-01-13T14:11:03.780+0800    INFO    SSHCommand      {k:v...}
	ss := strings.Fields(line)
	pos := strings.Index(line, "{")
	if len(ss) < 4 || ss[1] != "INFO" || ss[2] != "CheckPoint" || pos == -1 {
		return nil, nil
	}

	m := make(map[string]interface{})
	if err := json.Unmarshal([]byte(line[pos:]), &m); err != nil {
		return nil, errors.AddStack(err)
	}

	return m, nil
}

func comparePoint(a, b interface{}) bool {
	ma := a.(map[string]interface{})
	mb := b.(map[string]interface{})

next_set:
	for _, fs := range checkfields {
		for _, cf := range fs.Slice() {
			if cf.eq == nil {
				continue
			}
			if !cf.eq(ma[cf.field], mb[cf.field]) {
				continue next_set
			}
		}
		return true
	}
	return false
}
