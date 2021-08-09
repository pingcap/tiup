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
	"path/filepath"
	"strings"
	"time"

	"github.com/fatih/color"
	"github.com/joomcode/errorx"
	perrs "github.com/pingcap/errors"
	"github.com/pingcap/tiup/pkg/cluster/api"
	"github.com/pingcap/tiup/pkg/cluster/ctxt"
	"github.com/pingcap/tiup/pkg/cluster/executor"
	operator "github.com/pingcap/tiup/pkg/cluster/operation"
	"github.com/pingcap/tiup/pkg/cluster/spec"
	"github.com/pingcap/tiup/pkg/cluster/task"
	"github.com/pingcap/tiup/pkg/logger/log"
	"github.com/pingcap/tiup/pkg/set"
	"github.com/pingcap/tiup/pkg/tui"
)

// CheckOptions contains the options for check command
type CheckOptions struct {
	User         string // username to login to the SSH server
	IdentityFile string // path to the private key file
	UsePassword  bool   // use password instead of identity file for ssh connection
	Opr          *operator.CheckOptions
	ApplyFix     bool // try to apply fixes of failed checks
	ExistCluster bool // check an exist cluster
}

// CheckCluster check cluster before deploying or upgrading
func (m *Manager) CheckCluster(clusterOrTopoName string, opt CheckOptions, gOpt operator.Options) error {
	var topo spec.Specification
	if opt.ExistCluster { // check for existing cluster
		clusterName := clusterOrTopoName

		exist, err := m.specManager.Exist(clusterName)
		if err != nil {
			return err
		}

		if !exist {
			return perrs.Errorf("cluster %s does not exist", clusterName)
		}

		metadata, err := spec.ClusterMetadata(clusterName)
		if err != nil {
			return err
		}
		opt.User = metadata.User
		opt.IdentityFile = m.specManager.Path(clusterName, "ssh", "id_rsa")

		topo = *metadata.Topology
		topo.AdjustByVersion(metadata.Version)
	} else { // check before cluster is deployed
		topoFileName := clusterOrTopoName

		if err := spec.ParseTopologyYaml(topoFileName, &topo); err != nil {
			return err
		}
		spec.ExpandRelativeDir(&topo)

		clusterList, err := m.specManager.GetAllClusters()
		if err != nil {
			return err
		}
		// use a dummy cluster name, the real cluster name is set during deploy
		if err := spec.CheckClusterPortConflict(clusterList, "nonexist-dummy-tidb-cluster", &topo); err != nil {
			return err
		}
		if err := spec.CheckClusterDirConflict(clusterList, "nonexist-dummy-tidb-cluster", &topo); err != nil {
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

	if err := m.fillHostArch(sshConnProps, sshProxyProps, &topo, &gOpt, opt.User); err != nil {
		return err
	}

	if err := checkSystemInfo(sshConnProps, sshProxyProps, &topo, &gOpt, &opt); err != nil {
		return err
	}

	if !opt.ExistCluster {
		return nil
	}
	// following checks are all for existing cluster

	// check PD status
	return m.checkRegionsInfo(clusterOrTopoName, &topo, &gOpt)
}

// checkSystemInfo performs series of checks and tests of the deploy server
func checkSystemInfo(s, p *tui.SSHConnectionProps, topo *spec.Specification, gOpt *operator.Options, opt *CheckOptions) error {
	var (
		collectTasks  []*task.StepDisplay
		checkSysTasks []*task.StepDisplay
		cleanTasks    []*task.StepDisplay
		applyFixTasks []*task.StepDisplay
		downloadTasks []*task.StepDisplay
	)
	insightVer := spec.TiDBComponentVersion(spec.ComponentCheckCollector, "")

	uniqueHosts := map[string]int{}             // host -> ssh-port
	uniqueArchList := make(map[string]struct{}) // map["os-arch"]{}

	roleFilter := set.NewStringSet(gOpt.Roles...)
	nodeFilter := set.NewStringSet(gOpt.Nodes...)
	components := topo.ComponentsByUpdateOrder()
	components = operator.FilterComponent(components, roleFilter)

	for _, comp := range components {
		instances := operator.FilterInstance(comp.Instances(), nodeFilter)
		if len(instances) < 1 {
			continue
		}

		for _, inst := range instances {
			archKey := fmt.Sprintf("%s-%s", inst.OS(), inst.Arch())
			if _, found := uniqueArchList[archKey]; !found {
				uniqueArchList[archKey] = struct{}{}
				t0 := task.NewBuilder().
					Download(
						spec.ComponentCheckCollector,
						inst.OS(),
						inst.Arch(),
						insightVer,
					).
					BuildAsStep(fmt.Sprintf("  - Downloading check tools for %s/%s", inst.OS(), inst.Arch()))
				downloadTasks = append(downloadTasks, t0)
			}

			t1 := task.NewBuilder()
			// checks that applies to each instance
			if opt.ExistCluster {
				t1 = t1.CheckSys(
					inst.GetHost(),
					inst.DeployDir(),
					task.CheckTypePermission,
					topo,
					opt.Opr,
				)
			}
			// if the data dir set in topology is relative, and the home dir of deploy user
			// and the user run the check command is on different partitions, the disk detection
			// may be using incorrect partition for validations.
			for _, dataDir := range spec.MultiDirAbs(opt.User, inst.DataDir()) {
				// build checking tasks
				t1 = t1.
					CheckSys(
						inst.GetHost(),
						dataDir,
						task.CheckTypeFIO,
						topo,
						opt.Opr,
					)
				if opt.ExistCluster {
					t1 = t1.CheckSys(
						inst.GetHost(),
						dataDir,
						task.CheckTypePermission,
						topo,
						opt.Opr,
					)
				}
			}

			// checks that applies to each host
			if _, found := uniqueHosts[inst.GetHost()]; !found {
				uniqueHosts[inst.GetHost()] = inst.GetSSHPort()
				// build system info collecting tasks
				t2 := task.NewBuilder().
					RootSSH(
						inst.GetHost(),
						inst.GetSSHPort(),
						opt.User,
						s.Password,
						s.IdentityFile,
						s.IdentityFilePassphrase,
						gOpt.SSHTimeout,
						gOpt.OptTimeout,
						gOpt.SSHProxyHost,
						gOpt.SSHProxyPort,
						gOpt.SSHProxyUser,
						p.Password,
						p.IdentityFile,
						p.IdentityFilePassphrase,
						gOpt.SSHProxyTimeout,
						gOpt.SSHType,
						topo.GlobalOptions.SSHType,
					).
					Mkdir(opt.User, inst.GetHost(), filepath.Join(task.CheckToolsPathDir, "bin")).
					CopyComponent(
						spec.ComponentCheckCollector,
						inst.OS(),
						inst.Arch(),
						insightVer,
						"", // use default srcPath
						inst.GetHost(),
						task.CheckToolsPathDir,
					).
					Shell(
						inst.GetHost(),
						filepath.Join(task.CheckToolsPathDir, "bin", "insight"),
						"",
						false,
					).
					BuildAsStep(fmt.Sprintf("  - Getting system info of %s:%d", inst.GetHost(), inst.GetSSHPort()))
				collectTasks = append(collectTasks, t2)

				// build checking tasks
				t1 = t1.
					// check for general system info
					CheckSys(
						inst.GetHost(),
						"",
						task.CheckTypeSystemInfo,
						topo,
						opt.Opr,
					).
					CheckSys(
						inst.GetHost(),
						"",
						task.CheckTypePartitions,
						topo,
						opt.Opr,
					).
					// check for listening port
					Shell(
						inst.GetHost(),
						"ss -lnt",
						"",
						false,
					).
					CheckSys(
						inst.GetHost(),
						"",
						task.CheckTypePort,
						topo,
						opt.Opr,
					).
					// check for system limits
					Shell(
						inst.GetHost(),
						"cat /etc/security/limits.conf",
						"",
						false,
					).
					CheckSys(
						inst.GetHost(),
						"",
						task.CheckTypeSystemLimits,
						topo,
						opt.Opr,
					).
					// check for kernel params
					Shell(
						inst.GetHost(),
						"sysctl -a",
						"",
						true,
					).
					CheckSys(
						inst.GetHost(),
						"",
						task.CheckTypeSystemConfig,
						topo,
						opt.Opr,
					).
					// check for needed system service
					CheckSys(
						inst.GetHost(),
						"",
						task.CheckTypeService,
						topo,
						opt.Opr,
					).
					// check for needed packages
					CheckSys(
						inst.GetHost(),
						"",
						task.CheckTypePackage,
						topo,
						opt.Opr,
					)
			}

			checkSysTasks = append(
				checkSysTasks,
				t1.BuildAsStep(fmt.Sprintf("  - Checking node %s", inst.GetHost())),
			)

			t3 := task.NewBuilder().
				RootSSH(
					inst.GetHost(),
					inst.GetSSHPort(),
					opt.User,
					s.Password,
					s.IdentityFile,
					s.IdentityFilePassphrase,
					gOpt.SSHTimeout,
					gOpt.OptTimeout,
					gOpt.SSHProxyHost,
					gOpt.SSHProxyPort,
					gOpt.SSHProxyUser,
					p.Password,
					p.IdentityFile,
					p.IdentityFilePassphrase,
					gOpt.SSHProxyTimeout,
					gOpt.SSHType,
					topo.GlobalOptions.SSHType,
				).
				Rmdir(inst.GetHost(), task.CheckToolsPathDir).
				BuildAsStep(fmt.Sprintf("  - Cleanup check files on %s:%d", inst.GetHost(), inst.GetSSHPort()))
			cleanTasks = append(cleanTasks, t3)
		}
	}

	t := task.NewBuilder().
		ParallelStep("+ Download necessary tools", false, downloadTasks...).
		ParallelStep("+ Collect basic system information", false, collectTasks...).
		ParallelStep("+ Check system requirements", false, checkSysTasks...).
		ParallelStep("+ Cleanup check files", false, cleanTasks...).
		Build()

	ctx := ctxt.New(context.Background(), gOpt.Concurrency)
	if err := t.Execute(ctx); err != nil {
		if errorx.Cast(err) != nil {
			// FIXME: Map possible task errors and give suggestions.
			return err
		}
		return perrs.Trace(err)
	}

	// FIXME: add fix result to output
	checkResultTable := [][]string{
		// Header
		{"Node", "Check", "Result", "Message"},
	}
	checkResults := make([]HostCheckResult, 0)
	for host := range uniqueHosts {
		tf := task.NewBuilder().
			RootSSH(
				host,
				uniqueHosts[host],
				opt.User,
				s.Password,
				s.IdentityFile,
				s.IdentityFilePassphrase,
				gOpt.SSHTimeout,
				gOpt.OptTimeout,
				gOpt.SSHProxyHost,
				gOpt.SSHProxyPort,
				gOpt.SSHProxyUser,
				p.Password,
				p.IdentityFile,
				p.IdentityFilePassphrase,
				gOpt.SSHProxyTimeout,
				gOpt.SSHType,
				topo.GlobalOptions.SSHType,
			)
		res, err := handleCheckResults(ctx, host, opt, tf)
		if err != nil {
			continue
		}
		checkResults = append(checkResults, res...)
		applyFixTasks = append(applyFixTasks, tf.BuildAsStep(fmt.Sprintf("  - Applying changes on %s", host)))
	}
	resLines := formatHostCheckResults(checkResults)
	checkResultTable = append(checkResultTable, resLines...)

	// print check results *before* trying to applying checks
	// FIXME: add fix result to output, and display the table after fixing
	tui.PrintTable(checkResultTable, true)

	if opt.ApplyFix {
		tc := task.NewBuilder().
			ParallelStep("+ Try to apply changes to fix failed checks", false, applyFixTasks...).
			Build()
		if err := tc.Execute(ctx); err != nil {
			if errorx.Cast(err) != nil {
				// FIXME: Map possible task errors and give suggestions.
				return err
			}
			return perrs.Trace(err)
		}
	}

	return nil
}

// HostCheckResult represents the check result of each node
type HostCheckResult struct {
	Node    string `json:"node"`
	Name    string `json:"name"`
	Status  string `json:"status"`
	Message string `json:"message"`
}

// handleCheckResults parses the result of checks
func handleCheckResults(ctx context.Context, host string, opt *CheckOptions, t *task.Builder) ([]HostCheckResult, error) {
	rr, _ := ctxt.GetInner(ctx).GetCheckResults(host)
	if len(rr) < 1 {
		return nil, fmt.Errorf("no check results found for %s", host)
	}
	results := []*operator.CheckResult{}
	for _, r := range rr {
		results = append(results, r.(*operator.CheckResult))
	}

	items := make([]HostCheckResult, 0)
	// log.Infof("Check results of %s: (only errors and important info are displayed)", color.HiCyanString(host))
	for _, r := range results {
		var item HostCheckResult
		if r.Err != nil {
			if r.IsWarning() {
				item = HostCheckResult{Node: host, Name: r.Name, Status: "Warn", Message: r.Error()}
			} else {
				item = HostCheckResult{Node: host, Name: r.Name, Status: "Fail", Message: r.Error()}
			}
			if !opt.ApplyFix {
				items = append(items, item)
				continue
			}
			msg, err := fixFailedChecks(host, r, t)
			if err != nil {
				log.Debugf("%s: fail to apply fix to %s (%s)", host, r.Name, err)
			}
			if msg != "" {
				// show auto fixing info
				item.Message = msg
			}
		} else if r.Msg != "" {
			item = HostCheckResult{Node: host, Name: r.Name, Status: "Pass", Message: r.Msg}
		}

		// show errors and messages only, ignore empty lines
		// if len(line) > 0 {
		if len(item.Node) > 0 {
			items = append(items, item)
		}
	}

	return items, nil
}

func formatHostCheckResults(results []HostCheckResult) [][]string {
	lines := make([][]string, 0)
	for _, r := range results {
		var coloredStatus string
		switch r.Status {
		case "Warn":
			coloredStatus = color.YellowString(r.Status)
		case "Fail":
			coloredStatus = color.HiRedString(r.Status)
		default:
			coloredStatus = color.GreenString(r.Status)
		}
		line := []string{r.Node, r.Name, coloredStatus, r.Message}
		lines = append(lines, line)
	}
	return lines
}

// fixFailedChecks tries to automatically apply changes to fix failed checks
func fixFailedChecks(host string, res *operator.CheckResult, t *task.Builder) (string, error) {
	msg := ""
	switch res.Name {
	case operator.CheckNameSysService:
		if strings.Contains(res.Msg, "not found") {
			return "", nil
		}
		fields := strings.Fields(res.Msg)
		if len(fields) < 2 {
			return "", fmt.Errorf("can not perform action of service, %s", res.Msg)
		}
		t.SystemCtl(host, fields[1], fields[0], false)
		msg = fmt.Sprintf("will try to '%s'", color.HiBlueString(res.Msg))
	case operator.CheckNameSysctl:
		fields := strings.Fields(res.Msg)
		if len(fields) < 3 {
			return "", fmt.Errorf("can not set kernel parameter, %s", res.Msg)
		}
		t.Sysctl(host, fields[0], fields[2])
		msg = fmt.Sprintf("will try to set '%s'", color.HiBlueString(res.Msg))
	case operator.CheckNameLimits:
		fields := strings.Fields(res.Msg)
		if len(fields) < 4 {
			return "", fmt.Errorf("can not set limits, %s", res.Msg)
		}
		t.Limit(host, fields[0], fields[1], fields[2], fields[3])
		msg = fmt.Sprintf("will try to set '%s'", color.HiBlueString(res.Msg))
	case operator.CheckNameSELinux:
		t.Shell(host,
			fmt.Sprintf(
				"sed -i 's/^[[:blank:]]*SELINUX=enforcing/SELINUX=disabled/g' %s && %s",
				"/etc/selinux/config",
				"setenforce 0",
			),
			"",
			true)
		msg = fmt.Sprintf("will try to %s, reboot might be needed", color.HiBlueString("disable SELinux"))
	case operator.CheckNameTHP:
		t.Shell(host,
			fmt.Sprintf(`if [ -d %[1]s ]; then echo never > %[1]s/defrag && echo never > %[1]s/enabled; fi`, "/sys/kernel/mm/transparent_hugepage"),
			"",
			true)
		msg = fmt.Sprintf("will try to %s, please check again after reboot", color.HiBlueString("disable THP"))
	default:
		msg = fmt.Sprintf("%s, auto fixing not supported", res)
	}
	return msg, nil
}

// checkRegionsInfo checks peer status from PD
func (m *Manager) checkRegionsInfo(clusterName string, topo *spec.Specification, gOpt *operator.Options) error {
	log.Infof("Checking region status of the cluster %s...", clusterName)

	tlsConfig, err := topo.TLSConfig(m.specManager.Path(clusterName, spec.TLSCertKeyDir))
	if err != nil {
		return err
	}
	pdClient := api.NewPDClient(
		topo.GetPDList(),
		time.Second*time.Duration(gOpt.APITimeout),
		tlsConfig,
	)

	hasUnhealthy := false
	for _, state := range []string{
		"miss-peer",
		"pending-peer",
	} {
		rInfo, err := pdClient.CheckRegion(state)
		if err != nil {
			return err
		}
		if rInfo.Count > 0 {
			log.Warnf(
				"Regions are not fully healthy: %s",
				color.YellowString("%d %s", rInfo.Count, state),
			)
			hasUnhealthy = true
		}
	}
	if hasUnhealthy {
		log.Warnf("Please fix unhealthy regions before other operations.")
	} else {
		log.Infof("All regions are healthy.")
	}
	return nil
}
