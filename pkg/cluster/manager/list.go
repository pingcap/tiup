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
	"errors"

	perrs "github.com/pingcap/errors"
	"github.com/pingcap/tiup/pkg/cliutil"
	"github.com/pingcap/tiup/pkg/cluster/spec"
	"github.com/pingcap/tiup/pkg/meta"
)

// ListCluster list the clusters.
func (m *Manager) ListCluster() error {
	names, err := m.specManager.List()
	if err != nil {
		return err
	}

	clusterTable := [][]string{
		// Header
		{"Name", "User", "Version", "Path", "PrivateKey"},
	}

	for _, name := range names {
		metadata, err := m.meta(name)
		if err != nil && !errors.Is(perrs.Cause(err), meta.ErrValidate) &&
			!errors.Is(perrs.Cause(err), spec.ErrNoTiSparkMaster) {
			return perrs.Trace(err)
		}

		base := metadata.GetBaseMeta()

		clusterTable = append(clusterTable, []string{
			name,
			base.User,
			base.Version,
			m.specManager.Path(name),
			m.specManager.Path(name, "ssh", "id_rsa"),
		})
	}

	cliutil.PrintTable(clusterTable, true)
	return nil
}
