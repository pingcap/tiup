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

package command

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/fatih/color"
	"github.com/joomcode/errorx"
	"github.com/pingcap-incubator/tiup-cluster/pkg/bindversion"
	"github.com/pingcap-incubator/tiup-cluster/pkg/cliutil"
	"github.com/pingcap-incubator/tiup-cluster/pkg/cliutil/prepare"
	"github.com/pingcap-incubator/tiup-cluster/pkg/log"
	"github.com/pingcap-incubator/tiup-cluster/pkg/logger"
	"github.com/pingcap-incubator/tiup-cluster/pkg/meta"
	"github.com/pingcap-incubator/tiup-cluster/pkg/operation"
	"github.com/pingcap-incubator/tiup-cluster/pkg/task"
	"github.com/pingcap-incubator/tiup-cluster/pkg/utils"
	"github.com/pingcap/errors"
	"github.com/spf13/cobra"
)

type checkOptions struct {
	user         string // username to login to the SSH server
	identityFile string // path to the private key file
	opr          *operator.CheckOptions
	applyFix     bool // try to apply fixes of failed checks
}

func newCheckCmd() *cobra.Command {
	opt := checkOptions{
		opr: &operator.CheckOptions{},
	}
	cmd := &cobra.Command{
		Use:   "check <topology.yml>",
		Short: "Perform preflight checks for the cluster.",
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) != 1 {
				return cmd.Help()
			}

			logger.EnableAuditLog()
			var topo meta.TopologySpecification
			if err := utils.ParseTopologyYaml(args[0], &topo); err != nil {
				return err
			}

			// use a dummy cluster name, the real cluster name is set during deploy
			if err := prepare.CheckClusterPortConflict("nonexist-dummy-tidb-cluster", &topo); err != nil {
				return err
			}
			if err := prepare.CheckClusterDirConflict("nonexist-dummy-tidb-cluster", &topo); err != nil {
				return err
			}

			sshConnProps, err := cliutil.ReadIdentityFileOrPassword(opt.identityFile)
			if err != nil {
				return err
			}

			if err := checkSystemInfo(sshConnProps, &topo, &opt); err != nil {
				return err
			}

			return nil
		},
	}

	cmd.Flags().StringVar(&opt.user, "user", "root", "The user name to login via SSH. The user must has root (or sudo) privilege.")
	cmd.Flags().StringVarP(&opt.identityFile, "identity_file", "i", "", "The path of the SSH identity file. If specified, public key authentication will be used.")

	cmd.Flags().BoolVar(&opt.opr.EnableCPU, "enable-cpu", false, "Enable CPU thread count check")
	cmd.Flags().BoolVar(&opt.opr.EnableMem, "enable-mem", false, "Enable memory size check")
	cmd.Flags().BoolVar(&opt.opr.EnableDisk, "enable-disk", false, "Enable disk IO (fio) check")
	cmd.Flags().BoolVar(&opt.applyFix, "apply", false, "Try to fix failed checks")

	return cmd
}

