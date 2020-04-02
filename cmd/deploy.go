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

package cmd

import (
	"fmt"
	"io/ioutil"
	"os"
	"path"
	"path/filepath"
	"strings"

	"github.com/ScaleFT/sshkeys"
	"github.com/fatih/color"
	"github.com/joomcode/errorx"
	"github.com/pingcap-incubator/tiops/pkg/bindversion"
	"github.com/pingcap-incubator/tiops/pkg/cliutil"
	"github.com/pingcap-incubator/tiops/pkg/errutil"
	"github.com/pingcap-incubator/tiops/pkg/executor"
	"github.com/pingcap-incubator/tiops/pkg/log"
	"github.com/pingcap-incubator/tiops/pkg/logger"
	"github.com/pingcap-incubator/tiops/pkg/meta"
	"github.com/pingcap-incubator/tiops/pkg/task"
	"github.com/pingcap-incubator/tiops/pkg/utils"
	"github.com/pingcap-incubator/tiup/pkg/repository"
	"github.com/pingcap-incubator/tiup/pkg/set"
	tiuputils "github.com/pingcap-incubator/tiup/pkg/utils"
	"github.com/pingcap/errors"
	"github.com/spf13/cobra"
	"golang.org/x/crypto/ssh"
)

var (
	errNSDeploy            = errNS.NewSubNamespace("deploy")
	errDeployNameDuplicate = errNSDeploy.NewType("name_dup", errutil.ErrTraitPreCheck)
)

type componentInfo struct {
	component string
	version   repository.Version
}

type deployOptions struct {
	user        string // username to login to the SSH server
	usePasswd   bool   // use password for authentication
	password    string // password of the user
	keyFile     string // path to the private key file
	passphrase  string // passphrase of the private key file
	skipConfirm bool   // skip the confirmation of topology
}

func newDeploy() *cobra.Command {
	opt := deployOptions{}
	cmd := &cobra.Command{
		Use:          "deploy <cluster-name> <version> <topology.yaml>",
		Short:        "Deploy a cluster for production",
		SilenceUsage: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			shouldContinue, err := cliutil.CheckCommandArgsAndMayPrintHelp(cmd, args, 3)
			if err != nil {
				return err
			}
			if !shouldContinue {
				return nil
			}

			if opt.usePasswd {
				// FIXME: We should prompt for password when necessary automatically.
				opt.password = cliutil.PromptForPassword("Password: ")
				fmt.Println("")
			}

			// https://github.com/appleboy/easyssh-proxy/blob/7594a28d719d7da8767c7b024043b261ea15796c/easyssh.go#L96
			// If this not a correct file path, it will ignore and just print a log to stdout.
			// So we check here first.
			if opt.keyFile != "" {
				err = checkKey(opt.keyFile, opt.passphrase)
				if err != nil {
					return err
				}
			}

			if len(opt.keyFile) == 0 && !opt.usePasswd {
				// FIXME: We should lookup identity key automatically.
				return executor.ErrSSHRequireCredential.
					New("Identity file and password is unspecified").
					WithProperty(cliutil.SuggestionFromTemplate(`
You should specify either SSH identity file or password.

To SSH connect using identity file:
  {{ColorCommand}}{{OsArgs}} -i <file>{{ColorReset}}

To SSH connect using password:
  {{ColorCommand}}{{OsArgs}} --password{{ColorReset}}

`, nil))
			}

			logger.EnableAuditLog()
			return deploy(args[0], args[1], args[2], opt)
		},
	}

	cmd.Flags().StringVar(&opt.user, "user", "root", "Specify the system user name")
	cmd.Flags().BoolVar(&opt.usePasswd, "password", false, "Specify the password of system user")
	cmd.Flags().StringVarP(&opt.keyFile, "identity_file", "i", "", "Specify the path of the SSH identity file")
	// FIXME: We should prompt for passphrase automatically
	cmd.Flags().StringVar(&opt.passphrase, "passphrase", "", "Specify the passphrase of the SSH identity file")
	cmd.Flags().BoolVarP(&opt.skipConfirm, "yes", "y", false, "Skip the confirmation of topology")

	return cmd
}

