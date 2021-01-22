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
	"context"
	"errors"
	"time"

	"github.com/joomcode/errorx"
	perrs "github.com/pingcap/errors"
	"github.com/pingcap/tiup/pkg/cliutil"
	"github.com/pingcap/tiup/pkg/cluster/api"
	"github.com/pingcap/tiup/pkg/cluster/ctxt"
	"github.com/pingcap/tiup/pkg/cluster/executor"
	operator "github.com/pingcap/tiup/pkg/cluster/operation"
	"github.com/pingcap/tiup/pkg/cluster/spec"
	"github.com/pingcap/tiup/pkg/cluster/task"
	"github.com/pingcap/tiup/pkg/logger/log"
	"github.com/pingcap/tiup/pkg/set"
	"github.com/pingcap/tiup/pkg/utils"
)

// ScaleOutOptions contains the options for scale out.
type ScaleOutOptions struct {
	User           string // username to login to the SSH server
	SkipCreateUser bool   // don't create user
	IdentityFile   string // path to the private key file
	UsePassword    bool   // use password instead of identity file for ssh connection
	NoLabels       bool   // don't check labels for TiKV instance
}

// ScaleOut scale out the cluster.
func (m *Manager) ScaleOut(
	name string,
	topoFile string,
	afterDeploy func(b *task.Builder, newPart spec.Topology),
	final func(b *task.Builder, name string, meta spec.Metadata),
	opt ScaleOutOptions,
	skipConfirm bool,
	gOpt operator.Options,
) error {
	metadata, err := m.meta(name)
	// allow specific validation errors so that user can recover a broken
	// cluster if it is somehow in a bad state.
	if err != nil &&
		!errors.Is(perrs.Cause(err), spec.ErrNoTiSparkMaster) {
		return err
	}

	topo := metadata.GetTopology()
	base := metadata.GetBaseMeta()
	// Inherit existing global configuration. We must assign the inherited values before unmarshalling
	// because some default value rely on the global options and monitored options.
	newPart := topo.NewPart()

	// The no tispark master error is ignored, as if the tispark master is removed from the topology
	// file for some reason (manual edit, for example), it is still possible to scale-out it to make
	// the whole topology back to normal state.
	if err := spec.ParseTopologyYaml(topoFile, newPart); err != nil &&
		!errors.Is(perrs.Cause(err), spec.ErrNoTiSparkMaster) {
		return err
	}

	if err := validateNewTopo(newPart); err != nil {
		return err
	}

	// Abort scale out operation if the merged topology is invalid
	mergedTopo := topo.MergeTopo(newPart)
	if err := mergedTopo.Validate(); err != nil {
		return err
	}
	spec.ExpandRelativeDir(mergedTopo)

	if topo, ok := topo.(*spec.Specification); ok && !opt.NoLabels {
		// Check if TiKV's label set correctly
		pdList := topo.BaseTopo().MasterList
		tlsCfg, err := topo.TLSConfig(m.specManager.Path(name, spec.TLSCertKeyDir))
		if err != nil {
			return err
		}
		pdClient := api.NewPDClient(pdList, 10*time.Second, tlsCfg)
		lbs, err := pdClient.GetLocationLabels()
		if err != nil {
			return err
		}
		if err := spec.CheckTiKVLabels(lbs, mergedTopo.(*spec.Specification)); err != nil {
			return perrs.Errorf("check TiKV label failed, please fix that before continue:\n%s", err)
		}
	}

	clusterList, err := m.specManager.GetAllClusters()
	if err != nil {
		return err
	}
	if err := spec.CheckClusterPortConflict(clusterList, name, mergedTopo); err != nil {
		return err
	}
	if err := spec.CheckClusterDirConflict(clusterList, name, mergedTopo); err != nil {
		return err
	}

	patchedComponents := set.NewStringSet()
	newPart.IterInstance(func(instance spec.Instance) {
		if utils.IsExist(m.specManager.Path(name, spec.PatchDirName, instance.ComponentName()+".tar.gz")) {
			patchedComponents.Insert(instance.ComponentName())
		}
	})

	if !skipConfirm {
		// patchedComponents are components that have been patched and overwrited
		if err := m.confirmTopology(name, base.Version, newPart, patchedComponents); err != nil {
			return err
		}
	}

	var sshConnProps *cliutil.SSHConnectionProps = &cliutil.SSHConnectionProps{}
	if gOpt.SSHType != executor.SSHTypeNone {
		var err error
		if sshConnProps, err = cliutil.ReadIdentityFileOrPassword(opt.IdentityFile, opt.UsePassword); err != nil {
			return err
		}
	}

	// Build the scale out tasks
	t, err := buildScaleOutTask(
		m, name, metadata, mergedTopo, opt, sshConnProps, newPart,
		patchedComponents, gOpt, afterDeploy, final)
	if err != nil {
		return err
	}

	if err := t.Execute(ctxt.New(context.Background())); err != nil {
		if errorx.Cast(err) != nil {
			// FIXME: Map possible task errors and give suggestions.
			return err
		}
		return perrs.Trace(err)
	}

	log.Infof("Scaled cluster `%s` out successfully", name)

	return nil
}

// validateNewTopo checks the new part of scale-out topology to make sure it's supported
func validateNewTopo(topo spec.Topology) (err error) {
	topo.IterInstance(func(instance spec.Instance) {
		// check for "imported" parameter, it can not be true when scaling out
		if instance.IsImported() {
			err = errors.New(
				"'imported' is set to 'true' for new instance, this is only used " +
					"for instances imported from tidb-ansible and make no sense when " +
					"scaling out, please delete the line or set it to 'false' for new instances")
			return
		}
	})
	return err
}
