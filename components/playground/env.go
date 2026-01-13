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

package main

import (
	"os"
	"path/filepath"

	"github.com/pingcap/errors"
)

// targetTag find the target playground we want to send the command.
// first try the tag of current instance, then find the first playground.
// so, if running multi playground, you must specify the tag to send the command to.
// note the flowing two instance will use two different tag.
// 1. tiup playground
// 2. tiup playground display
func targetTag() (port int, err error) {
	port, err = loadPort(dataDir)
	if err == nil {
		return port, nil
	}
	err = nil

	_ = filepath.Walk(filepath.Dir(dataDir), func(path string, info os.FileInfo, err error) error {
		if port != 0 {
			return filepath.SkipDir
		}

		// ignore error
		if err != nil {
			return nil
		}

		if !info.IsDir() {
			return nil
		}

		port, _ = loadPort(path)
		return nil
	})

	if port == 0 {
		return 0, errors.Errorf("no playground running")
	}

	return
}