func fixDir(topo *meta.Specification) func(string) string {
	return func(dir string) string {
		if !strings.HasPrefix(dir, "/") {
			return path.Join("/home/", topo.GlobalOptions.User, dir)
		}
		return dir
	}
}

func checkClusterDirConflict(topo *meta.Specification) error {
	dirs := []string{}
	existDirs := []string{}

	clusterDir := meta.ProfilePath(meta.TiOpsClusterDir)
	fileInfos, err := ioutil.ReadDir(clusterDir)
	if err != nil && !os.IsNotExist(err) {
		return err
	}

	for _, fi := range fileInfos {
		if tiuputils.IsNotExist(meta.ClusterPath(fi.Name(), meta.MetaFileName)) {
			continue
		}
		metadata, err := meta.ClusterMetadata(fi.Name())
		if err != nil {
			return errors.Trace(err)
		}

		f := fixDir(metadata.Topology)
		metadata.Topology.IterInstance(func(inst meta.Instance) {
			existDirs = append(existDirs,
				path.Join(inst.GetHost(), f(inst.DeployDir())),
				path.Join(inst.GetHost(), f(inst.DataDir())),
				path.Join(inst.GetHost(), f(inst.LogDir())),
				path.Join(inst.GetHost(), f(metadata.Topology.MonitoredOptions.DeployDir)),
				path.Join(inst.GetHost(), f(metadata.Topology.MonitoredOptions.DataDir)),
				path.Join(inst.GetHost(), f(metadata.Topology.MonitoredOptions.LogDir)),
			)
		})
	}
	f := fixDir(topo)
	topo.IterInstance(func(inst meta.Instance) {
		dirs = append(dirs,
			path.Join(inst.GetHost(), f(inst.DeployDir())),
			path.Join(inst.GetHost(), f(inst.DataDir())),
			path.Join(inst.GetHost(), f(inst.LogDir())),
			path.Join(inst.GetHost(), f(topo.MonitoredOptions.DeployDir)),
			path.Join(inst.GetHost(), f(topo.MonitoredOptions.DataDir)),
			path.Join(inst.GetHost(), f(topo.MonitoredOptions.LogDir)),
		)
	})

	for _, d1 := range dirs {
		for _, d2 := range existDirs {
			if !strings.HasSuffix(d1, "/") {
				d1 += "/"
			}
			if !strings.HasSuffix(d2, "/") {
				d2 += "/"
			}
			if strings.HasPrefix(d1, d2) || strings.HasPrefix(d2, d1) {
				return errors.Errorf("directory %s already has been taken by other cluster", d1)
			}
		}
	}

	return nil
}

func confirmTopology(clusterName, version string, topo *meta.Specification) error {
	log.Infof("Please confirm your topology:")

	cyan := color.New(color.FgCyan, color.Bold)
	fmt.Printf("TiDB Cluster: %s\n", cyan.Sprint(clusterName))
	fmt.Printf("TiDB Version: %s\n", cyan.Sprint(version))

	clusterTable := [][]string{
		// Header
		{"Type", "Host", "Ports", "Directories"},
	}

	topo.IterInstance(func(instance meta.Instance) {
		clusterTable = append(clusterTable, []string{
			instance.ComponentName(),
			instance.GetHost(),
			utils.JoinInt(instance.UsedPorts(), "/"),
			strings.Join(instance.UsedDirs(), ","),
		})
	})

	cliutil.PrintTable(clusterTable, true)

	log.Warnf("Attention:")
	log.Warnf("    1. If the topology is not what you expected, check your yaml file.")
	log.Warnf("    1. Please confirm there is no port/directory conflicts in same host.")

	return cliutil.PromptForConfirmOrAbortError("Do you want to continue? [y/N]: ")
}

