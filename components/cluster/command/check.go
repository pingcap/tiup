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
	"path"
	"path/filepath"
	"strings"

	"github.com/fatih/color"
	"github.com/joomcode/errorx"
	perrs "github.com/pingcap/errors"
	"github.com/pingcap/tiup/pkg/cliutil"
	"github.com/pingcap/tiup/pkg/cluster/clusterutil"
	"github.com/pingcap/tiup/pkg/cluster/executor"
	operator "github.com/pingcap/tiup/pkg/cluster/operation"
	"github.com/pingcap/tiup/pkg/cluster/spec"
	"github.com/pingcap/tiup/pkg/cluster/task"
	"github.com/pingcap/tiup/pkg/logger/log"
	"github.com/pingcap/tiup/pkg/utils"
	"github.com/spf13/cobra"
)

type checkOptions struct {
	user         string // username to login to the SSH server
	identityFile string // path to the private key file
	usePassword  bool   // use password instead of identity file for ssh connection
	opr          *operator.CheckOptions
	applyFix     bool // try to apply fixes of failed checks
	existCluster bool // check an exist cluster
}

func newCheckCmd() *cobra.Command {
	opt := checkOptions{
		opr:          &operator.CheckOptions{},
		identityFile: path.Join(utils.UserHome(), ".ssh", "id_rsa"),
	}
	cmd := &cobra.Command{
		Use:   "check <topology.yml | cluster-name>",
		Short: "Perform preflight checks for the cluster.",
		Long: `Perform preflight checks for the cluster. By default, it checks deploy servers
before a cluster is deployed, the input is the topology.yaml for the cluster.
If '--cluster' is set, it will perform checks for an existing cluster, the input
is the cluster name. Some checks are ignore in this mode, such as port and dir
conflict checks with other clusters`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) != 1 {
				return cmd.Help()
			}

			var topo spec.Specification
			if opt.existCluster { // check for existing cluster
				clusterName := args[0]

				exist, err := tidbSpec.Exist(clusterName)
				if err != nil {
					return perrs.AddStack(err)
				}

				if !exist {
					return perrs.Errorf("cluster %s does not exist", clusterName)
				}

				metadata, err := spec.ClusterMetadata(clusterName)
				if err != nil {
					return err
				}
				opt.user = metadata.User
				opt.identityFile = tidbSpec.Path(clusterName, "ssh", "id_rsa")

				topo = *metadata.Topology
			} else { // check before cluster is deployed
				if err := clusterutil.ParseTopologyYaml(args[0], &topo); err != nil {
					return err
				}

				clusterList, err := tidbSpec.GetAllClusters()
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

			// natvie ssh has it's own logic to find the default identity_file
			if gOpt.SSHType == executor.SSHTypeSystem && !utils.IsFlagSetByUser(cmd.Flags(), "identity_file") {
				opt.identityFile = ""
			}

			var sshConnProps *cliutil.SSHConnectionProps = &cliutil.SSHConnectionProps{}
			if gOpt.SSHType != executor.SSHTypeNone {
				var err error
				if sshConnProps, err = cliutil.ReadIdentityFileOrPassword(opt.identityFile, opt.usePassword); err != nil {
					return err
				}
			}

			return checkSystemInfo(sshConnProps, &topo, &opt)
		},
	}

	cmd.Flags().StringVarP(&opt.user, "user", "u", utils.CurrentUser(), "The user name to login via SSH. The user must has root (or sudo) privilege.")
	cmd.Flags().StringVarP(&opt.identityFile, "identity_file", "i", opt.identityFile, "The path of the SSH identity file. If specified, public key authentication will be used.")
	cmd.Flags().BoolVarP(&opt.usePassword, "password", "p", false, "Use password of target hosts. If specified, password authentication will be used.")

	cmd.Flags().BoolVar(&opt.opr.EnableCPU, "enable-cpu", false, "Enable CPU thread count check")
	cmd.Flags().BoolVar(&opt.opr.EnableMem, "enable-mem", false, "Enable memory size check")
	cmd.Flags().BoolVar(&opt.opr.EnableDisk, "enable-disk", false, "Enable disk IO (fio) check")
	cmd.Flags().BoolVar(&opt.applyFix, "apply", false, "Try to fix failed checks")
	cmd.Flags().BoolVar(&opt.existCluster, "cluster", false, "Check existing cluster, the input is a cluster name.")

	return cmd
}

