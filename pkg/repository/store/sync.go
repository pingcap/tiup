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
	"os"
	"os/exec"
	"path"
	"time"

	"github.com/pingcap/tiup/pkg/utils"
)

// Syncer sync diff files to target
type Syncer interface {
	Sync(srcDir string) error
}

type combinedSyncer struct {
	syncers []Syncer
}

func (s *combinedSyncer) Sync(srcDir string) error {
	for _, sy := range s.syncers {
		if err := sy.Sync(srcDir); err != nil {
			return err
		}
	}
	return nil
}

func combine(syncers ...Syncer) Syncer {
	return &combinedSyncer{syncers}
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

	if err := utils.MkdirAll(dstDir, 0755); err != nil {
		return err
	}

	files, err := os.ReadDir(srcDir)
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

type externalSyncer struct {
	script string
}

func newExternalSyncer(scriptPath string) Syncer {
	return &externalSyncer{scriptPath}
}

func (s *externalSyncer) Sync(srcDir string) error {
	cmd := exec.Command(s.script, srcDir)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}