func deploy(clusterName, version, topoFile string, opt deployOptions) error {
	if err := utils.ValidateClusterNameOrError(clusterName); err != nil {
		return err
	}
	if tiuputils.IsExist(meta.ClusterPath(clusterName, meta.MetaFileName)) {
		// FIXME: When change to use args, the suggestion text need to be updated.
		return errDeployNameDuplicate.
			New("Cluster name '%s' is duplicated", clusterName).
			WithProperty(cliutil.SuggestionFromFormat("Please specify another cluster name"))
	}

	var topo meta.TopologySpecification
	if err := utils.ParseTopologyYaml(topoFile, &topo); err != nil {
		return err
	}

	// TODO: check port conflict cross cluster
	if err := checkClusterDirConflict(&topo); err != nil {
		return err
	}

	if !opt.skipConfirm {
		if err := confirmTopology(clusterName, version, &topo); err != nil {
			return err
		}
	}

	if err := os.MkdirAll(meta.ClusterPath(clusterName), 0755); err != nil {
		return errorx.InitializationFailed.
			Wrap(err, "Failed to create cluster metadata directory '%s'", meta.ClusterPath(clusterName)).
			WithProperty(errutil.ErrPropSuggestion, "Please check file system permissions and try again.")
	}

	var (
		envInitTasks      []task.Task // tasks which are used to initialize environment
		downloadCompTasks []task.Task // tasks which are used to download components
		deployCompTasks   []task.Task // tasks which are used to copy components to remote host
	)

	// Initialize environment
	uniqueHosts := set.NewStringSet()
	topo.IterInstance(func(inst meta.Instance) {
		if !uniqueHosts.Exist(inst.GetHost()) {
			uniqueHosts.Insert(inst.GetHost())
			t := task.NewBuilder().
				RootSSH(inst.GetHost(), inst.GetSSHPort(), opt.user, opt.password, opt.keyFile, opt.passphrase).
				EnvInit(inst.GetHost(), topo.GlobalOptions.User).
				UserSSH(inst.GetHost(), topo.GlobalOptions.User).
				Build()
			envInitTasks = append(envInitTasks, t)
		}
	})

	// Download missing component
	downloadCompTasks = buildDownloadCompTasks(version, &topo)

	// Deploy components to remote
	topo.IterInstance(func(inst meta.Instance) {
		version := bindversion.ComponentVersion(inst.ComponentName(), version)
		deployDir := inst.DeployDir()
		if !strings.HasPrefix(deployDir, "/") {
			deployDir = filepath.Join("/home/", topo.GlobalOptions.User, deployDir)
		}
		// data dir would be empty for components which don't need it
		dataDir := inst.DataDir()
		if dataDir != "" && !strings.HasPrefix(dataDir, "/") {
			dataDir = filepath.Join("/home/", topo.GlobalOptions.User, dataDir)
		}
		// log dir will always be with values, but might not used by the component
		logDir := inst.LogDir()
		if !strings.HasPrefix(logDir, "/") {
			logDir = filepath.Join("/home/", topo.GlobalOptions.User, logDir)
		}
		// Deploy component
		t := task.NewBuilder().
			Mkdir(topo.GlobalOptions.User, inst.GetHost(),
				filepath.Join(deployDir, "bin"),
				filepath.Join(deployDir, "conf"),
				filepath.Join(deployDir, "scripts"),
				dataDir,
				logDir).
			CopyComponent(inst.ComponentName(), version, inst.GetHost(), deployDir).
			InitConfig(
				clusterName,
				inst,
				topo.GlobalOptions.User,
				meta.DirPaths{
					Deploy: deployDir,
					Data:   dataDir,
					Log:    logDir,
					Cache:  meta.ClusterPath(clusterName, "config"),
				},
			).
			Build()
		deployCompTasks = append(deployCompTasks, t)
	})

	// Deploy monitor relevant components to remote
	dlTasks, dpTasks := buildMonitoredDeployTask(clusterName, uniqueHosts, topo.GlobalOptions, topo.MonitoredOptions, version)
	downloadCompTasks = append(downloadCompTasks, dlTasks...)
	deployCompTasks = append(deployCompTasks, dpTasks...)

	t := task.NewBuilder().
		SSHKeyGen(meta.ClusterPath(clusterName, "ssh", "id_rsa")).
		Parallel(downloadCompTasks...).
		Parallel(envInitTasks...).
		Parallel(deployCompTasks...).
		Build()

	if err := t.Execute(task.NewContext()); err != nil {
		if errorx.Cast(err) != nil {
			// FIXME: Map possible task errors and give suggestions.
			return err
		}
		return errors.Trace(err)
	}

	return meta.SaveClusterMeta(clusterName, &meta.ClusterMeta{
		User:     topo.GlobalOptions.User,
		Version:  version,
		Topology: &topo,
	})
}