// checkSystemInfo performs series of checks and tests of the deploy server
func checkSystemInfo(s *cliutil.SSHConnectionProps, topo *spec.Specification, opt *checkOptions) error {
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
	topo.IterInstance(func(inst spec.Instance) {
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
					gOpt.SSHTimeout,
					gOpt.SSHType,
					topo.GlobalOptions.SSHType,
				).
				Mkdir(opt.user, inst.GetHost(), filepath.Join(task.CheckToolsPathDir, "bin")).
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
					false,
				).
				BuildAsStep(fmt.Sprintf("  - Getting system info of %s:%d", inst.GetHost(), inst.GetSSHPort()))
			collectTasks = append(collectTasks, t1)

			// build checking tasks
			t2 := task.NewBuilder().
				// check for general system info
				CheckSys(
					inst.GetHost(),
					"",
					task.CheckTypeSystemInfo,
					topo,
					opt.opr,
				).
				CheckSys(
					inst.GetHost(),
					"",
					task.CheckTypePartitions,
					topo,
					opt.opr,
				).
				// check for listening port
				Shell(
					inst.GetHost(),
					"ss -lnt",
					false,
				).
				CheckSys(
					inst.GetHost(),
					"",
					task.CheckTypePort,
					topo,
					opt.opr,
				).
				// check for system limits
				Shell(
					inst.GetHost(),
					"cat /etc/security/limits.conf",
					false,
				).
				CheckSys(
					inst.GetHost(),
					"",
					task.CheckTypeSystemLimits,
					topo,
					opt.opr,
				).
				// check for kernel params
				Shell(
					inst.GetHost(),
					"sysctl -a",
					true,
				).
				CheckSys(
					inst.GetHost(),
					"",
					task.CheckTypeSystemConfig,
					topo,
					opt.opr,
				).
				// check for needed system service
				CheckSys(
					inst.GetHost(),
					"",
					task.CheckTypeService,
					topo,
					opt.opr,
				).
				// check for needed packages
				CheckSys(
					inst.GetHost(),
					"",
					task.CheckTypePackage,
					topo,
					opt.opr,
				)

			// if the data dir set in topology is relative, and the home dir of deploy user
			// and the user run the check command is on different partitions, the disk detection
			// may be using incorrect partition for validations.
			for _, dataDir := range clusterutil.MultiDirAbs(opt.user, inst.DataDir()) {
				// build checking tasks
				t2 = t2.
					CheckSys(
						inst.GetHost(),
						dataDir,
						task.CheckTypeFIO,
						topo,
						opt.opr,
					)
			}
			checkSysTasks = append(
				checkSysTasks,
				t2.BuildAsStep(fmt.Sprintf("  - Checking node %s", inst.GetHost())),
			)

			t3 := task.NewBuilder().
				RootSSH(
					inst.GetHost(),
					inst.GetSSHPort(),
					opt.user,
					s.Password,
					s.IdentityFile,
					s.IdentityFilePassphrase,
					gOpt.SSHTimeout,
					gOpt.SSHType,
					topo.GlobalOptions.SSHType,
				).
				Rmdir(inst.GetHost(), task.CheckToolsPathDir).
				BuildAsStep(fmt.Sprintf("  - Cleanup check files on %s:%d", inst.GetHost(), inst.GetSSHPort()))
			cleanTasks = append(cleanTasks, t3)
		}
	})

	t := task.NewBuilder().
		ParallelStep("+ Download necessary tools", false, downloadTasks...).
		ParallelStep("+ Collect basic system information", false, collectTasks...).
		ParallelStep("+ Check system requirements", false, checkSysTasks...).
		ParallelStep("+ Cleanup check files", false, cleanTasks...).
		Build()

	ctx := task.NewContext()
	if err := t.Execute(ctx); err != nil {
		if errorx.Cast(err) != nil {
			// FIXME: Map possible task errors and give suggestions.
			return err
		}
		return perrs.Trace(err)
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
				gOpt.SSHTimeout,
				gOpt.SSHType,
				topo.GlobalOptions.SSHType,
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
				"sed -i 's/^[[:blank:]]*SELINUX=enforcing/SELINUX=no/g' %s && %s",
				"/etc/selinux/config",
				"setenforce 0",
			),
			true)
		msg = fmt.Sprintf("will try to %s, reboot might be needed", color.HiBlueString("disable SELinux"))
	default:
		msg = fmt.Sprintf("%s, auto fixing not supported", res)
	}
	return msg, nil
}
