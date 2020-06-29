package spec

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

import (
	"fmt"
	"path"
	"strings"

	"github.com/pingcap/tiup/pkg/cluster/spec"

	"github.com/pingcap/errors"
	"github.com/pingcap/tiup/pkg/cluster/clusterutil"
	"github.com/pingcap/tiup/pkg/cluster/executor"
)

func checkConfig(e executor.Executor, componentName, clusterVersion, nodeOS, arch, config string, paths DirPaths) error {
	repo, err := clusterutil.NewRepository(nodeOS, arch)
	if err != nil {
		return err
	}
	ver := spec.ComponentVersion(componentName, clusterVersion)
	entry, err := repo.ComponentBinEntry(componentName, ver)
	if err != nil {
		return err
	}

	binPath := path.Join(paths.Deploy, "bin", entry)
	// Skip old versions
	if !hasConfigCheckFlag(e, binPath) {
		return nil
	}

	configPath := path.Join(paths.Deploy, "conf", config)
	_, _, err = e.Execute(fmt.Sprintf("%s --config-check --config=%s", binPath, configPath), false)
	return errors.Annotatef(err, "check config failed: %s", componentName)
}

func hasConfigCheckFlag(e executor.Executor, binPath string) bool {
	stdout, stderr, _ := e.Execute(fmt.Sprintf("%s --help", binPath), false)
	return strings.Contains(string(stdout), "config-check") || strings.Contains(string(stderr), "config-check")
}
