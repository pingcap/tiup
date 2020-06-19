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

package utils

import (
	"bufio"
	"fmt"
	"os"
	"time"

	"github.com/pingcap/errors"
)

// RetryOption is options for Retry()
type RetryOption struct {
	Attempts int64
	Delay    time.Duration
	Timeout  time.Duration
}

// default values for RetryOption
var (
	defaultAttempts int64 = 20
	defaultDelay          = time.Millisecond * 500 // 500ms
	defaultTimeout        = time.Second * 10       // 10s
)

// Retry retries the func until it returns no error or reaches attempts limit or
// timed out, either one is earlier
func Retry(doFunc func() error, opts ...RetryOption) error {
	var cfg RetryOption
	if len(opts) > 0 {
		cfg = opts[0]
	} else {
		cfg = RetryOption{
			Attempts: defaultAttempts,
			Delay:    defaultDelay,
			Timeout:  defaultTimeout,
		}
	}

	// timeout must be greater than 0
	if cfg.Timeout <= 0 {
		return fmt.Errorf("timeout (%s) must be greater than 0", cfg.Timeout)
	}
	// set options automatically for invalid value
	if cfg.Delay <= 0 {
		cfg.Delay = defaultDelay
	}
	if cfg.Attempts <= 0 {
		cfg.Attempts = cfg.Timeout.Milliseconds()/cfg.Delay.Milliseconds() + 1
	}

	timeoutChan := time.After(cfg.Timeout)

	// call the function
	var attemptCount int64
	for attemptCount = 0; attemptCount < cfg.Attempts; attemptCount++ {
		if err := doFunc(); err == nil {
			return nil
		}

		// check for timeout
		select {
		case <-timeoutChan:
			return fmt.Errorf("operation timed out after %s", cfg.Timeout)
		default:
			time.Sleep(cfg.Delay)
		}
	}

	return fmt.Errorf("operation exceeds the max retry attempts of %d", cfg.Attempts)
}

// TailN try get the latest n line of the file.
func TailN(fname string, n int) (lines []string, err error) {
	file, err := os.Open(fname)
	if err != nil {
		return nil, errors.AddStack(err)
	}
	defer file.Close()

	estimateLineSize := 1024

	stat, err := os.Stat(fname)
	if err != nil {
		return nil, errors.AddStack(err)
	}

	start := int(stat.Size()) - n*estimateLineSize
	if start < 0 {
		start = 0
	}

	_, err = file.Seek(int64(start), 0 /*means relative to the origin of the file*/)
	if err != nil {
		return nil, errors.AddStack(err)
	}

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		lines = append(lines, scanner.Text())
	}

	if len(lines) > n {
		lines = lines[len(lines)-n:]
	}

	return
}
