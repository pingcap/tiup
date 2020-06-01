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

package store

import (
	"fmt"
	"io/ioutil"
	"os"
	"path"
	"time"

	"github.com/pingcap/tiup/pkg/utils"
)

// Syncer sync diff files to target
type Syncer interface {
	Sync(srcDir string) error
}

type fsSyncer struct {
	root string
}

func newFsSyncer(root string) Syncer {
	return &fsSyncer{root}
}

func (s *fsSyncer) Sync(srcDir string) error {
	unix := time.Now().UnixNano()
	dstDir := path.Join(s.root, fmt.Sprintf("commit-%d", unix))

	if err := os.MkdirAll(dstDir, 0755); err != nil {
		return err
	}

	files, err := ioutil.ReadDir(srcDir)
	if err != nil {
		return err
	}

	for _, f := range files {
		if err := utils.Copy(path.Join(srcDir, f.Name()), path.Join(dstDir, f.Name())); err != nil {
			return err
		}
	}

	return nil
}
