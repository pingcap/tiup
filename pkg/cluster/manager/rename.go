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

package manager

import (
	"fmt"
	"os"

	"github.com/fatih/color"
	"github.com/pingcap/tiup/pkg/cliutil"
	"github.com/pingcap/tiup/pkg/cluster/clusterutil"
	operator "github.com/pingcap/tiup/pkg/cluster/operation"
	"github.com/pingcap/tiup/pkg/cluster/spec"
	"github.com/pingcap/tiup/pkg/logger/log"
	"github.com/pingcap/tiup/pkg/utils"
)

// Rename the cluster
func (m *Manager) Rename(name string, opt operator.Options, newName string, skipConfirm bool) error {
	if err := clusterutil.ValidateClusterNameOrError(name); err != nil {
		return err
	}
	if !utils.IsExist(m.specManager.Path(name)) {
		return errorRenameNameNotExist.
			New("Cluster name '%s' not exist", name).
			WithProperty(cliutil.SuggestionFromFormat("Please double check your cluster name"))
	}

	if err := clusterutil.ValidateClusterNameOrError(newName); err != nil {
		return err
	}
	if utils.IsExist(m.specManager.Path(newName)) {
		return errorRenameNameDuplicate.
			New("Cluster name '%s' is duplicated", newName).
			WithProperty(cliutil.SuggestionFromFormat("Please specify another cluster name"))
	}

	if !skipConfirm {
		if err := cliutil.PromptForConfirmOrAbortError(
			fmt.Sprintf("Will rename the cluster name from %s to %s.\nDo you confirm this action? [y/N]:", color.HiYellowString(name), color.HiYellowString(newName)),
		); err != nil {
			return err
		}
	}

	_, err := m.meta(name)
	if err != nil { // refuse renaming if current cluster topology is not valid
		return err
	}

	if err := os.Rename(m.specManager.Path(name), m.specManager.Path(newName)); err != nil {
		return err
	}

	log.Infof("Rename cluster `%s` -> `%s` successfully", name, newName)

	opt.Roles = []string{spec.ComponentGrafana, spec.ComponentPrometheus}
	return m.Reload(newName, opt, false, skipConfirm)
}
