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
	logprinter "github.com/pingcap/tiup/pkg/logger/printer"
	"github.com/pingcap/tiup/pkg/set"
	"github.com/pingcap/tiup/pkg/tui"
	"github.com/pingcap/tiup/pkg/utils"
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

	// check the scale out file lock is exist
	err := checkScaleOutLock(m, name, opt, skipConfirm)
	if err != nil {
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

	// if stage2 is true, the new part data store in scale-out file lock
	if opt.Stage2 {
		// Acquire the Scale-out file lock
		newPart, err = m.specManager.ScaleOutLock(name)
		if err != nil {
			return err
		}
	} else { // if stage2 is true, not need check topology or other
		// check for the input topology to let user confirm if there're any
		// global configs set
		if err := checkForGlobalConfigs(m.logger, topoFile, skipConfirm); err != nil {
			return err
		}

		// The no tispark master error is ignored, as if the tispark master is removed from the topology
		// file for some reason (manual edit, for example), it is still possible to scale-out it to make
		// the whole topology back to normal state.
		if err := spec.ParseTopologyYaml(topoFile, newPart, true); err != nil &&
			!errors.Is(perrs.Cause(err), spec.ErrNoTiSparkMaster) {
			return err
		}

		if err := checkTiFlashWithTLS(topo, base.Version); err != nil {
			return err
		}

		if newPartTopo, ok := newPart.(*spec.Specification); ok {
			newPartTopo.AdjustByVersion(base.Version)
		}

		if err := validateNewTopo(newPart); err != nil {
			return err
		}
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

	if err := m.fillHost(sshConnProps, sshProxyProps, newPart, &gOpt, opt.User); err != nil {
		return err
	}

	var mergedTopo spec.Topology
	// in satge2, not need mergedTopo
	if opt.Stage2 {
		mergedTopo = topo
	} else {
		// Abort scale out operation if the merged topology is invalid
		mergedTopo = topo.MergeTopo(newPart)
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
				pdClient := api.NewPDClient(
					context.WithValue(context.TODO(), logprinter.ContextKeyLogger, m.logger),
					pdList, 10*time.Second, tlsCfg,
				)
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

		if err := checkConflict(m, name, mergedTopo); err != nil {
			return err
		}
	}

	patchedComponents := set.NewStringSet()
	// if stage2 is true, this check is not work
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

	// Build the scale out tasks
	t, err := buildScaleOutTask(
		m, name, metadata, mergedTopo, opt, sshConnProps, sshProxyProps, newPart,
		patchedComponents, gOpt, afterDeploy, final)
	if err != nil {
		return err
	}

	ctx := ctxt.New(
		context.Background(),
		gOpt.Concurrency,
		m.logger,
	)
	ctx = context.WithValue(ctx, ctxt.CtxBaseTopo, topo)
	if err := t.Execute(ctx); err != nil {
		if errorx.Cast(err) != nil {
			// FIXME: Map possible task errors and give suggestions.
			return err
		}
		return perrs.Trace(err)
	}

	if opt.Stage1 {
		m.logger.Infof(`The new instance is not started!
You need to execute '%s' to start the new instance.`, color.YellowString("tiup cluster scale-out %s --stage2", name))
	}

	m.logger.Infof("Scaled cluster `%s` out successfully", color.YellowString(name))

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
func checkForGlobalConfigs(logger *logprinter.Logger, topoFile string, skipConfirm bool) error {
	yamlFile, err := spec.ReadYamlFile(topoFile)
	if err != nil {
		return err
	}

	var newPart map[string]interface{}
	if err := yaml.Unmarshal(yamlFile, &newPart); err != nil {
		return err
	}

	// user confirmed, skip checks

	for k := range newPart {
		switch k {
		case "global",
			"monitored",
			"server_configs":
			logger.Warnf(`You have one or more of %s fields configured in
	the scale out topology, but they will be ignored during the scaling out process.
	If you want to use configs different from the existing cluster, cancel now and
	set them in the specification fileds for each host.`, color.YellowString(`["global", "monitored", "server_configs"]`))
			if !skipConfirm {
				if err := tui.PromptForConfirmOrAbortError("Do you want to continue? [y/N]: "); err != nil {
					return err
				}
			}
			return nil
		}
	}
	return nil
}

// checkEnvWithStage1 check environment in scale-out stage 1
func checkScaleOutLock(m *Manager, name string, opt DeployOptions, skipConfirm bool) error {
	locked, _ := m.specManager.IsScaleOutLocked(name)

	if (!opt.Stage1 && !opt.Stage2) && locked {
		return m.specManager.ScaleOutLockedErr(name)
	}

	if opt.Stage1 {
		if locked {
			return m.specManager.ScaleOutLockedErr(name)
		}

		m.logger.Warnf(`The parameter '%s' is set, new instance will not be started
	Please manually execute '%s' to finish the process.`,
			color.YellowString("--stage1"),
			color.YellowString("tiup cluster scale-out %s --stage2", name))
		if !skipConfirm {
			if err := tui.PromptForConfirmOrAbortError("Do you want to continue? [y/N]: "); err != nil {
				return err
			}
		}
	}

	if opt.Stage2 {
		if !locked {
			return fmt.Errorf("The scale-out file lock does not exist, please make sure to run 'tiup-cluster scale-out %s --stage1' first", name)
		}

		m.logger.Warnf(`The parameter '%s' is set, only start the new instances and reload configs.`, color.YellowString("--stage2"))
		if !skipConfirm {
			if err := tui.PromptForConfirmOrAbortError("Do you want to continue? [y/N]: "); err != nil {
				return err
			}
		}
	}

	return nil
}
