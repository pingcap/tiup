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
	"fmt"
	"reflect"
	"strconv"
	"strings"
	"time"
)

// JoinInt joins a slice of int to string
func JoinInt(nums []int, delim string) string {
	result := ""
	for _, i := range nums {
		result += strconv.Itoa(i)
		result += delim
	}
	return strings.TrimSuffix(result, delim)
}

// InSlice checks if a element is present in a slice, returns false if slice is
// empty, or any other error occurs
func InSlice(elem interface{}, slice interface{}) bool {
	s := reflect.ValueOf(slice)
	if s.Kind() != reflect.Slice {
		return false
	}

	for i := 0; i < s.Len(); i++ {
		if elem == s.Index(i).Interface() {
			return true
		}
	}
	return false
}

// RetryOption is options for Retry()
type RetryOption struct {
	Attempts int
	Delay    time.Duration
	Timeout  time.Duration
}

// default values for RetryOption
var (
	defaultAttempts = 10
	defaultDelay    = time.Millisecond * 500 // 500ms
	defaultTimeout  = time.Second * 10       // 10s
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

	timeoutChan := make(chan string, 1)

	// call the function
	var attemptCount int
	for attemptCount = 0; attemptCount < cfg.Attempts; attemptCount++ {
		go func() {
			err := doFunc()
			if err == nil {
				timeoutChan <- "done"
			}
		}()
		time.Sleep(cfg.Delay)
	}

	// check for attempts
	if attemptCount >= cfg.Attempts {
		return fmt.Errorf("operation exceeds the max retry attempts %d", cfg.Attempts)
	}

	// check for timeout
	select {
	case <-timeoutChan:
		return nil
	case <-time.After(cfg.Timeout):
		return fmt.Errorf("operation timed out after %s", cfg.Timeout)
	}
}
