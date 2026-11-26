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
	"bytes"
	"errors"
	"fmt"
	"io"
	"os"

	"github.com/fatih/color"
	perrs "github.com/pingcap/errors"
	"github.com/pingcap/tiup/pkg/cluster/clusterutil"
	"github.com/pingcap/tiup/pkg/cluster/spec"
	"github.com/pingcap/tiup/pkg/meta"
	"github.com/pingcap/tiup/pkg/tui"
	"github.com/pingcap/tiup/pkg/utils"
	"gopkg.in/yaml.v3"
)

// EditConfigOptions contains the options for config edition.
type EditConfigOptions struct {
	NewTopoFile string // path to new topology file to substitute the original one
}

// EditConfig lets the user edit the cluster's config.
func (m *Manager) EditConfig(name string, opt EditConfigOptions, skipConfirm bool) error {
	if err := clusterutil.ValidateClusterNameOrError(name); err != nil {
		return err
	}

	metadata, err := m.meta(name)
	if err != nil && !errors.Is(perrs.Cause(err), meta.ErrValidate) {
		return err
	}

	topo := metadata.GetTopology()

	data, err := yaml.Marshal(topo)
	if err != nil {
		return perrs.AddStack(err)
	}

	newTopo, err := m.editTopo(topo, data, opt, skipConfirm)
	if err != nil {
		return err
	}

	if newTopo == nil {
		return nil
	}

	m.logger.Infof("Applying changes...")
	metadata.SetTopology(newTopo)
	err = m.specManager.SaveMeta(name, metadata)
	if err != nil {
		return perrs.Annotate(err, "failed to save meta")
	}

	m.logger.Infof("Applied successfully, please use `%s reload %s [-N <nodes>] [-R <roles>]` to reload config.", tui.OsArgs0(), name)
	return nil
}

// If the flag --topology-file is specified, the first 2 steps will be skipped.
// 1. Write Topology to a temporary file.
// 2. Open file in editor.
// 3. Check and update Topology.
// 4. Save meta file.
func (m *Manager) editTopo(origTopo spec.Topology, data []byte, opt EditConfigOptions, skipConfirm bool) (spec.Topology, error) {
	var name string
	if opt.NewTopoFile == "" {
		file, err := os.CreateTemp(os.TempDir(), "*")
		if err != nil {
			return nil, perrs.AddStack(err)
		}

		name = file.Name()

		_, err = io.Copy(file, bytes.NewReader(data))
		if err != nil {
			return nil, perrs.AddStack(err)
		}

		err = file.Close()
		if err != nil {
			return nil, perrs.AddStack(err)
		}

		err = utils.OpenFileInEditor(name)
		if err != nil {
			return nil, err
		}
	} else {
		name = opt.NewTopoFile
	}

	// Now user finish editing the file or user has provided the new topology file
	newData, err := os.ReadFile(name)
	if err != nil {
		return nil, perrs.AddStack(err)
	}

	newTopo := m.specManager.NewMetadata().GetTopology()
	decoder := yaml.NewDecoder(bytes.NewReader(newData))
	decoder.KnownFields(true)
	err = decoder.Decode(newTopo)
	if err != nil {
		fmt.Print(color.RedString("New topology could not be saved: "))
		m.logger.Infof("Failed to parse topology file: %v", err)
		if opt.NewTopoFile == "" {
			if pass, _ := tui.PromptForConfirmNo("Do you want to continue editing? [Y/n]: "); !pass {
				return m.editTopo(origTopo, newData, opt, skipConfirm)
			}
		}
		m.logger.Infof("Nothing changed.")
		return nil, nil
	}

	// report error if immutable field has been changed
	if err := utils.ValidateSpecDiff(origTopo, newTopo); err != nil {
		fmt.Print(color.RedString("New topology could not be saved: "))
		m.logger.Errorf("%s", err)
		if opt.NewTopoFile == "" {
			if pass, _ := tui.PromptForConfirmNo("Do you want to continue editing? [Y/n]: "); !pass {
				return m.editTopo(origTopo, newData, opt, skipConfirm)
			}
		}
		m.logger.Infof("Nothing changed.")
		return nil, nil
	}

	origData, err := yaml.Marshal(origTopo)
	if err != nil {
		return nil, perrs.AddStack(err)
	}

	if bytes.Equal(origData, newData) {
		m.logger.Infof("The file has nothing changed")
		return nil, nil
	}

	utils.ShowDiff(string(origData), string(newData), os.Stdout)

	if !skipConfirm {
		if err := tui.PromptForConfirmOrAbortError(
			"%s", color.HiYellowString("Please check change highlight above, do you want to apply the change? [y/N]:"),
		); err != nil {
			return nil, err
		}
	}

	return newTopo, nil
}
