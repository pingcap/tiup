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
	"github.com/pingcap/tiup/pkg/logger/log"
	"github.com/pingcap/tiup/pkg/tui"
	"golang.org/x/mod/semver"
)

// TLS set cluster enable/disable encrypt communication by tls
func (m *Manager) TLS(name string, gOpt operator.Options, enable, skipRestart, cleanCertificate, reloadCertificate bool) error {
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
	globalOptions.TLSEnabled = enable

	//  load certificate file
	if enable {
		if err := loadCertificate(m, name, globalOptions, reloadCertificate); err != nil {
			return err
		}
	}

	// check tiflash version
	if clusterSpec, ok := topo.(*spec.Specification); ok {
		if clusterSpec.GlobalOptions.TLSEnabled &&
			semver.Compare(base.Version, "v4.0.5") < 0 &&
			len(clusterSpec.TiFlashServers) > 0 {
			return fmt.Errorf("TiFlash %s is not supported in TLS enabled cluster", base.Version)
		}
	}

	if err := tui.PromptForConfirmOrAbortError(
		fmt.Sprintf("Enable/Disable TLS will restart the cluster `%s` , the policy is %s.\nDo you want to continue? [y/N]:",
			color.HiYellowString(name),
			color.HiRedString(fmt.Sprintf("%v", !skipRestart)),
		),
	); err != nil {
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

	// Build the tls  tasks
	t, err := buildTLSTask(
		m, name, metadata, gOpt, sshProxyProps, skipRestart, cleanCertificate)
	if err != nil {
		return err
	}

	if err := t.Execute(ctxt.New(context.Background(), gOpt.Concurrency)); err != nil {
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
		log.Infof("\tCleanup localhost tls file success")
	}

	if enable {
		log.Infof("Enable cluster `%s` TLS between TiDB components successfully", name)
	} else {
		log.Infof("Disable cluster `%s` TLS between TiDB components successfully", name)
	}

	if skipRestart {
		log.Infof("Please restart the cluster `%s` to apply the TLS configuration", name)
	}

	return nil
}

// loadCertificate
// certificate file exists and reload is true
// will reload certificate file
func loadCertificate(m *Manager, clusterName string, globalOptions *spec.GlobalOptions, reload bool) error {
	err := m.checkCertificate(clusterName)

	// no need to reload and the file already exists
	if !reload && err == nil {
		return nil
	}

	_, err = m.genAndSaveCertificate(clusterName, globalOptions)

	return err
}
