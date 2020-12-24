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
	"io/ioutil"
	"os"

	"github.com/fatih/color"
	perrs "github.com/pingcap/errors"
	"github.com/pingcap/tiup/pkg/cliutil"
	"github.com/pingcap/tiup/pkg/cluster/spec"
	"github.com/pingcap/tiup/pkg/logger/log"
	"github.com/pingcap/tiup/pkg/meta"
	"github.com/pingcap/tiup/pkg/utils"
	"gopkg.in/yaml.v2"
)

// EditConfig lets the user edit the cluster's config.
func (m *Manager) EditConfig(name string, skipConfirm bool) error {
	metadata, err := m.meta(name)
	if err != nil && !errors.Is(perrs.Cause(err), meta.ErrValidate) {
		return err
	}

	topo := metadata.GetTopology()

	data, err := yaml.Marshal(topo)
	if err != nil {
		return perrs.AddStack(err)
	}

	newTopo, err := m.editTopo(topo, data, skipConfirm)
	if err != nil {
		return err
	}

	if newTopo == nil {
		return nil
	}

	log.Infof("Applying changes...")
	metadata.SetTopology(newTopo)
	err = m.specManager.SaveMeta(name, metadata)
	if err != nil {
		return perrs.Annotate(err, "failed to save meta")
	}

	log.Infof("Applied successfully, please use `%s reload %s [-N <nodes>] [-R <roles>]` to reload config.", cliutil.OsArgs0(), name)
	return nil
}

// 1. Write Topology to a temporary file.
// 2. Open file in editor.
// 3. Check and update Topology.
// 4. Save meta file.
func (m *Manager) editTopo(origTopo spec.Topology, data []byte, skipConfirm bool) (spec.Topology, error) {
	file, err := ioutil.TempFile(os.TempDir(), "*")
	if err != nil {
		return nil, perrs.AddStack(err)
	}

	name := file.Name()

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

	// Now user finish editing the file.
	newData, err := ioutil.ReadFile(name)
	if err != nil {
		return nil, perrs.AddStack(err)
	}

	newTopo := m.specManager.NewMetadata().GetTopology()
	err = yaml.UnmarshalStrict(newData, newTopo)
	if err != nil {
		fmt.Print(color.RedString("New topology could not be saved: "))
		log.Infof("Failed to parse topology file: %v", err)
		if !cliutil.PromptForConfirmNo("Do you want to continue editing? [Y/n]: ") {
			return m.editTopo(origTopo, newData, skipConfirm)
		}
		log.Infof("Nothing changed.")
		return nil, nil
	}

	// report error if immutable field has been changed
	if err := utils.ValidateSpecDiff(origTopo, newTopo); err != nil {
		fmt.Print(color.RedString("New topology could not be saved: "))
		log.Errorf("%s", err)
		if !cliutil.PromptForConfirmNo("Do you want to continue editing? [Y/n]: ") {
			return m.editTopo(origTopo, newData, skipConfirm)
		}
		log.Infof("Nothing changed.")
		return nil, nil
	}

	origData, err := yaml.Marshal(origTopo)
	if err != nil {
		return nil, perrs.AddStack(err)
	}

	if bytes.Equal(origData, newData) {
		log.Infof("The file has nothing changed")
		return nil, nil
	}

	utils.ShowDiff(string(origData), string(newData), os.Stdout)

	if !skipConfirm {
		if err := cliutil.PromptForConfirmOrAbortError(
			color.HiYellowString("Please check change highlight above, do you want to apply the change? [y/N]:"),
		); err != nil {
			return nil, err
		}
	}

	return newTopo, nil
}
