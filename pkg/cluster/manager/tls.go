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
	"crypto/tls"
	"crypto/x509"
	"encoding/pem"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/fatih/color"
	"github.com/joomcode/errorx"
	perrs "github.com/pingcap/errors"
	"github.com/pingcap/tiup/pkg/cluster/clusterutil"
	"github.com/pingcap/tiup/pkg/cluster/ctxt"
	"github.com/pingcap/tiup/pkg/cluster/executor"
	operator "github.com/pingcap/tiup/pkg/cluster/operation"
	"github.com/pingcap/tiup/pkg/cluster/spec"
	"github.com/pingcap/tiup/pkg/crypto"
	"github.com/pingcap/tiup/pkg/crypto/rand"
	"github.com/pingcap/tiup/pkg/set"
	"github.com/pingcap/tiup/pkg/tui"
	"github.com/pingcap/tiup/pkg/utils"
	"software.sslmate.com/src/go-pkcs12"
)

// CustomTLSOptions holds the flags for enabling custom (BYOC) TLS mode.
type CustomTLSOptions struct {
	Enabled    bool
	ClientCA   string
	ClientCert string
	ClientKey  string
}

// validateCustomClientCerts checks that the provided client cert files exist,
// the cert+key pair is loadable, and the CA is a valid PEM-encoded certificate.
func validateCustomClientCerts(caPath, certPath, keyPath string) error {
	for _, p := range []string{caPath, certPath, keyPath} {
		if !utils.IsExist(p) {
			return fmt.Errorf("file not found: %s", p)
		}
	}

	// Validate cert+key pair
	if _, err := tls.LoadX509KeyPair(certPath, keyPath); err != nil {
		return perrs.Annotatef(err, "invalid client cert/key pair (%s, %s)", certPath, keyPath)
	}

	// Validate CA is parseable
	caPEM, err := os.ReadFile(caPath)
	if err != nil {
		return perrs.Annotatef(err, "cannot read CA file %s", caPath)
	}
	caBlock, _ := pem.Decode(caPEM)
	if caBlock == nil {
		return fmt.Errorf("failed to decode PEM from CA file %s", caPath)
	}
	if _, err := x509.ParseCertificate(caBlock.Bytes); err != nil {
		return perrs.Annotatef(err, "invalid CA certificate in %s", caPath)
	}

	return nil
}

