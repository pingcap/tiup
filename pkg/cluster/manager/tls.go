// Copyright 2021 PingCAP, Inc.
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
	"fmt"
	"os"

	"github.com/fatih/color"
	"github.com/joomcode/errorx"
	perrs "github.com/pingcap/errors"
	"github.com/pingcap/tiup/pkg/cluster/clusterutil"
	"github.com/pingcap/tiup/pkg/cluster/ctxt"
	"github.com/pingcap/tiup/pkg/cluster/executor"
	operator "github.com/pingcap/tiup/pkg/cluster/operation"
	"github.com/pingcap/tiup/pkg/cluster/spec"

	"github.com/pingcap/tiup/pkg/set"
	"github.com/pingcap/tiup/pkg/tui"
	"golang.org/x/mod/semver"
)

// TLS set cluster enable/disable encrypt communication by tls
func (m *Manager) TLS(name string, gOpt operator.Options, enable, cleanCertificate, reloadCertificate, skipConfirm bool) error {
	if err := clusterutil.ValidateClusterNameOrError(name); err != nil {
		return err
	}

	// check locked
	if err := m.specManager.ScaleOutLockedErr(name); err != nil {
		return err
	}

	metadata, err := m.meta(name)
	if err != nil {
		return err
	}

	topo := metadata.GetTopology()
	base := metadata.GetBaseMeta()

	// set tls_enabled
	globalOptions := topo.BaseTopo().GlobalOptions
	// if force is true, skip this check
	if globalOptions.TLSEnabled == enable && !gOpt.Force {
		if enable {
			m.logger.Infof("cluster `%s` TLS status is already enabled\n", name)
		} else {
			m.logger.Infof("cluster `%s` TLS status is already disabled\n", name)
		}
		return nil
	}
	globalOptions.TLSEnabled = enable

	if err := checkTLSEnv(topo, name, base.Version, skipConfirm); err != nil {
		return err
	}

	var (
		sshProxyProps *tui.SSHConnectionProps = &tui.SSHConnectionProps{}
	)
	if gOpt.SSHType != executor.SSHTypeNone {
		var err error
		if len(gOpt.SSHProxyHost) != 0 {
			if sshProxyProps, err = tui.ReadIdentityFileOrPassword(gOpt.SSHProxyIdentity, gOpt.SSHProxyUsePassword); err != nil {
				return err
			}
		}
	}

	// delFileMap: files that need to be cleaned up, if flag -- cleanCertificate are used
	delFileMap, err := getTLSFileMap(m, name, topo, enable, cleanCertificate, skipConfirm)
	if err != nil {
		return err
	}

	// Build the tls  tasks
	t, err := buildTLSTask(
		m, name, metadata, gOpt, reloadCertificate, sshProxyProps, delFileMap)
	if err != nil {
		return err
	}

	ctx := ctxt.New(
		context.Background(),
		gOpt.Concurrency,
		m.logger,
	)
	if err := t.Execute(ctx); err != nil {
		if errorx.Cast(err) != nil {
			// FIXME: Map possible task errors and give suggestions.
			return err
		}
		return perrs.Trace(err)
	}

	if err := m.specManager.SaveMeta(name, metadata); err != nil {
		return err
	}

	if !enable {
		// the cleanCertificate parameter will only take effect when enable is false
		if cleanCertificate {
			os.RemoveAll(m.specManager.Path(name, spec.TLSCertKeyDir))
		}
		m.logger.Infof("\tCleanup localhost tls file success")
	}

	if enable {
		m.logger.Infof("Enabled TLS between TiDB components for cluster `%s` successfully", name)
	} else {
		m.logger.Infof("Disabled TLS between TiDB components for cluster `%s` successfully", name)
	}
	return nil
}

// checkTLSEnv check tiflash vserson and show confirm
func checkTLSEnv(topo spec.Topology, clusterName, version string, skipConfirm bool) error {
	// check tiflash version
	if clusterSpec, ok := topo.(*spec.Specification); ok {
		if clusterSpec.GlobalOptions.TLSEnabled {
			if semver.Compare(version, "v4.0.5") < 0 && len(clusterSpec.TiFlashServers) > 0 {
				return fmt.Errorf("TiFlash %s is not supported in TLS enabled cluster", version)
			}
		}

		if len(clusterSpec.PDServers) != 1 {
			return errorx.EnsureStackTrace(fmt.Errorf("Having multiple PD nodes is not supported when enable/disable TLS")).
				WithProperty(tui.SuggestionFromString("Please `scale-in` PD nodes to one and try again."))
		}
	}

	if !skipConfirm {
		return tui.PromptForConfirmOrAbortError(
			fmt.Sprintf("Enable/Disable TLS will %s the cluster `%s`\nDo you want to continue? [y/N]:",
				color.HiYellowString("stop and restart"),
				color.HiYellowString(clusterName),
			))
	}
	return nil
}

// getTLSFileMap
func getTLSFileMap(m *Manager, clusterName string, topo spec.Topology,
	enableTLS, cleanCertificate, skipConfirm bool) (map[string]set.StringSet, error) {
	delFileMap := make(map[string]set.StringSet)

	if !enableTLS && cleanCertificate {
		// get:  host: set(tlsdir)
		delFileMap = getCleanupFiles(topo, false, false, cleanCertificate, false, []string{}, []string{})
		// build file list string
		delFileList := fmt.Sprintf("\n%s:\n %s", color.CyanString("localhost"), m.specManager.Path(clusterName, spec.TLSCertKeyDir))
		for host, fileList := range delFileMap {
			delFileList += fmt.Sprintf("\n%s:", color.CyanString(host))
			for _, dfp := range fileList.Slice() {
				delFileList += fmt.Sprintf("\n %s", dfp)
			}
		}

		m.logger.Warnf("The parameter `%s` will delete the following files: %s", color.YellowString("--clean-certificate"), delFileList)

		if !skipConfirm {
			if err := tui.PromptForConfirmOrAbortError("Do you want to continue? [y/N]:"); err != nil {
				return delFileMap, err
			}
		}
	}

	return delFileMap, nil
}