// checkSystemInfo performs series of checks and tests of the deploy server
func checkSystemInfo(s *cliutil.SSHConnectionProps, topo *meta.TopologySpecification, opt *checkOptions) error {
	var (
		collectTasks  []*task.StepDisplay
		checkSysTasks []*task.StepDisplay
		cleanTasks    []*task.StepDisplay
		applyFixTasks []*task.StepDisplay
	)
	insightVer := bindversion.ComponentVersion(bindversion.ComponentCheckCollector, "")

	uniqueHosts := map[string]int{} // host -> ssh-port
	topo.IterInstance(func(inst meta.Instance) {
		if _, found := uniqueHosts[inst.GetHost()]; !found {
			uniqueHosts[inst.GetHost()] = inst.GetSSHPort()

			// build system info collecting tasks
			t1 := task.NewBuilder().
				RootSSH(
					inst.GetHost(),
					inst.GetSSHPort(),
					opt.user,
					s.Password,
					s.IdentityFile,
					s.IdentityFilePassphrase,
					sshTimeout,
				).
				Mkdir(opt.user, inst.GetHost(), filepath.Join(task.CheckToolsPathDir, "bin")).
				Chown(opt.user, inst.GetHost(), task.CheckToolsPathDir).
				CopyComponent(bindversion.ComponentCheckCollector, insightVer, inst.GetHost(), task.CheckToolsPathDir).
				Shell(
					inst.GetHost(),
					filepath.Join(task.CheckToolsPathDir, "bin", "insight"),
					false,
				).
				BuildAsStep(fmt.Sprintf("  - Getting system info of %s:%d", inst.GetHost(), inst.GetSSHPort()))
			collectTasks = append(collectTasks, t1)

			// build checking tasks
			t2 := task.NewBuilder().
				CheckSys(
					inst.GetHost(),
					inst.DataDir(),
					task.CheckTypeSystemInfo,
					topo,
					opt.opr,
				).
				CheckSys(
					inst.GetHost(),
					inst.DataDir(),
					task.CheckTypePartitions,
					topo,
					opt.opr,
				).
				Shell(
					inst.GetHost(),
					"cat /etc/security/limits.conf",
					false,
				).
				CheckSys(
					inst.GetHost(),
					inst.DataDir(),
					task.CheckTypeSystemLimits,
					topo,
					opt.opr,
				).
				Shell(
					inst.GetHost(),
					"sysctl -a",
					true,
				).
				CheckSys(
					inst.GetHost(),
					inst.DataDir(),
					task.CheckTypeSystemConfig,
					topo,
					opt.opr,
				).
				CheckSys(
					inst.GetHost(),
					inst.DataDir(),
					task.CheckTypeService,
					topo,
					opt.opr,
				).
				CheckSys(
					inst.GetHost(),
					inst.DataDir(),
					task.CheckTypePackage,
					topo,
					opt.opr,
				).
				CheckSys(
					inst.GetHost(),
					inst.DataDir(),
					task.CheckTypeFIO,
					topo,
					opt.opr,
				).
				BuildAsStep(fmt.Sprintf("  - Checking node %s", inst.GetHost()))
			checkSysTasks = append(checkSysTasks, t2)

			t3 := task.NewBuilder().
				RootSSH(
					inst.GetHost(),
					inst.GetSSHPort(),
					opt.user,
					s.Password,
					s.IdentityFile,
					s.IdentityFilePassphrase,
					sshTimeout,
				).
				Rmdir(opt.user, inst.GetHost(), task.CheckToolsPathDir).
				BuildAsStep(fmt.Sprintf("  - Cleanup check files on %s:%d", inst.GetHost(), inst.GetSSHPort()))
			cleanTasks = append(cleanTasks, t3)
		}
	})

	t := task.NewBuilder().
		Download(bindversion.ComponentCheckCollector, insightVer).
		ParallelStep("+ Collect basic system information", collectTasks...).
		ParallelStep("+ Check system requirements", checkSysTasks...).
		ParallelStep("+ Cleanup check files", cleanTasks...).
		Build()

	ctx := task.NewContext()
	if err := t.Execute(ctx); err != nil {
		if errorx.Cast(err) != nil {
			// FIXME: Map possible task errors and give suggestions.
			return err
		}
		return errors.Trace(err)
	}

	var checkResultTable [][]string
	// FIXME: add fix result to output
	checkResultTable = [][]string{
		// Header
		{"Node", "Check", "Result", "Message"},
	}
	for host := range uniqueHosts {
		tf := task.NewBuilder().
			RootSSH(
				host,
				uniqueHosts[host],
				opt.user,
				s.Password,
				s.IdentityFile,
				s.IdentityFilePassphrase,
				sshTimeout,
			)
		resLines, err := handleCheckResults(ctx, host, opt, tf)
		if err != nil {
			continue
		}
		applyFixTasks = append(applyFixTasks, tf.BuildAsStep(fmt.Sprintf("  - Applying changes on %s", host)))
		checkResultTable = append(checkResultTable, resLines...)
	}

	// print check results *before* trying to applying checks
	// FIXME: add fix result to output, and display the table after fixing
	cliutil.PrintTable(checkResultTable, true)

	if opt.applyFix {
		tc := task.NewBuilder().
			ParallelStep("+ Try to apply changes to fix failed checks", applyFixTasks...).
			Build()
		if err := tc.Execute(ctx); err != nil {
			if errorx.Cast(err) != nil {
				// FIXME: Map possible task errors and give suggestions.
				return err
			}
			return errors.Trace(err)
		}
	}

	return nil
}

// handleCheckResults parses the result of checks
func handleCheckResults(ctx *task.Context, host string, opt *checkOptions, t *task.Builder) ([][]string, error) {
	results, _ := ctx.GetCheckResults(host)
	if len(results) < 1 {
		return nil, fmt.Errorf("no check results found for %s", host)
	}

	lines := make([][]string, 0)
	//log.Infof("Check results of %s: (only errors and important info are displayed)", color.HiCyanString(host))
	for _, r := range results {
		var line []string
		if r.Err != nil {
			if r.IsWarning() {
				line = []string{host, r.Name, color.YellowString("Warn"), r.Error()}
			} else {
				line = []string{host, r.Name, color.HiRedString("Fail"), r.Error()}
			}
			if !opt.applyFix {
				lines = append(lines, line)
				continue
			}
			msg, err := fixFailedChecks(ctx, host, r, t)
			if err != nil {
				log.Debugf("%s: fail to apply fix to %s (%s)", host, r.Name, err)
			}
			if msg != "" {
				// show auto fixing info
				line[len(line)-1] = msg
			}
		} else if r.Msg != "" {
			line = []string{host, r.Name, color.GreenString("Pass"), r.Msg}
		}

		// show errors and messages only, ignore empty lines
		if len(line) > 0 {
			lines = append(lines, line)
		}
	}

	return lines, nil
}

// fixFailedChecks tries to automatically apply changes to fix failed checks
func fixFailedChecks(ctx *task.Context, host string, res *operator.CheckResult, t *task.Builder) (string, error) {
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
		t.SystemCtl(host, fields[1], fields[0])
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
				"sed -i 's/SELINUX=enforcing/SELINUX=no/g' %s && %s",
				"/etc/selinux/config",
				"setenforce 0",
			),
			true)
		msg = fmt.Sprintf("will try to %s, reboot might be needed", color.HiBlueString("disable SELinux"))
	case operator.CheckNameOSVer,
		operator.CheckNameCPUThreads,
		operator.CheckNameDisks,
		operator.CheckNameEpoll,
		operator.CheckNameMem,
		operator.CheckNameCommand,
		operator.CheckNameFio:
		// don't show unsupported message for checks that are impossible to fix by us
		return "", nil
	default:
		msg = fmt.Sprintf("%s, auto fixing not supported", res)
	}
	return msg, nil
}