// TLS set cluster enable/disable encrypt communication by tls
func (m *Manager) TLS(name string, gOpt operator.Options, enable, cleanCertificate, reloadCertificate, skipConfirm bool, customOpts CustomTLSOptions) error {
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

	globalOptions := topo.BaseTopo().GlobalOptions
	prevMode := globalOptions.TLSMode

	// Determine target TLS mode before the early-return check so we can
	// detect mode transitions (e.g. managed→custom) even when TLSEnabled
	// is already true.
	if enable && customOpts.Enabled {
		if err := validateCustomClientCerts(customOpts.ClientCA, customOpts.ClientCert, customOpts.ClientKey); err != nil {
			return err
		}
	}

	// If TLS state and mode are already at the target, nothing to do (unless --force).
	if globalOptions.TLSEnabled == enable && !gOpt.Force {
		if enable {
			m.logger.Infof("cluster `%s` TLS status is already enabled\n", name)
		} else {
			m.logger.Infof("cluster `%s` TLS status is already disabled\n", name)
		}
		return nil
	}

	// Handle mode transitions that require --force.
	if enable && globalOptions.TLSEnabled {
		// Cluster already has TLS enabled — this is a mode switch.
		isManagedPrev := prevMode == "" || prevMode == spec.TLSModeManaged
		if isManagedPrev && customOpts.Enabled {
			// managed → custom
			if !gOpt.Force {
				return fmt.Errorf("switching from managed to custom TLS requires --force")
			}
		}
		if prevMode == spec.TLSModeCustom && !customOpts.Enabled {
			// custom → managed
			if !gOpt.Force {
				return fmt.Errorf("switching from custom to managed TLS requires --force")
			}
			if !skipConfirm {
				if err := tui.PromptForConfirmOrAbortError(
					"Switching from custom to managed TLS will generate a new self-signed CA.\n" +
						"Existing custom certificates on remote nodes will no longer be trusted.\n" +
						"Do you want to continue? [y/N]:"); err != nil {
					return err
				}
			}
			// A new CA must be generated, so force certificate reload.
			reloadCertificate = true
		}
	}

	// Set target TLS state.
	globalOptions.TLSEnabled = enable
	if enable && customOpts.Enabled {
		globalOptions.TLSMode = spec.TLSModeCustom
	} else if enable {
		globalOptions.TLSMode = spec.TLSModeManaged
	}
	if !enable {
		globalOptions.TLSMode = ""
	}

	if err := checkTLSEnv(topo, name, base.Version, skipConfirm); err != nil {
		return err
	}

	// If custom mode, copy client certs to the standard TiUP TLS dir (with backup).
	if globalOptions.IsCustomTLS() {
		if err := m.swapClientCertFiles(name, customOpts.ClientCA, customOpts.ClientCert, customOpts.ClientKey); err != nil {
			return perrs.Annotate(err, "failed to install custom client certificates")
		}
	}

	var sshProxyProps = &tui.SSHConnectionProps{}
	if gOpt.SSHType != executor.SSHTypeNone {
		var err error
		if len(gOpt.SSHProxyHost) != 0 {
			if sshProxyProps, err = tui.ReadIdentityFileOrPassword(gOpt.SSHProxyIdentity, gOpt.SSHProxyUsePassword); err != nil {
				return err
			}
		}
	}

	// delFileMap: files that need to be cleaned up, if flag --cleanCertificate are used
	delFileMap, err := getTLSFileMap(m, name, topo, enable, cleanCertificate, skipConfirm)
	if err != nil {
		return err
	}

	// Build the tls tasks
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
		if globalOptions.IsCustomTLS() {
			m.logger.Infof("Enabled custom TLS for cluster `%s` successfully", name)
		} else {
			m.logger.Infof("Enabled TLS between TiDB components for cluster `%s` successfully", name)
		}
	} else {
		m.logger.Infof("Disabled TLS between TiDB components for cluster `%s` successfully", name)
	}
	return nil
}

