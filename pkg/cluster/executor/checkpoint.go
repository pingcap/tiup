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

package executor

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strings"
	"time"

	"github.com/pingcap/errors"
	"github.com/pingcap/tiup/pkg/queue"
	"go.uber.org/zap"
)

var (
	checkpoint *CheckPoint
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

// CheckPoint provides the ability to recover from a failed command at the failpoint
type CheckPoint struct {
	points *queue.AnyQueue
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

// NewCheckPoint returns a CheckPoint by given audit file
func NewCheckPoint(r io.Reader) (*CheckPoint, error) {
	cp := CheckPoint{
		points: queue.NewAnyQueue(func(a, b interface{}) bool {
			ma := a.(map[string]interface{})
			mb := b.(map[string]interface{})

			for _, field := range []string{"host", "port", "user", "cmd", "sudo", "src", "dst", "download"} {
				if fmt.Sprintf("%v", ma[field]) != fmt.Sprintf("%v", mb[field]) {
					return false
				}
			}
			return true
		}),
	}

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

// Check check if point exits in checkpoints, if exists, return previews state, otherwise, nil
func (c *CheckPoint) Check(point map[string]interface{}) map[string]interface{} {
	if p := c.points.Get(point); p != nil {
		return p.(map[string]interface{})
	}
	return nil
}

// CheckPointExecutor wraps Executor and inject checkpoints
//   ATTENTION please: the result of CheckPointExecutor shouldn't be used to impact
//                     external system like PD, otherwise, the external system may
//                     take wrong action.
type CheckPointExecutor struct {
	Executor
	config *SSHConfig
}

// Execute implements Executor interface.
func (c *CheckPointExecutor) Execute(cmd string, sudo bool, timeout ...time.Duration) (stdout []byte, stderr []byte, err error) {
	var point map[string]interface{}
	if checkpoint != nil {
		point = checkpoint.Check(map[string]interface{}{
			"host": c.config.Host,
			"port": c.config.Port,
			"user": c.config.User,
			"sudo": sudo,
			"cmd":  cmd,
		})
	}

	// Write checkpoint
	defer func() {
		if err != nil {
			return
		}

		zap.L().Info("CheckPoint",
			zap.String("host", c.config.Host),
			zap.Int("port", c.config.Port),
			zap.String("user", c.config.User),
			zap.Bool("sudo", sudo),
			zap.String("cmd", cmd),
			zap.String("stdout", string(stdout)),
			zap.String("stderr", string(stderr)),
			zap.Bool("hit", point != nil))
	}()

	if point != nil {
		return []byte(point["stdout"].(string)), []byte(point["stderr"].(string)), nil
	}

	return c.Executor.Execute(cmd, sudo, timeout...)
}

// Transfer implements Executer interface.
func (c *CheckPointExecutor) Transfer(src string, dst string, download bool) (err error) {
	var point map[string]interface{}
	if checkpoint != nil {
		point = checkpoint.Check(map[string]interface{}{
			"host":     c.config.Host,
			"port":     c.config.Port,
			"user":     c.config.User,
			"src":      src,
			"dst":      dst,
			"download": download,
		})
	}

	// Write checkpoint
	defer func() {
		if err != nil {
			return
		}

		zap.L().Info("CheckPoint",
			zap.String("host", c.config.Host),
			zap.Int("port", c.config.Port),
			zap.String("user", c.config.User),
			zap.String("src", src),
			zap.String("dst", dst),
			zap.Bool("download", download),
			zap.Bool("hit", point != nil))
	}()

	if point != nil {
		return nil
	}

	return c.Executor.Transfer(src, dst, download)
}
