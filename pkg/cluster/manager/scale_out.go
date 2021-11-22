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
	"fmt"
	"time"

	"github.com/fatih/color"
	"github.com/joomcode/errorx"
	perrs "github.com/pingcap/errors"
	"github.com/pingcap/tiup/pkg/cluster/api"
	"github.com/pingcap/tiup/pkg/cluster/clusterutil"
	"github.com/pingcap/tiup/pkg/cluster/ctxt"
	"github.com/pingcap/tiup/pkg/cluster/executor"
	operator "github.com/pingcap/tiup/pkg/cluster/operation"
	"github.com/pingcap/tiup/pkg/cluster/spec"
	"github.com/pingcap/tiup/pkg/cluster/task"
	"github.com/pingcap/tiup/pkg/logger/log"
	"github.com/pingcap/tiup/pkg/set"
	"github.com/pingcap/tiup/pkg/tui"
	"github.com/pingcap/tiup/pkg/utils"
	"golang.org/x/mod/semver"
	"gopkg.in/yaml.v3"
)

// ScaleOut scale out the cluster.
func (m *Manager) ScaleOut(
	name string,
	topoFile string,
	afterDeploy func(b *task.Builder, newPart spec.Topology, gOpt operator.Options),
	final func(b *task.Builder, name string, meta spec.Metadata, gOpt operator.Options),
	opt DeployOptions,
	skipConfirm bool,
	gOpt operator.Options,
) error {
	if err := clusterutil.ValidateClusterNameOrError(name); err != nil {
		return err
	}

	// check for the input topology to let user confirm if there're any
	// global configs set
	if err := checkForGlobalConfigs(topoFile); err != nil {
		return err
	}

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

	if clusterSpec, ok := topo.(*spec.Specification); ok {
		if clusterSpec.GlobalOptions.TLSEnabled &&
			semver.Compare(base.Version, "v4.0.5") < 0 &&
			len(clusterSpec.TiFlashServers) > 0 {
			return fmt.Errorf("TiFlash %s is not supported in TLS enabled cluster", base.Version)
		}
	}

	if newPartTopo, ok := newPart.(*spec.Specification); ok {
		newPartTopo.AdjustByVersion(base.Version)
	}

	if err := validateNewTopo(newPart); err != nil {
		return err
	}

	var (
		sshConnProps  *tui.SSHConnectionProps = &tui.SSHConnectionProps{}
		sshProxyProps *tui.SSHConnectionProps = &tui.SSHConnectionProps{}
	)
	if gOpt.SSHType != executor.SSHTypeNone {
		var err error
		if sshConnProps, err = tui.ReadIdentityFileOrPassword(opt.IdentityFile, opt.UsePassword); err != nil {
			return err
		}
		if len(gOpt.SSHProxyHost) != 0 {
			if sshProxyProps, err = tui.ReadIdentityFileOrPassword(gOpt.SSHProxyIdentity, gOpt.SSHProxyUsePassword); err != nil {
				return err
			}
		}
	}

	if err := m.fillHostArch(sshConnProps, sshProxyProps, newPart, &gOpt, opt.User); err != nil {
		return err
	}

	// Abort scale out operation if the merged topology is invalid
	mergedTopo := topo.MergeTopo(newPart)
	if err := mergedTopo.Validate(); err != nil {
		return err
	}
	spec.ExpandRelativeDir(mergedTopo)

	if topo, ok := mergedTopo.(*spec.Specification); ok {
		// Check if TiKV's label set correctly
		if !opt.NoLabels {
			pdList := topo.BaseTopo().MasterList
			tlsCfg, err := topo.TLSConfig(m.specManager.Path(name, spec.TLSCertKeyDir))
			if err != nil {
				return err
			}
			pdClient := api.NewPDClient(pdList, 10*time.Second, tlsCfg)
			lbs, placementRule, err := pdClient.GetLocationLabels()
			if err != nil {
				return err
			}
			if !placementRule {
				if err := spec.CheckTiKVLabels(lbs, mergedTopo.(*spec.Specification)); err != nil {
					return perrs.Errorf("check TiKV label failed, please fix that before continue:\n%s", err)
				}
			}
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
			instance.SetPatched(true)
		}
	})

	if !skipConfirm {
		// patchedComponents are components that have been patched and overwrited
		if err := m.confirmTopology(name, base.Version, newPart, patchedComponents); err != nil {
			return err
		}
	}

	if opt.NoStart {
		cyan := color.New(color.FgHiRed, color.Bold)
		msg := cyan.Sprintf(`You use the parameter --no-start ! 
The new instance will not start, need to manually execute 'tiup-cluster start %s' .`, name)
		log.Warnf(msg)

		if err := tui.PromptForConfirmOrAbortError("Do you want to continue? [y/N]: "); err != nil {
			return err
		}
	}

	// Build the scale out tasks
	t, err := buildScaleOutTask(
		m, name, metadata, mergedTopo, opt, sshConnProps, sshProxyProps, newPart,
		patchedComponents, gOpt, afterDeploy, final)
	if err != nil {
		return err
	}

	ctx := ctxt.New(context.Background(), gOpt.Concurrency)
	ctx = context.WithValue(ctx, ctxt.CtxBaseTopo, topo)
	if err := t.Execute(ctx); err != nil {
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

// checkForGlobalConfigs checks the input scale out topology to make sure users are aware
// of the global config fields in it will be ignored.
func checkForGlobalConfigs(topoFile string) error {
	yamlFile, err := spec.ReadYamlFile(topoFile)
	if err != nil {
		return err
	}

	var newPart map[string]interface{}
	if err := yaml.Unmarshal(yamlFile, &newPart); err != nil {
		return err
	}

	for k := range newPart {
		switch k {
		case "global",
			"monitored",
			"server_configs":
			log.Warnf(color.YellowString(
				`You have one or more of ["global", "monitored", "server_configs"] fields configured in
the scale out topology, but they will be ignored during the scaling out process.
If you want to use configs different from the existing cluster, cancel now and
set them in the specification fileds for each host.`))
			if err := tui.PromptForConfirmOrAbortError("Do you want to continue? [y/N]: "); err != nil {
				return err
			}
			return nil // user confirmed, skip further checks
		}
	}
	return nil
}