func buildDownloadCompTasks(version string, topo *meta.Specification) []task.Task {
	var tasks []task.Task
	topo.IterComponent(func(comp meta.Component) {
		if len(comp.Instances()) < 1 {
			return
		}
		version := bindversion.ComponentVersion(comp.Name(), version)
		t := task.NewBuilder().Download(comp.Name(), version).Build()
		tasks = append(tasks, t)
	})
	return tasks
}

func buildMonitoredDeployTask(
	clusterName string,
	uniqueHosts set.StringSet,
	globalOptions meta.GlobalOptions,
	monitoredOptions meta.MonitoredOptions,
	version string) (downloadCompTasks, deployCompTasks []task.Task) {
	for _, comp := range []string{meta.ComponentNodeExporter, meta.ComponentBlackboxExporter} {
		version := bindversion.ComponentVersion(comp, version)
		t := task.NewBuilder().
			Download(comp, version).
			Build()
		downloadCompTasks = append(downloadCompTasks, t)

		for host := range uniqueHosts {
			deployDir := monitoredOptions.DeployDir
			if !strings.HasPrefix(deployDir, "/") {
				deployDir = filepath.Join("/home/", globalOptions.User, deployDir)
			}
			// data dir would be empty for components which don't need it
			dataDir := monitoredOptions.DataDir
			if dataDir != "" && !strings.HasPrefix(dataDir, "/") {
				dataDir = filepath.Join("/home/", globalOptions.User, dataDir)
			}
			logDir := monitoredOptions.LogDir
			if !strings.HasPrefix(logDir, "/") {
				logDir = filepath.Join("/home/", globalOptions.User, logDir)
			}

			// Deploy component
			t := task.NewBuilder().
				UserSSH(host, globalOptions.User).
				Mkdir(globalOptions.User, host,
					filepath.Join(deployDir, "bin"),
					filepath.Join(deployDir, "conf"),
					filepath.Join(deployDir, "scripts"),
					dataDir,
					logDir).
				CopyComponent(comp, version, host, deployDir).
				MonitoredConfig(
					clusterName,
					comp,
					host,
					monitoredOptions,
					globalOptions.User,
					meta.DirPaths{
						Deploy: deployDir,
						Data:   dataDir,
						Log:    logDir,
						Cache:  meta.ClusterPath(clusterName, "config"),
					},
				).Build()
			deployCompTasks = append(deployCompTasks, t)
		}
	}
	return
}

func checkKey(keypath string, passphrase string) error {
	var err error
	buf, err := ioutil.ReadFile(keypath)
	if err != nil {
		return err
	}

	if passphrase != "" {
		_, err = sshkeys.ParseEncryptedPrivateKey(buf, []byte(passphrase))
	} else {
		_, err = ssh.ParsePrivateKey(buf)
	}

	return err
}
