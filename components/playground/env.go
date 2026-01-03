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
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"github.com/pingcap/errors"
)

func resolvePlaygroundTarget(explicitTag, tiupDataDir, dataDir string) (playgroundTarget, error) {
	// If the caller provides an explicit target (tag or TIUP_INSTANCE_DATA_DIR),
	// do not guess.
	if explicitTag != "" || tiupDataDir != "" {
		port, err := loadPort(dataDir)
		if err != nil {
			tag := explicitTag
			if tag == "" {
				tag = filepath.Base(dataDir)
			}
			return playgroundTarget{}, playgroundNotRunningError{err: errors.Annotatef(err, "no playground running for tag %q", tag)}
		}
		tag := explicitTag
		if tag == "" {
			tag = filepath.Base(dataDir)
		}
		return playgroundTarget{tag: tag, dir: dataDir, port: port}, nil
	}

	baseDir := dataDir
	if baseDir == "" {
		return playgroundTarget{}, playgroundNotRunningError{err: errors.Errorf("no playground running")}
	}

	targets, err := listPlaygroundTargets(baseDir)
	if err != nil {
		return playgroundTarget{}, errors.AddStack(err)
	}
	if len(targets) == 0 {
		return playgroundTarget{}, playgroundNotRunningError{err: errors.Errorf("no playground running")}
	}
	if len(targets) == 1 {
		// Single running playground: implicit selection is unambiguous.
		return targets[0], nil
	}

	var items []string
	for _, t := range targets {
		items = append(items, fmt.Sprintf("%s(%d)", t.tag, t.port))
	}
	slices.Sort(items)
	return playgroundTarget{}, errors.Errorf("multiple playgrounds found: %s; please specify --tag", strings.Join(items, ", "))
}

type playgroundTarget struct {
	tag  string
	dir  string
	port int
}

func listPlaygroundTargets(baseDir string) ([]playgroundTarget, error) {
	entries, err := os.ReadDir(baseDir)
	if err != nil {
		return nil, errors.AddStack(err)
	}

	var out []playgroundTarget
	for _, ent := range entries {
		if !ent.IsDir() {
			continue
		}
		dir := filepath.Join(baseDir, ent.Name())
		port, err := loadPort(dir)
		if err != nil || port <= 0 {
			continue
		}
		out = append(out, playgroundTarget{tag: ent.Name(), dir: dir, port: port})
	}

	slices.SortStableFunc(out, func(a, b playgroundTarget) int {
		return strings.Compare(a.tag, b.tag)
	})
	return out, nil
}