// checkTLSEnv check tiflash version and show confirm
func checkTLSEnv(topo spec.Topology, clusterName, version string, skipConfirm bool) error {
	// check tiflash version
	if err := checkTiFlashWithTLS(topo, version); err != nil {
		return err
	}

	if clusterSpec, ok := topo.(*spec.Specification); ok {
		if len(clusterSpec.PDServers) != 1 {
			return errorx.EnsureStackTrace(fmt.Errorf("having multiple PD nodes is not supported when enable/disable TLS")).
				WithProperty(tui.SuggestionFromString("Please `scale-in` PD nodes to one and try again."))
		}
	}

	if err := topo.Validate(); err != nil {
		return err
	}

	if !skipConfirm {
		return tui.PromptForConfirmOrAbortError(
			"%s", fmt.Sprintf("Enable/Disable TLS will %s the cluster `%s`\nDo you want to continue? [y/N]:",
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
		var delFileList strings.Builder
		delFileList.WriteString(fmt.Sprintf("\n%s:\n %s", color.CyanString("localhost"), m.specManager.Path(clusterName, spec.TLSCertKeyDir)))
		for host, fileList := range delFileMap {
			delFileList.WriteString(fmt.Sprintf("\n%s:", color.CyanString(host)))
			for _, dfp := range fileList.Slice() {
				delFileList.WriteString(fmt.Sprintf("\n %s", dfp))
			}
		}

		m.logger.Warnf("The parameter `%s` will delete the following files: %s", color.YellowString("--clean-certificate"), delFileList.String())

		if !skipConfirm {
			if err := tui.PromptForConfirmOrAbortError("Do you want to continue? [y/N]:"); err != nil {
				return delFileMap, err
			}
		}
	}

	return delFileMap, nil
}

// SwapClientCert replaces the client certificate files in the standard TiUP TLS
// directory for a cluster with user-provided files, creating timestamped backups
// of the originals. No cluster restart is performed — these certs are used by
// TiUP itself, not by remote components.
//
// Only valid for clusters in custom certificate mode.
func (m *Manager) SwapClientCert(clusterName, caPath, certPath, keyPath string) error {
	metadata, err := m.meta(clusterName)
	if err != nil {
		return err
	}
	if !metadata.GetTopology().BaseTopo().GlobalOptions.IsCustomTLS() {
		return fmt.Errorf("cluster `%s` is not in custom certificate mode; swap-client-cert only applies to clusters with --custom TLS enabled", clusterName)
	}
	return m.swapClientCertFiles(clusterName, caPath, certPath, keyPath)
}

// swapClientCertFiles performs the actual file swap without checking the cluster
// mode. Called by SwapClientCert (after the mode guard) and by TLS() (which has
// already set the mode in memory but not yet persisted it).
func (m *Manager) swapClientCertFiles(clusterName, caPath, certPath, keyPath string) error {
	for _, p := range []string{caPath, certPath, keyPath} {
		if !utils.IsExist(p) {
			return fmt.Errorf("file not found: %s", p)
		}
	}

	tlsDir := m.specManager.Path(clusterName, spec.TLSCertKeyDir)
	timestamp := time.Now().Format("20060102T150405")

	// Backup existing files
	for _, name := range []string{spec.TLSCACert, spec.TLSClientCert, spec.TLSClientKey, spec.PFXClientCert} {
		src := filepath.Join(tlsDir, name)
		if utils.IsExist(src) {
			if err := os.Rename(src, fmt.Sprintf("%s.bak.%s", src, timestamp)); err != nil {
				return perrs.Annotatef(err, "cannot backup %s", src)
			}
		}
	}

	// Copy new files
	filesToCopy := []struct {
		src, dstName string
	}{
		{caPath, spec.TLSCACert},
		{certPath, spec.TLSClientCert},
		{keyPath, spec.TLSClientKey},
	}
	for _, f := range filesToCopy {
		data, err := os.ReadFile(f.src)
		if err != nil {
			return perrs.Annotatef(err, "cannot read %s", f.src)
		}
		if err := os.WriteFile(filepath.Join(tlsDir, f.dstName), data, 0600); err != nil {
			return perrs.Annotatef(err, "cannot write %s", f.dstName)
		}
	}

	// Regenerate client.pfx from new cert+key+CA (best-effort).
	// User-provided keys may not be RSA, so we use go-pkcs12 directly
	// rather than the crypto.PrivKey interface.
	if err := regenerateClientPFX(tlsDir); err != nil {
		m.logger.Warnf("Failed to regenerate %s (non-fatal): %v", spec.PFXClientCert, err)
	}

	return nil
}

// regenerateClientPFX reads the client cert, key, and CA from tlsDir and
// encodes them into a PKCS#12 file. Returns an error on any failure — the
// caller decides whether to treat it as fatal.
func regenerateClientPFX(tlsDir string) error {
	certPEM, err := os.ReadFile(filepath.Join(tlsDir, spec.TLSClientCert))
	if err != nil {
		return err
	}
	keyPEM, err := os.ReadFile(filepath.Join(tlsDir, spec.TLSClientKey))
	if err != nil {
		return err
	}
	caPEM, err := os.ReadFile(filepath.Join(tlsDir, spec.TLSCACert))
	if err != nil {
		return err
	}

	tlsCert, err := tls.X509KeyPair(certPEM, keyPEM)
	if err != nil {
		return err
	}
	clientCert, err := x509.ParseCertificate(tlsCert.Certificate[0])
	if err != nil {
		return err
	}

	caBlock, _ := pem.Decode(caPEM)
	if caBlock == nil {
		return fmt.Errorf("failed to decode CA PEM")
	}
	caCert, err := x509.ParseCertificate(caBlock.Bytes)
	if err != nil {
		return err
	}

	pfxData, err := pkcs12.Encode(
		rand.Reader,
		tlsCert.PrivateKey,
		clientCert,
		[]*x509.Certificate{caCert},
		crypto.PKCS12Password,
	)
	if err != nil {
		return err
	}

	return os.WriteFile(filepath.Join(tlsDir, spec.PFXClientCert), pfxData, 0600)
}
