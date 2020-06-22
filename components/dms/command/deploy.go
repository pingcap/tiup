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

/*
import (
	"context"
	"fmt"
	"io/ioutil"
	"os"
	"path"
	"path/filepath"
	"strings"

	"github.com/pingcap/tiup/pkg/repository/v0manifest"

	"github.com/fatih/color"
	"github.com/joomcode/errorx"
	"github.com/pingcap/errors"
	"github.com/pingcap/tiup/pkg/cliutil"
	"github.com/pingcap/tiup/pkg/cliutil/prepare"
	"github.com/pingcap/tiup/pkg/cluster/clusterutil"
	"github.com/pingcap/tiup/pkg/cluster/meta"
	operator "github.com/pingcap/tiup/pkg/cluster/operation"
	"github.com/pingcap/tiup/pkg/cluster/report"
	"github.com/pingcap/tiup/pkg/cluster/task"
	"github.com/pingcap/tiup/pkg/errutil"
	"github.com/pingcap/tiup/pkg/logger"
	"github.com/pingcap/tiup/pkg/logger/log"
	"github.com/pingcap/tiup/pkg/set"
	"github.com/pingcap/tiup/pkg/telemetry"
	tiuputils "github.com/pingcap/tiup/pkg/utils"
	"github.com/spf13/cobra"
)

var (
	teleReport    *telemetry.Report
	clusterReport *telemetry.ClusterReport
	teleNodeInfos []*telemetry.NodeInfo
	teleTopology  string
)

var (
	errNSDeploy            = errNS.NewSubNamespace("deploy")
	errDeployNameDuplicate = errNSDeploy.NewType("name_dup", errutil.ErrTraitPreCheck)
)

type (
	componentInfo struct {
		component string
		version   v0manifest.Version
	}

	deployOptions struct {
		user         string // username to login to the SSH server
		identityFile string // path to the private key file
		usePassword  bool   // use password instead of identity file for ssh connection
	}

	hostInfo struct {
		ssh  int    // ssh port of host
		os   string // operating system
		arch string // cpu architecture
		// vendor string
	}
)

func newDeploy() *cobra.Command {
	opt := deployOptions{
		identityFile: path.Join(tiuputils.UserHome(), ".ssh", "id_rsa"),
	}
	cmd := &cobra.Command{
		Use:          "deploy <cluster-name> <version> <topology.yaml>",
		Short:        "Deploy a DM cluster for production",
		Long:         "Deploy a DM cluster for production. SSH connection will be used to deploy files, as well as creating system users for running the service.",
		SilenceUsage: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			shouldContinue, err := cliutil.CheckCommandArgsAndMayPrintHelp(cmd, args, 3)
			if err != nil {
				return err
			}
			if !shouldContinue {
				return nil
			}

			logger.EnableAuditLog()
			return deploy(args[0], args[1], args[2], opt)
		},
	}

	cmd.Flags().StringVar(&opt.user, "user", tiuputils.CurrentUser(), "The user name to login via SSH. The user must has root (or sudo) privilege.")
	cmd.Flags().StringVarP(&opt.identityFile, "identity_file", "i", opt.identityFile, "The path of the SSH identity file. If specified, public key authentication will be used.")
	cmd.Flags().BoolVarP(&opt.usePassword, "password", "p", false, "Use password of target hosts. If specified, password authentication will be used.")

	return cmd
}

func confirmTopology(clusterName, version string, topo *meta.DMSTopologySpecification, patchedRoles set.StringSet) error {
	log.Infof("Please confirm your topology:")

	cyan := color.New(color.FgCyan, color.Bold)
	fmt.Printf("DM Cluster: %s\n", cyan.Sprint(clusterName))
	fmt.Printf("DM Version: %s\n", cyan.Sprint(version))

	clusterTable := [][]string{
		// Header
		{"Type", "Host", "Ports", "OS/Arch", "Directories"},
	}

	topo.IterInstance(func(instance meta.Instance) {
		comp := instance.ComponentName()
		if patchedRoles.Exist(comp) {
			comp = comp + " (patched)"
		}
		clusterTable = append(clusterTable, []string{
			comp,
			instance.GetHost(),
			clusterutil.JoinInt(instance.UsedPorts(), "/"),
			cliutil.OsArch(instance.OS(), instance.Arch()),
			strings.Join(instance.UsedDirs(), ","),
		})
	})

	cliutil.PrintTable(clusterTable, true)

	log.Warnf("Attention:")
	log.Warnf("    1. If the topology is not what you expected, check your yaml file.")
	log.Warnf("    2. Please confirm there is no port/directory conflicts in same host.")
	if len(patchedRoles) != 0 {
		log.Errorf("    3. The component marked as `patched` has been replaced by previours patch command.")
	}

	return cliutil.PromptForConfirmOrAbortError("Do you want to continue? [y/N]: ")
}

func deploy(clusterName, clusterVersion, topoFile string, opt deployOptions) error {
	if err := clusterutil.ValidateClusterNameOrError(clusterName); err != nil {
		return err
	}
	if tiuputils.IsExist(meta.ClusterPath(clusterName, meta.MetaFileName)) {
		// FIXME: When change to use args, the suggestion text need to be updated.
		return errDeployNameDuplicate.
			New("Cluster name '%s' is duplicated", clusterName).
			WithProperty(cliutil.SuggestionFromFormat("Please specify another cluster name"))
	}

	var topo meta.DMSTopologySpecification
	if err := clusterutil.ParseTopologyYaml(topoFile, &topo); err != nil {
		return err
	}

	if data, err := ioutil.ReadFile(topoFile); err == nil {
		teleTopology = string(data)
	}

	if err := prepare.CheckClusterPortConflict(clusterName, &topo); err != nil {
		return err
	}
	if err := prepare.CheckClusterDirConflict(clusterName, &topo); err != nil {
		return err
	}

	if !skipConfirm {
		if err := confirmTopology(clusterName, clusterVersion, &topo, set.NewStringSet()); err != nil {
			return err
		}
	}

	sshConnProps, err := cliutil.ReadIdentityFileOrPassword(opt.identityFile, opt.usePassword)
	if err != nil {
		return err
	}

	if err := os.MkdirAll(meta.ClusterPath(clusterName), 0755); err != nil {
		return errorx.InitializationFailed.
			Wrap(err, "Failed to create cluster metadata directory '%s'", meta.ClusterPath(clusterName)).
			WithProperty(cliutil.SuggestionFromString("Please check file system permissions and try again."))
	}

	var (
		envInitTasks      []*task.StepDisplay // tasks which are used to initialize environment
		downloadCompTasks []*task.StepDisplay // tasks which are used to download components
		deployCompTasks   []*task.StepDisplay // tasks which are used to copy components to remote host
	)

	// Initialize environment
	uniqueHosts := make(map[string]hostInfo) // host -> ssh-port, os, arch
	globalOptions := topo.GlobalOptions
	topo.IterInstance(func(inst meta.Instance) {
		if _, found := uniqueHosts[inst.GetHost()]; !found {
			uniqueHosts[inst.GetHost()] = hostInfo{
				ssh:  inst.GetSSHPort(),
				os:   inst.OS(),
				arch: inst.Arch(),
			}
			var dirs []string
			for _, dir := range []string{globalOptions.DeployDir, globalOptions.LogDir} {
				if dir == "" {
					continue
				}
				dirs = append(dirs, clusterutil.Abs(globalOptions.User, dir))
			}
			// the default, relative path of data dir is under deploy dir
			if strings.HasPrefix(globalOptions.DataDir, "/") {
				dirs = append(dirs, globalOptions.DataDir)
			}
			t := task.NewBuilder().
				RootSSH(
					inst.GetHost(),
					inst.GetSSHPort(),
					opt.user,
					sshConnProps.Password,
					sshConnProps.IdentityFile,
					sshConnProps.IdentityFilePassphrase,
					gOpt.SSHTimeout,
				).
				EnvInit(inst.GetHost(), globalOptions.User).
				Mkdir(globalOptions.User, inst.GetHost(), dirs...).
				BuildAsStep(fmt.Sprintf("  - Prepare %s:%d", inst.GetHost(), inst.GetSSHPort()))
			envInitTasks = append(envInitTasks, t)
		}
	})

	// Download missing component
	downloadCompTasks = prepare.BuildDownloadCompTasks(clusterVersion, &topo)

	// Deploy components to remote
	topo.IterInstance(func(inst meta.Instance) {
		version := meta.ComponentVersion(inst.ComponentName(), clusterVersion)
		deployDir := clusterutil.Abs(globalOptions.User, inst.DeployDir())
		// data dir would be empty for components which don't need it
		dataDirs := clusterutil.MultiDirAbs(globalOptions.User, inst.DataDir())
		// log dir will always be with values, but might not used by the component
		logDir := clusterutil.Abs(globalOptions.User, inst.LogDir())
		// Deploy component
		t := task.NewBuilder().
			UserSSH(inst.GetHost(), inst.GetSSHPort(), globalOptions.User, gOpt.SSHTimeout).
			Mkdir(globalOptions.User, inst.GetHost(),
				deployDir, logDir,
				filepath.Join(deployDir, "bin"),
				filepath.Join(deployDir, "conf"),
				filepath.Join(deployDir, "scripts")).
			Mkdir(globalOptions.User, inst.GetHost(), dataDirs...).
			CopyComponent(
				inst.ComponentName(),
				inst.OS(),
				inst.Arch(),
				version,
				inst.GetHost(),
				deployDir,
			).
			InitConfig(
				clusterName,
				clusterVersion,
				inst,
				globalOptions.User,
				false,
				meta.DirPaths{
					Deploy: deployDir,
					Data:   dataDirs,
					Log:    logDir,
					Cache:  meta.ClusterPath(clusterName, meta.TempConfigPath),
				},
			).
			BuildAsStep(fmt.Sprintf("  - Copy %s -> %s", inst.ComponentName(), inst.GetHost()))
		deployCompTasks = append(deployCompTasks, t)
	})

	nodeInfoTask := task.NewBuilder().Func("Check status", func(ctx *task.Context) error {
		var err error
		teleNodeInfos, err = operator.GetNodeInfo(context.Background(), ctx, &topo)
		_ = err
		// intend to never return error
		return nil
	}).BuildAsStep("Check status").SetHidden(true)
	if report.Enable() {
		deployCompTasks = append(deployCompTasks, nodeInfoTask)
	}

	builder := task.NewBuilder().
		Step("+ Generate SSH keys",
			task.NewBuilder().SSHKeyGen(meta.ClusterPath(clusterName, "ssh", "id_rsa")).Build()).
		ParallelStep("+ Download DM components", downloadCompTasks...).
		ParallelStep("+ Initialize target host environments", envInitTasks...).
		ParallelStep("+ Copy files", deployCompTasks...)

	if report.Enable() {
		builder.ParallelStep("+ Check status", nodeInfoTask)
	}

	t := builder.Build()

	if err := t.Execute(task.NewContext()); err != nil {
		if errorx.Cast(err) != nil {
			// FIXME: Map possible task errors and give suggestions.
			return err
		}
		return errors.Trace(err)
	}

	err = meta.SaveDMMeta(clusterName, &meta.DMMeta{
		User:     globalOptions.User,
		Version:  clusterVersion,
		Topology: &topo,
	})
	if err != nil {
		return errors.Trace(err)
	}

	hint := color.New(color.Bold).Sprintf("tiup dm start %s", clusterName)
	log.Infof("Deployed cluster `%s` successfully, you can start the cluster via `%s`", clusterName, hint)
	return nil
}
*/
