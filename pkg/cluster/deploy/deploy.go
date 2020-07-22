package deploy

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"sort"
	"strings"

	"github.com/fatih/color"
	"github.com/joomcode/errorx"
	perrs "github.com/pingcap/errors"
	"github.com/pingcap/tiup/pkg/cliutil"
	"github.com/pingcap/tiup/pkg/cliutil/prepare"
	"github.com/pingcap/tiup/pkg/cluster/clusterutil"
	operator "github.com/pingcap/tiup/pkg/cluster/operation"
	"github.com/pingcap/tiup/pkg/cluster/spec"
	"github.com/pingcap/tiup/pkg/cluster/task"
	"github.com/pingcap/tiup/pkg/errutil"
	"github.com/pingcap/tiup/pkg/logger/log"
	"github.com/pingcap/tiup/pkg/meta"
	"github.com/pingcap/tiup/pkg/set"
	"github.com/pingcap/tiup/pkg/utils"
	"github.com/pingcap/tiup/pkg/version"
	"golang.org/x/mod/semver"
	"gopkg.in/yaml.v2"
)

//revive:disable:exported

var (
	errNSDeploy            = errorx.NewNamespace("deploy")
	errDeployNameDuplicate = errNSDeploy.NewType("name_dup", errutil.ErrTraitPreCheck)
)

// Deployer to deploy a cluster.
type Deployer struct {
	sysName     string
	specManager *spec.SpecManager
	newMeta     func() spec.Metadata
}

// NewDeployer create a Deployer.
func NewDeployer(sysName string, specManager *spec.SpecManager, newMeta func() spec.Metadata) *Deployer {
	return &Deployer{
		sysName:     sysName,
		specManager: specManager,
		newMeta:     newMeta,
	}
}

// StartCluster start the cluster with specified name.
func (d *Deployer) StartCluster(name string, options operator.Options, fn ...func(b *task.Builder, metadata spec.Metadata)) error {
	log.Infof("Starting cluster %s...", name)

	metadata, err := d.meta(name)
	if err != nil {
		return perrs.AddStack(err)
	}

	topo := metadata.GetTopology()
	base := metadata.GetBaseMeta()

	b := task.NewBuilder().
		SSHKeySet(
			d.specManager.Path(name, "ssh", "id_rsa"),
			d.specManager.Path(name, "ssh", "id_rsa.pub")).
		ClusterSSH(topo, base.User, options.SSHTimeout).
		Serial(task.NewFunc("StartCluster", func(ctx *task.Context) error {
			return operator.Start(ctx, topo, options)
		}))

	if len(fn) > 0 {
		if len(fn) != 1 {
			panic("wrong fn param")
		}
		fn[0](b, metadata)
	}

	t := b.Build()

	if err := t.Execute(task.NewContext()); err != nil {
		if errorx.Cast(err) != nil {
			// FIXME: Map possible task errors and give suggestions.
			return err
		}
		return perrs.Trace(err)
	}

	log.Infof("Started cluster `%s` successfully", name)
	return nil
}

// StopCluster stop the cluster.
func (d *Deployer) StopCluster(clusterName string, options operator.Options) error {
	metadata, err := d.meta(clusterName)
	if err != nil {
		return perrs.AddStack(err)
	}

	topo := metadata.GetTopology()
	base := metadata.GetBaseMeta()

	t := task.NewBuilder().
		SSHKeySet(
			d.specManager.Path(clusterName, "ssh", "id_rsa"),
			d.specManager.Path(clusterName, "ssh", "id_rsa.pub")).
		ClusterSSH(metadata.GetTopology(), base.User, options.SSHTimeout).
		Serial(task.NewFunc("StopCluster", func(ctx *task.Context) error {
			return operator.Stop(ctx, topo, options)
		})).
		Build()

	if err := t.Execute(task.NewContext()); err != nil {
		if errorx.Cast(err) != nil {
			// FIXME: Map possible task errors and give suggestions.
			return err
		}
		return perrs.Trace(err)
	}

	log.Infof("Stopped cluster `%s` successfully", clusterName)
	return nil
}

// RestartCluster restart the cluster.
func (d *Deployer) RestartCluster(clusterName string, options operator.Options) error {
	metadata, err := d.meta(clusterName)
	if err != nil {
		return perrs.AddStack(err)
	}

	topo := metadata.GetTopology()
	base := metadata.GetBaseMeta()

	t := task.NewBuilder().
		SSHKeySet(
			d.specManager.Path(clusterName, "ssh", "id_rsa"),
			d.specManager.Path(clusterName, "ssh", "id_rsa.pub")).
		ClusterSSH(topo, base.User, options.SSHTimeout).
		Serial(task.NewFunc("RestartCluster", func(ctx *task.Context) error {
			return operator.Restart(ctx, topo, options)
		})).
		Build()

	if err := t.Execute(task.NewContext()); err != nil {
		if errorx.Cast(err) != nil {
			// FIXME: Map possible task errors and give suggestions.
			return err
		}
		return perrs.Trace(err)
	}

	log.Infof("Restarted cluster `%s` successfully", clusterName)
	return nil
}

// ListCluster list the clusters.
func (d *Deployer) ListCluster() error {
	names, err := d.specManager.List()
	if err != nil {
		return perrs.AddStack(err)
	}

	clusterTable := [][]string{
		// Header
		{"Name", "User", "Version", "Path", "PrivateKey"},
	}

	for _, name := range names {
		metadata, err := d.meta(name)
		if err != nil {
			return perrs.Trace(err)
		}

		base := metadata.GetBaseMeta()

		clusterTable = append(clusterTable, []string{
			name,
			base.User,
			base.Version,
			d.specManager.Path(name),
			d.specManager.Path(name, "ssh", "id_rsa"),
		})
	}

	cliutil.PrintTable(clusterTable, true)
	return nil
}

// DestroyCluster destroy the cluster.
func (d *Deployer) DestroyCluster(clusterName string, gOpt operator.Options, destroyOpt operator.Options, skipConfirm bool) error {
	metadata, err := d.meta(clusterName)
	if err != nil {
		return perrs.AddStack(err)
	}

	topo := metadata.GetTopology()
	base := metadata.GetBaseMeta()

	if !skipConfirm {
		if err := cliutil.PromptForConfirmOrAbortError(
			"This operation will destroy %s %s cluster %s and its data.\nDo you want to continue? [y/N]:",
			d.sysName,
			color.HiYellowString(base.Version),
			color.HiYellowString(clusterName)); err != nil {
			return err
		}
		log.Infof("Destroying cluster...")
	}

	t := task.NewBuilder().
		SSHKeySet(
			d.specManager.Path(clusterName, "ssh", "id_rsa"),
			d.specManager.Path(clusterName, "ssh", "id_rsa.pub")).
		ClusterSSH(topo, base.User, gOpt.SSHTimeout).
		Serial(task.NewFunc("StopCluster", func(ctx *task.Context) error {
			return operator.Stop(ctx, topo, operator.Options{})
		})).
		Serial(task.NewFunc("DestroyCluster", func(ctx *task.Context) error {
			return operator.Destroy(ctx, topo, destroyOpt)
		})).
		Build()

	if err := t.Execute(task.NewContext()); err != nil {
		if errorx.Cast(err) != nil {
			// FIXME: Map possible task errors and give suggestions.
			return err
		}
		return perrs.Trace(err)
	}

	if err := d.specManager.Remove(clusterName); err != nil {
		return perrs.Trace(err)
	}

	log.Infof("Destroyed cluster `%s` successfully", clusterName)
	return nil

}

// ExecOptions for exec shell command.
type ExecOptions struct {
	Command string
	Sudo    bool
}

// Exec shell command on host in the tidb cluster.
func (d *Deployer) Exec(clusterName string, opt ExecOptions, gOpt operator.Options) error {
	metadata, err := d.meta(clusterName)
	if err != nil {
		return perrs.AddStack(err)
	}

	topo := metadata.GetTopology()
	base := metadata.GetBaseMeta()

	filterRoles := set.NewStringSet(gOpt.Roles...)
	filterNodes := set.NewStringSet(gOpt.Nodes...)

	var shellTasks []task.Task
	uniqueHosts := map[string]int{} // host -> ssh-port
	topo.IterInstance(func(inst spec.Instance) {
		if _, found := uniqueHosts[inst.GetHost()]; !found {
			if len(gOpt.Roles) > 0 && !filterRoles.Exist(inst.Role()) {
				return
			}

			if len(gOpt.Nodes) > 0 && !filterNodes.Exist(inst.GetHost()) {
				return
			}

			uniqueHosts[inst.GetHost()] = inst.GetSSHPort()
		}
	})

	for host := range uniqueHosts {
		shellTasks = append(shellTasks,
			task.NewBuilder().
				Shell(host, opt.Command, opt.Sudo).
				Build())
	}

	t := task.NewBuilder().
		SSHKeySet(
			d.specManager.Path(clusterName, "ssh", "id_rsa"),
			d.specManager.Path(clusterName, "ssh", "id_rsa.pub")).
		ClusterSSH(topo, base.User, gOpt.SSHTimeout).
		Parallel(shellTasks...).
		Build()

	execCtx := task.NewContext()
	if err := t.Execute(execCtx); err != nil {
		if errorx.Cast(err) != nil {
			// FIXME: Map possible task errors and give suggestions.
			return err
		}
		return perrs.Trace(err)
	}

	// print outputs
	for host := range uniqueHosts {
		stdout, stderr, ok := execCtx.GetOutputs(host)
		if !ok {
			continue
		}
		log.Infof("Outputs of %s on %s:",
			color.CyanString(opt.Command),
			color.CyanString(host))
		if len(stdout) > 0 {
			log.Infof("%s:\n%s", color.GreenString("stdout"), stdout)
		}
		if len(stderr) > 0 {
			log.Infof("%s:\n%s", color.RedString("stderr"), stderr)
		}
	}

	return nil
}

// Display cluster meta and topology.
func (d *Deployer) Display(clusterName string, opt operator.Options) error {
	metadata, err := d.meta(clusterName)
	if err != nil {
		return perrs.AddStack(err)
	}

	topo := metadata.GetTopology()
	base := metadata.GetBaseMeta()

	// display cluster meta
	cyan := color.New(color.FgCyan, color.Bold)
	fmt.Printf("%s Cluster: %s\n", d.sysName, cyan.Sprint(clusterName))
	fmt.Printf("%s Version: %s\n", d.sysName, cyan.Sprint(base.Version))

	// display topology
	clusterTable := [][]string{
		// Header
		{"ID", "Role", "Host", "Ports", "OS/Arch", "Status", "Data Dir", "Deploy Dir"},
	}

	ctx := task.NewContext()
	err = ctx.SetSSHKeySet(d.specManager.Path(clusterName, "ssh", "id_rsa"),
		d.specManager.Path(clusterName, "ssh", "id_rsa.pub"))
	if err != nil {
		return perrs.AddStack(err)
	}

	err = ctx.SetClusterSSH(topo, base.User, opt.SSHTimeout)
	if err != nil {
		return perrs.AddStack(err)
	}

	filterRoles := set.NewStringSet(opt.Roles...)
	filterNodes := set.NewStringSet(opt.Nodes...)
	pdList := topo.BaseTopo().MasterList
	for _, comp := range topo.ComponentsByStartOrder() {
		for _, ins := range comp.Instances() {
			// apply role filter
			if len(filterRoles) > 0 && !filterRoles.Exist(ins.Role()) {
				continue
			}
			// apply node filter
			if len(filterNodes) > 0 && !filterNodes.Exist(ins.ID()) {
				continue
			}

			dataDir := "-"
			insDirs := ins.UsedDirs()
			deployDir := insDirs[0]
			if len(insDirs) > 1 {
				dataDir = insDirs[1]
			}

			status := ins.Status(pdList...)
			// Query the service status
			if status == "-" {
				e, found := ctx.GetExecutor(ins.GetHost())
				if found {
					active, _ := operator.GetServiceStatus(e, ins.ServiceName())
					if parts := strings.Split(strings.TrimSpace(active), " "); len(parts) > 2 {
						if parts[1] == "active" {
							status = "Up"
						} else {
							status = parts[1]
						}
					}
				}
			}
			clusterTable = append(clusterTable, []string{
				color.CyanString(ins.ID()),
				ins.Role(),
				ins.GetHost(),
				clusterutil.JoinInt(ins.UsedPorts(), "/"),
				cliutil.OsArch(ins.OS(), ins.Arch()),
				formatInstanceStatus(status),
				dataDir,
				deployDir,
			})

		}
	}

	// Sort by role,host,ports
	sort.Slice(clusterTable[1:], func(i, j int) bool {
		lhs, rhs := clusterTable[i+1], clusterTable[j+1]
		// column: 1 => role, 2 => host, 3 => ports
		for _, col := range []int{1, 2} {
			if lhs[col] != rhs[col] {
				return lhs[col] < rhs[col]
			}
		}
		return lhs[3] < rhs[3]
	})

	cliutil.PrintTable(clusterTable, true)

	return nil
}

// EditConfig let the user edit the config.
func (d *Deployer) EditConfig(clusterName string, skipConfirm bool) error {
	metadata, err := d.meta(clusterName)
	if err != nil {
		return perrs.AddStack(err)
	}

	topo := metadata.GetTopology()

	data, err := yaml.Marshal(topo)
	if err != nil {
		return perrs.AddStack(err)
	}

	newTopo, err := editTopo(topo, data, skipConfirm)
	if err != nil {
		return perrs.AddStack(err)
	}

	if newTopo == nil {
		return nil
	}

	log.Infof("Apply the change...")
	metadata.SetTopology(newTopo)
	err = d.specManager.SaveMeta(clusterName, metadata)
	if err != nil {
		return perrs.Annotate(err, "failed to save meta")
	}

	log.Infof("Apply change successfully, please use `%s reload %s [-N <nodes>] [-R <roles>]` to reload config.", cliutil.OsArgs0(), clusterName)
	return nil
}

// Reload the cluster.
func (d *Deployer) Reload(clusterName string, opt operator.Options) error {
	metadata, err := d.meta(clusterName)
	if err != nil {
		return perrs.AddStack(err)
	}

	topo := metadata.GetTopology()
	base := metadata.GetBaseMeta()

	var refreshConfigTasks []task.Task

	hasImported := false

	topo.IterInstance(func(inst spec.Instance) {
		deployDir := clusterutil.Abs(base.User, inst.DeployDir())
		// data dir would be empty for components which don't need it
		dataDirs := clusterutil.MultiDirAbs(base.User, inst.DataDir())
		// log dir will always be with values, but might not used by the component
		logDir := clusterutil.Abs(base.User, inst.LogDir())

		// Download and copy the latest component to remote if the cluster is imported from Ansible
		tb := task.NewBuilder().UserSSH(inst.GetHost(), inst.GetSSHPort(), base.User, opt.SSHTimeout)
		if inst.IsImported() {
			switch compName := inst.ComponentName(); compName {
			case spec.ComponentGrafana, spec.ComponentPrometheus, spec.ComponentAlertManager:
				version := spec.ComponentVersion(compName, base.Version)
				tb.Download(compName, inst.OS(), inst.Arch(), version).
					CopyComponent(compName, inst.OS(), inst.Arch(), version, "", inst.GetHost(), deployDir)
			}
			hasImported = true
		}

		// Refresh all configuration
		t := tb.InitConfig(clusterName,
			base.Version,
			d.specManager,
			inst, base.User,
			opt.IgnoreConfigCheck,
			meta.DirPaths{
				Deploy: deployDir,
				Data:   dataDirs,
				Log:    logDir,
				Cache:  d.specManager.Path(clusterName, spec.TempConfigPath),
			}).Build()
		refreshConfigTasks = append(refreshConfigTasks, t)
	})

	// handle dir scheme changes
	if hasImported {
		if err := spec.HandleImportPathMigration(clusterName); err != nil {
			return perrs.AddStack(err)
		}
	}

	t := task.NewBuilder().
		SSHKeySet(
			d.specManager.Path(clusterName, "ssh", "id_rsa"),
			d.specManager.Path(clusterName, "ssh", "id_rsa.pub")).
		ClusterSSH(topo, base.User, opt.SSHTimeout).
		Parallel(refreshConfigTasks...).
		Serial(task.NewFunc("UpgradeCluster", func(ctx *task.Context) error {
			return operator.Upgrade(ctx, topo, opt)
		})).
		Build()

	if err := t.Execute(task.NewContext()); err != nil {
		if errorx.Cast(err) != nil {
			// FIXME: Map possible task errors and give suggestions.
			return err
		}
		return perrs.Trace(err)
	}

	log.Infof("Reloaded cluster `%s` successfully", clusterName)

	return nil
}

// Upgrade the cluster.
func (d *Deployer) Upgrade(clusterName string, clusterVersion string, opt operator.Options) error {
	metadata, err := d.meta(clusterName)
	if err != nil {
		return perrs.AddStack(err)
	}

	topo := metadata.GetTopology()
	base := metadata.GetBaseMeta()

	var (
		downloadCompTasks []task.Task // tasks which are used to download components
		copyCompTasks     []task.Task // tasks which are used to copy components to remote host

		uniqueComps = map[string]struct{}{}
	)

	if err := versionCompare(base.Version, clusterVersion); err != nil {
		return err
	}

	hasImported := false
	for _, comp := range topo.ComponentsByUpdateOrder() {
		for _, inst := range comp.Instances() {
			version := spec.ComponentVersion(inst.ComponentName(), clusterVersion)
			if version == "" {
				return perrs.Errorf("unsupported component: %v", inst.ComponentName())
			}
			compInfo := componentInfo{
				component: inst.ComponentName(),
				version:   version,
			}

			// Download component from repository
			key := fmt.Sprintf("%s-%s-%s-%s", compInfo.component, compInfo.version, inst.OS(), inst.Arch())
			if _, found := uniqueComps[key]; !found {
				uniqueComps[key] = struct{}{}
				t := task.NewBuilder().
					Download(inst.ComponentName(), inst.OS(), inst.Arch(), version).
					Build()
				downloadCompTasks = append(downloadCompTasks, t)
			}

			deployDir := clusterutil.Abs(base.User, inst.DeployDir())
			// data dir would be empty for components which don't need it
			dataDirs := clusterutil.MultiDirAbs(base.User, inst.DataDir())
			// log dir will always be with values, but might not used by the component
			logDir := clusterutil.Abs(base.User, inst.LogDir())

			// Deploy component
			tb := task.NewBuilder()
			if inst.IsImported() {
				switch inst.ComponentName() {
				case spec.ComponentPrometheus, spec.ComponentGrafana, spec.ComponentAlertManager:
					tb.CopyComponent(
						inst.ComponentName(),
						inst.OS(),
						inst.Arch(),
						version,
						"", // use default srcPath
						inst.GetHost(),
						deployDir,
					)
				}
				hasImported = true
			}

			// backup files of the old version
			tb = tb.BackupComponent(inst.ComponentName(), base.Version, inst.GetHost(), deployDir)

			// copy dependency component if needed
			switch inst.ComponentName() {
			case spec.ComponentTiSpark:
				tb = tb.DeploySpark(inst, version, "" /* default srcPath */, deployDir)
			default:
				tb = tb.CopyComponent(
					inst.ComponentName(),
					inst.OS(),
					inst.Arch(),
					version,
					"", // use default srcPath
					inst.GetHost(),
					deployDir,
				)
			}

			tb.InitConfig(
				clusterName,
				clusterVersion,
				d.specManager,
				inst,
				base.User,
				opt.IgnoreConfigCheck,
				meta.DirPaths{
					Deploy: deployDir,
					Data:   dataDirs,
					Log:    logDir,
					Cache:  d.specManager.Path(clusterName, spec.TempConfigPath),
				},
			)
			copyCompTasks = append(copyCompTasks, tb.Build())
		}
	}

	// handle dir scheme changes
	if hasImported {
		if err := spec.HandleImportPathMigration(clusterName); err != nil {
			return err
		}
	}

	t := task.NewBuilder().
		SSHKeySet(
			d.specManager.Path(clusterName, "ssh", "id_rsa"),
			d.specManager.Path(clusterName, "ssh", "id_rsa.pub")).
		ClusterSSH(topo, base.User, opt.SSHTimeout).
		Parallel(downloadCompTasks...).
		Parallel(copyCompTasks...).
		Serial(task.NewFunc("UpgradeCluster", func(ctx *task.Context) error {
			return operator.Upgrade(ctx, topo, opt)
		})).
		Build()

	if err := t.Execute(task.NewContext()); err != nil {
		if errorx.Cast(err) != nil {
			// FIXME: Map possible task errors and give suggestions.
			return err
		}
		return perrs.Trace(err)
	}

	metadata.SetVersion(clusterVersion)

	if err := d.specManager.SaveMeta(clusterName, metadata); err != nil {
		return perrs.Trace(err)
	}

	if err := os.RemoveAll(d.specManager.Path(clusterName, "patch")); err != nil {
		return perrs.Trace(err)
	}

	log.Infof("Upgraded cluster `%s` successfully", clusterName)

	return nil
}

// Patch the cluster.
func (d *Deployer) Patch(clusterName string, packagePath string, opt operator.Options, overwrite bool) error {
	metadata, err := d.meta(clusterName)
	if err != nil {
		return perrs.AddStack(err)
	}

	topo := metadata.GetTopology()
	base := metadata.GetBaseMeta()

	if exist := utils.IsExist(packagePath); !exist {
		return perrs.New("specified package not exists")
	}

	insts, err := instancesToPatch(topo, opt)
	if err != nil {
		return err
	}
	if err := checkPackage(d.specManager, clusterName, insts[0].ComponentName(), insts[0].OS(), insts[0].Arch(), packagePath); err != nil {
		return err
	}

	var replacePackageTasks []task.Task
	for _, inst := range insts {
		deployDir := clusterutil.Abs(base.User, inst.DeployDir())
		tb := task.NewBuilder()
		tb.BackupComponent(inst.ComponentName(), base.Version, inst.GetHost(), deployDir).
			InstallPackage(packagePath, inst.GetHost(), deployDir)
		replacePackageTasks = append(replacePackageTasks, tb.Build())
	}

	t := task.NewBuilder().
		SSHKeySet(
			d.specManager.Path(clusterName, "ssh", "id_rsa"),
			d.specManager.Path(clusterName, "ssh", "id_rsa.pub")).
		ClusterSSH(topo, base.User, opt.SSHTimeout).
		Parallel(replacePackageTasks...).
		Serial(task.NewFunc("UpgradeCluster", func(ctx *task.Context) error {
			return operator.Upgrade(ctx, topo, opt)
		})).
		Build()

	if err := t.Execute(task.NewContext()); err != nil {
		if errorx.Cast(err) != nil {
			// FIXME: Map possible task errors and give suggestions.
			return err
		}
		return perrs.Trace(err)
	}

	if overwrite {
		if err := overwritePatch(d.specManager, clusterName, insts[0].ComponentName(), packagePath); err != nil {
			return err
		}
	}

	return nil
}

// ScaleOutOptions contains the options for scale out.
type ScaleOutOptions struct {
	User         string // username to login to the SSH server
	IdentityFile string // path to the private key file
	UsePassword  bool   // use password instead of identity file for ssh connection
}

// DeployOptions contains the options for scale out.
// TODO: merge ScaleOutOptions, should check config too when scale out.
type DeployOptions struct {
	User              string // username to login to the SSH server
	IdentityFile      string // path to the private key file
	UsePassword       bool   // use password instead of identity file for ssh connection
	IgnoreConfigCheck bool   // ignore config check result
}

// Deploy a cluster.
func (d *Deployer) Deploy(
	clusterName string,
	clusterVersion string,
	topoFile string,
	opt DeployOptions,
	afterDeploy func(b *task.Builder, newPart spec.Topology),
	skipConfirm bool,
	optTimeout int64,
	sshTimeout int64,
) error {
	if err := clusterutil.ValidateClusterNameOrError(clusterName); err != nil {
		return err
	}

	exist, err := d.specManager.Exist(clusterName)
	if err != nil {
		return perrs.AddStack(err)
	}

	if exist {
		// FIXME: When change to use args, the suggestion text need to be updated.
		return errDeployNameDuplicate.
			New("Cluster name '%s' is duplicated", clusterName).
			WithProperty(cliutil.SuggestionFromFormat("Please specify another cluster name"))
	}

	metadata := d.specManager.NewMetadata()
	topo := metadata.GetTopology()

	if err := clusterutil.ParseTopologyYaml(topoFile, &topo); err != nil {
		return err
	}

	base := topo.BaseTopo()

	if err := prepare.CheckClusterPortConflict(d.specManager, clusterName, topo); err != nil {
		return err
	}
	if err := prepare.CheckClusterDirConflict(d.specManager, clusterName, topo); err != nil {
		return err
	}

	if !skipConfirm {
		if err := d.confirmTopology(clusterName, clusterVersion, topo, set.NewStringSet()); err != nil {
			return err
		}
	}

	sshConnProps, err := cliutil.ReadIdentityFileOrPassword(opt.IdentityFile, opt.UsePassword)
	if err != nil {
		return err
	}

	if err := os.MkdirAll(d.specManager.Path(clusterName), 0755); err != nil {
		return errorx.InitializationFailed.
			Wrap(err, "Failed to create cluster metadata directory '%s'", d.specManager.Path(clusterName)).
			WithProperty(cliutil.SuggestionFromString("Please check file system permissions and try again."))
	}

	var (
		envInitTasks      []*task.StepDisplay // tasks which are used to initialize environment
		downloadCompTasks []*task.StepDisplay // tasks which are used to download components
		deployCompTasks   []*task.StepDisplay // tasks which are used to copy components to remote host
	)

	// Initialize environment
	uniqueHosts := make(map[string]hostInfo) // host -> ssh-port, os, arch
	globalOptions := base.GlobalOptions
	var iterErr error // error when itering over instances
	iterErr = nil
	topo.IterInstance(func(inst spec.Instance) {
		if _, found := uniqueHosts[inst.GetHost()]; !found {
			// check for "imported" parameter, it can not be true when scaling out
			if inst.IsImported() {
				iterErr = errors.New(
					"'imported' is set to 'true' for new instance, this is only used " +
						"for instances imported from tidb-ansible and make no sense when " +
						"deploying new instances, please delete the line or set it to 'false' for new instances")
				return // skip the host to avoid issues
			}

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
					opt.User,
					sshConnProps.Password,
					sshConnProps.IdentityFile,
					sshConnProps.IdentityFilePassphrase,
					sshTimeout,
				).
				EnvInit(inst.GetHost(), globalOptions.User).
				Mkdir(globalOptions.User, inst.GetHost(), dirs...).
				BuildAsStep(fmt.Sprintf("  - Prepare %s:%d", inst.GetHost(), inst.GetSSHPort()))
			envInitTasks = append(envInitTasks, t)
		}
	})

	if iterErr != nil {
		return iterErr
	}

	// Download missing component
	downloadCompTasks = prepare.BuildDownloadCompTasks(clusterVersion, topo)

	// Deploy components to remote
	topo.IterInstance(func(inst spec.Instance) {
		version := spec.ComponentVersion(inst.ComponentName(), clusterVersion)
		deployDir := clusterutil.Abs(globalOptions.User, inst.DeployDir())
		// data dir would be empty for components which don't need it
		dataDirs := clusterutil.MultiDirAbs(globalOptions.User, inst.DataDir())
		// log dir will always be with values, but might not used by the component
		logDir := clusterutil.Abs(globalOptions.User, inst.LogDir())
		// Deploy component
		// prepare deployment server
		t := task.NewBuilder().
			UserSSH(inst.GetHost(), inst.GetSSHPort(), globalOptions.User, sshTimeout).
			Mkdir(globalOptions.User, inst.GetHost(),
				deployDir, logDir,
				filepath.Join(deployDir, "bin"),
				filepath.Join(deployDir, "conf"),
				filepath.Join(deployDir, "scripts")).
			Mkdir(globalOptions.User, inst.GetHost(), dataDirs...)

		// copy dependency component if needed
		switch inst.ComponentName() {
		case spec.ComponentTiSpark:
			t = t.DeploySpark(inst, version, "" /* default srcPath */, deployDir)
		default:
			t = t.CopyComponent(
				inst.ComponentName(),
				inst.OS(),
				inst.Arch(),
				version,
				"", // use default srcPath
				inst.GetHost(),
				deployDir,
			)
		}

		// generate configs for the component
		t = t.InitConfig(
			clusterName,
			clusterVersion,
			d.specManager,
			inst,
			globalOptions.User,
			opt.IgnoreConfigCheck,
			meta.DirPaths{
				Deploy: deployDir,
				Data:   dataDirs,
				Log:    logDir,
				Cache:  d.specManager.Path(clusterName, spec.TempConfigPath),
			},
		)

		deployCompTasks = append(deployCompTasks,
			t.BuildAsStep(fmt.Sprintf("  - Copy %s -> %s", inst.ComponentName(), inst.GetHost())),
		)
	})

	// Deploy monitor relevant components to remote
	dlTasks, dpTasks := buildMonitoredDeployTask(
		d.specManager,
		clusterName,
		uniqueHosts,
		globalOptions,
		topo.GetMonitoredOptions(),
		clusterVersion,
		sshTimeout,
	)
	downloadCompTasks = append(downloadCompTasks, dlTasks...)
	deployCompTasks = append(deployCompTasks, dpTasks...)

	builder := task.NewBuilder().
		Step("+ Generate SSH keys",
			task.NewBuilder().SSHKeyGen(d.specManager.Path(clusterName, "ssh", "id_rsa")).Build()).
		ParallelStep("+ Download TiDB components", downloadCompTasks...).
		ParallelStep("+ Initialize target host environments", envInitTasks...).
		ParallelStep("+ Copy files", deployCompTasks...)

	if afterDeploy != nil {
		afterDeploy(builder, topo)
	}

	t := builder.Build()

	if err := t.Execute(task.NewContext()); err != nil {
		if errorx.Cast(err) != nil {
			// FIXME: Map possible task errors and give suggestions.
			return err
		}
		return perrs.AddStack(err)
	}

	metadata.SetUser(globalOptions.User)
	metadata.SetVersion(clusterVersion)
	err = d.specManager.SaveMeta(clusterName, metadata)

	if err != nil {
		return perrs.AddStack(err)
	}

	hint := color.New(color.Bold).Sprintf("%s start %s", cliutil.OsArgs0(), clusterName)
	log.Infof("Deployed cluster `%s` successfully, you can start the cluster via `%s`", clusterName, hint)
	return nil
}

// ScaleIn the cluster.
func (d *Deployer) ScaleIn(
	clusterName string,
	skipConfirm bool,
	sshTimeout int64,
	force bool,
	nodes []string,
	scale func(builer *task.Builder, metadata spec.Metadata),
) error {
	if !skipConfirm {
		if err := cliutil.PromptForConfirmOrAbortError(
			"This operation will delete the %s nodes in `%s` and all their data.\nDo you want to continue? [y/N]:",
			strings.Join(nodes, ","),
			color.HiYellowString(clusterName)); err != nil {
			return err
		}

		if force {
			if err := cliutil.PromptForConfirmOrAbortError(
				"Forcing scale in is unsafe and may result in data lost for stateful components.\nDo you want to continue? [y/N]:",
			); err != nil {
				return err
			}
		}

		log.Infof("Scale-in nodes...")
	}

	metadata, err := d.meta(clusterName)
	if err != nil {
		return perrs.AddStack(err)
	}

	topo := metadata.GetTopology()
	base := metadata.GetBaseMeta()

	// Regenerate configuration
	var regenConfigTasks []task.Task
	hasImported := false
	deletedNodes := set.NewStringSet(nodes...)
	for _, component := range topo.ComponentsByStartOrder() {
		for _, instance := range component.Instances() {
			if deletedNodes.Exist(instance.ID()) {
				continue
			}
			deployDir := clusterutil.Abs(base.User, instance.DeployDir())
			// data dir would be empty for components which don't need it
			dataDirs := clusterutil.MultiDirAbs(base.User, instance.DataDir())
			// log dir will always be with values, but might not used by the component
			logDir := clusterutil.Abs(base.User, instance.LogDir())

			// Download and copy the latest component to remote if the cluster is imported from Ansible
			tb := task.NewBuilder()
			if instance.IsImported() {
				switch compName := instance.ComponentName(); compName {
				case spec.ComponentGrafana, spec.ComponentPrometheus, spec.ComponentAlertManager:
					version := spec.ComponentVersion(compName, base.Version)
					tb.Download(compName, instance.OS(), instance.Arch(), version).
						CopyComponent(
							compName,
							instance.OS(),
							instance.Arch(),
							version,
							"", // use default srcPath
							instance.GetHost(),
							deployDir,
						)
				}
				hasImported = true
			}

			t := tb.InitConfig(clusterName,
				base.Version,
				d.specManager,
				instance,
				base.User,
				true, // always ignore config check result in scale in
				meta.DirPaths{
					Deploy: deployDir,
					Data:   dataDirs,
					Log:    logDir,
					Cache:  d.specManager.Path(clusterName, spec.TempConfigPath),
				},
			).Build()
			regenConfigTasks = append(regenConfigTasks, t)
		}
	}

	// handle dir scheme changes
	if hasImported {
		if err := spec.HandleImportPathMigration(clusterName); err != nil {
			return err
		}
	}

	b := task.NewBuilder().
		SSHKeySet(
			d.specManager.Path(clusterName, "ssh", "id_rsa"),
			d.specManager.Path(clusterName, "ssh", "id_rsa.pub")).
		ClusterSSH(topo, base.User, sshTimeout)

	// TODO: support command scale in operation.
	scale(b, metadata)

	t := b.Parallel(regenConfigTasks...).Build()

	if err := t.Execute(task.NewContext()); err != nil {
		if errorx.Cast(err) != nil {
			// FIXME: Map possible task errors and give suggestions.
			return err
		}
		return perrs.Trace(err)
	}

	log.Infof("Scaled cluster `%s` in successfully", clusterName)

	return nil
}

// ScaleOut scale out the cluster.
func (d *Deployer) ScaleOut(
	clusterName string,
	topoFile string,
	afterDeploy func(b *task.Builder, newPart spec.Topology),
	final func(b *task.Builder, name string, meta spec.Metadata),
	opt ScaleOutOptions,
	skipConfirm bool,
	optTimeout int64,
	sshTimeout int64,
) error {
	metadata, err := d.meta(clusterName)
	if err != nil {
		return perrs.AddStack(err)
	}

	topo := metadata.GetTopology()
	base := metadata.GetBaseMeta()

	// not allowing validation errors
	if err := topo.Validate(); err != nil {
		return err
	}

	// Inherit existing global configuration. We must assign the inherited values before unmarshalling
	// because some default value rely on the global options and monitored options.
	newPart := topo.NewPart()

	// The no tispark master error is ignored, as if the tispark master is removed from the topology
	// file for some reason (manual edit, for example), it is still possible to scale-out it to make
	// the whole topology back to normal state.
	if err := clusterutil.ParseTopologyYaml(topoFile, &newPart); err != nil &&
		!errors.Is(perrs.Cause(err), spec.ErrNoTiSparkMaster) {
		return err
	}

	if err := validateNewTopo(newPart); err != nil {
		return err
	}

	// Abort scale out operation if the merged topology is invalid
	mergedTopo := topo.MergeTopo(newPart)
	if err := mergedTopo.Validate(); err != nil {
		return err
	}

	if err := prepare.CheckClusterPortConflict(d.specManager, clusterName, mergedTopo); err != nil {
		return err
	}
	if err := prepare.CheckClusterDirConflict(d.specManager, clusterName, mergedTopo); err != nil {
		return err
	}

	patchedComponents := set.NewStringSet()
	newPart.IterInstance(func(instance spec.Instance) {
		if utils.IsExist(d.specManager.Path(clusterName, spec.PatchDirName, instance.ComponentName()+".tar.gz")) {
			patchedComponents.Insert(instance.ComponentName())
		}
	})

	if !skipConfirm {
		// patchedComponents are components that have been patched and overwrited
		if err := d.confirmTopology(clusterName, base.Version, newPart, patchedComponents); err != nil {
			return err
		}
	}

	sshConnProps, err := cliutil.ReadIdentityFileOrPassword(opt.IdentityFile, opt.UsePassword)
	if err != nil {
		return err
	}

	// Build the scale out tasks
	t, err := buildScaleOutTask(d, clusterName, metadata, mergedTopo, opt, sshConnProps, newPart, patchedComponents, optTimeout, sshTimeout, afterDeploy, final)
	if err != nil {
		return err
	}

	if err := t.Execute(task.NewContext()); err != nil {
		if errorx.Cast(err) != nil {
			// FIXME: Map possible task errors and give suggestions.
			return err
		}
		return perrs.Trace(err)
	}

	log.Infof("Scaled cluster `%s` out successfully", clusterName)

	return nil
}

func (d *Deployer) meta(name string) (metadata spec.Metadata, err error) {
	exist, err := d.specManager.Exist(name)
	if err != nil {
		return nil, perrs.AddStack(err)
	}

	if !exist {
		return nil, perrs.Errorf("cluster `%s` not exists", name)
	}

	metadata = d.newMeta()
	err = d.specManager.Metadata(name, metadata)
	if err != nil && !errors.Is(perrs.Cause(err), meta.ErrValidate) &&
		!errors.Is(perrs.Cause(err), spec.ErrNoTiSparkMaster) {
		return nil, err
	}

	return metadata, nil
}

// 1. Write Topology to a temporary file.
// 2. Open file in editor.
// 3. Check and update Topology.
// 4. Save meta file.
func editTopo(origTopo spec.Topology, data []byte, skipConfirm bool) (spec.Topology, error) {
	file, err := ioutil.TempFile(os.TempDir(), "*")
	if err != nil {
		return nil, perrs.AddStack(err)
	}

	name := file.Name()

	_, err = io.Copy(file, bytes.NewReader(data))
	if err != nil {
		return nil, perrs.AddStack(err)
	}

	err = file.Close()
	if err != nil {
		return nil, perrs.AddStack(err)
	}

	err = utils.OpenFileInEditor(name)
	if err != nil {
		return nil, perrs.AddStack(err)
	}

	// Now user finish editing the file.
	newData, err := ioutil.ReadFile(name)
	if err != nil {
		return nil, perrs.AddStack(err)
	}

	newTopo := new(spec.Specification)
	err = yaml.UnmarshalStrict(newData, newTopo)
	if err != nil {
		fmt.Print(color.RedString("New topology could not be saved: "))
		log.Infof("Failed to parse topology file: %v", err)
		if cliutil.PromptForConfirmReverse("Do you want to continue editing? [Y/n]: ") {
			return editTopo(origTopo, newData, skipConfirm)
		}
		log.Infof("Nothing changed.")
		return nil, nil
	}

	// report error if immutable field has been changed
	if err := utils.ValidateSpecDiff(origTopo, newTopo); err != nil {
		fmt.Print(color.RedString("New topology could not be saved: "))
		log.Errorf("%s", err)
		if cliutil.PromptForConfirmReverse("Do you want to continue editing? [Y/n]: ") {
			return editTopo(origTopo, newData, skipConfirm)
		}
		log.Infof("Nothing changed.")
		return nil, nil

	}

	origData, err := yaml.Marshal(origTopo)
	if err != nil {
		return nil, perrs.AddStack(err)
	}

	if bytes.Equal(origData, newData) {
		log.Infof("The file has nothing changed")
		return nil, nil
	}

	utils.ShowDiff(string(origData), string(newData), os.Stdout)

	if !skipConfirm {
		if err := cliutil.PromptForConfirmOrAbortError(
			color.HiYellowString("Please check change highlight above, do you want to apply the change? [y/N]:"),
		); err != nil {
			return nil, err
		}
	}

	return newTopo, nil
}

func formatInstanceStatus(status string) string {
	lowercaseStatus := strings.ToLower(status)

	startsWith := func(prefixs ...string) bool {
		for _, prefix := range prefixs {
			if strings.HasPrefix(lowercaseStatus, prefix) {
				return true
			}
		}
		return false
	}

	switch {
	case startsWith("up|l"): // up|l, up|l|ui
		return color.HiGreenString(status)
	case startsWith("up"):
		return color.GreenString(status)
	case startsWith("down", "err"): // down, down|ui
		return color.RedString(status)
	case startsWith("tombstone", "disconnected"), strings.Contains(status, "offline"):
		return color.YellowString(status)
	default:
		return status
	}
}

func versionCompare(curVersion, newVersion string) error {
	// Can always upgrade to 'nightly' event the current version is 'nightly'
	if newVersion == version.NightlyVersion {
		return nil
	}

	switch semver.Compare(curVersion, newVersion) {
	case -1:
		return nil
	case 0, 1:
		return perrs.Errorf("please specify a higher version than %s", curVersion)
	default:
		return perrs.Errorf("unreachable")
	}
}

type componentInfo struct {
	component string
	version   string
}

func instancesToPatch(topo spec.Topology, options operator.Options) ([]spec.Instance, error) {
	roleFilter := set.NewStringSet(options.Roles...)
	nodeFilter := set.NewStringSet(options.Nodes...)
	components := topo.ComponentsByStartOrder()
	components = operator.FilterComponent(components, roleFilter)

	instances := []spec.Instance{}
	comps := []string{}
	for _, com := range components {
		insts := operator.FilterInstance(com.Instances(), nodeFilter)
		if len(insts) > 0 {
			comps = append(comps, com.Name())
		}
		instances = append(instances, insts...)
	}
	if len(comps) > 1 {
		return nil, fmt.Errorf("can't patch more than one component at once: %v", comps)
	}

	if len(instances) == 0 {
		return nil, fmt.Errorf("no instance found on specifid role(%v) and nodes(%v)", options.Roles, options.Nodes)
	}

	return instances, nil
}

func checkPackage(specManager *spec.SpecManager, clusterName, comp, nodeOS, arch, packagePath string) error {
	metadata, err := spec.ClusterMetadata(clusterName)
	if err != nil && !errors.Is(perrs.Cause(err), meta.ErrValidate) {
		return err
	}

	ver := spec.ComponentVersion(comp, metadata.Version)
	repo, err := clusterutil.NewRepository(nodeOS, arch)
	if err != nil {
		return err
	}
	entry, err := repo.ComponentBinEntry(comp, ver)
	if err != nil {
		return err
	}

	checksum, err := utils.Checksum(packagePath)
	if err != nil {
		return err
	}
	cacheDir := specManager.Path(clusterName, "cache", comp+"-"+checksum[:7])
	if err := os.MkdirAll(cacheDir, 0755); err != nil {
		return err
	}
	if err := exec.Command("tar", "-xvf", packagePath, "-C", cacheDir).Run(); err != nil {
		return err
	}

	if exists := utils.IsExist(path.Join(cacheDir, entry)); !exists {
		return fmt.Errorf("entry %s not found in package %s", entry, packagePath)
	}

	return nil
}

func overwritePatch(specManager *spec.SpecManager, clusterName, comp, packagePath string) error {
	if err := os.MkdirAll(specManager.Path(clusterName, spec.PatchDirName), 0755); err != nil {
		return err
	}

	checksum, err := utils.Checksum(packagePath)
	if err != nil {
		return err
	}

	tg := specManager.Path(clusterName, spec.PatchDirName, comp+"-"+checksum[:7]+".tar.gz")
	if !utils.IsExist(tg) {
		if err := utils.CopyFile(packagePath, tg); err != nil {
			return err
		}
	}

	symlink := specManager.Path(clusterName, spec.PatchDirName, comp+".tar.gz")
	if utils.IsSymExist(symlink) {
		os.Remove(symlink)
	}
	return os.Symlink(tg, symlink)
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

func (d *Deployer) confirmTopology(clusterName, version string, topo spec.Topology, patchedRoles set.StringSet) error {
	log.Infof("Please confirm your topology:")

	cyan := color.New(color.FgCyan, color.Bold)
	fmt.Printf("%s Cluster: %s\n", d.sysName, cyan.Sprint(clusterName))
	fmt.Printf("%s Version: %s\n", d.sysName, cyan.Sprint(version))

	clusterTable := [][]string{
		// Header
		{"Type", "Host", "Ports", "OS/Arch", "Directories"},
	}

	topo.IterInstance(func(instance spec.Instance) {
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
		log.Errorf("    3. The component marked as `patched` has been replaced by previous patch command.")
	}

	if spec, ok := topo.(*spec.Specification); ok {
		if len(spec.TiSparkMasters) > 0 || len(spec.TiSparkWorkers) > 0 {
			log.Warnf("There are TiSpark nodes defined in the topology, please note that you'll need to manually install Java Runtime Environment (JRE) 8 on the host, other wise the TiSpark nodes will fail to start.")
			log.Warnf("You may read the OpenJDK doc for a reference: https://openjdk.java.net/install/")
		}
	}

	return cliutil.PromptForConfirmOrAbortError("Do you want to continue? [y/N]: ")
}

func buildScaleOutTask(
	d *Deployer,
	clusterName string,
	metadata spec.Metadata,
	mergedTopo spec.Topology,
	opt ScaleOutOptions,
	sshConnProps *cliutil.SSHConnectionProps,
	newPart spec.Topology,
	patchedComponents set.StringSet,
	optTimeout int64,
	sshTimeout int64,
	afterDeploy func(b *task.Builder, newPart spec.Topology),
	final func(b *task.Builder, name string, meta spec.Metadata),
) (task.Task, error) {
	var (
		envInitTasks       []task.Task // tasks which are used to initialize environment
		downloadCompTasks  []task.Task // tasks which are used to download components
		deployCompTasks    []task.Task // tasks which are used to copy components to remote host
		refreshConfigTasks []task.Task // tasks which are used to refresh configuration
	)

	topo := metadata.GetTopology()
	base := metadata.GetBaseMeta()
	specManager := d.specManager

	// Initialize the environments
	initializedHosts := set.NewStringSet()
	metadata.GetTopology().IterInstance(func(instance spec.Instance) {
		initializedHosts.Insert(instance.GetHost())
	})
	// uninitializedHosts are hosts which haven't been initialized yet
	uninitializedHosts := make(map[string]hostInfo) // host -> ssh-port, os, arch
	newPart.IterInstance(func(instance spec.Instance) {
		if host := instance.GetHost(); !initializedHosts.Exist(host) {
			if _, found := uninitializedHosts[host]; found {
				return
			}

			uninitializedHosts[host] = hostInfo{
				ssh:  instance.GetSSHPort(),
				os:   instance.OS(),
				arch: instance.Arch(),
			}

			var dirs []string
			globalOptions := metadata.GetTopology().BaseTopo().GlobalOptions
			for _, dir := range []string{globalOptions.DeployDir, globalOptions.DataDir, globalOptions.LogDir} {
				for _, dirname := range strings.Split(dir, ",") {
					if dirname == "" {
						continue
					}
					dirs = append(dirs, clusterutil.Abs(globalOptions.User, dirname))
				}
			}
			t := task.NewBuilder().
				RootSSH(
					instance.GetHost(),
					instance.GetSSHPort(),
					opt.User,
					sshConnProps.Password,
					sshConnProps.IdentityFile,
					sshConnProps.IdentityFilePassphrase,
					sshTimeout,
				).
				EnvInit(instance.GetHost(), base.User).
				Mkdir(globalOptions.User, instance.GetHost(), dirs...).
				Build()
			envInitTasks = append(envInitTasks, t)
		}
	})

	// Download missing component
	downloadCompTasks = convertStepDisplaysToTasks(prepare.BuildDownloadCompTasks(base.Version, newPart))

	// Deploy the new topology and refresh the configuration
	newPart.IterInstance(func(inst spec.Instance) {
		version := spec.ComponentVersion(inst.ComponentName(), base.Version)
		deployDir := clusterutil.Abs(base.User, inst.DeployDir())
		// data dir would be empty for components which don't need it
		dataDirs := clusterutil.MultiDirAbs(base.User, inst.DataDir())
		// log dir will always be with values, but might not used by the component
		logDir := clusterutil.Abs(base.User, inst.LogDir())

		// Deploy component
		tb := task.NewBuilder().
			UserSSH(inst.GetHost(), inst.GetSSHPort(), base.User, sshTimeout).
			Mkdir(base.User, inst.GetHost(),
				deployDir, logDir,
				filepath.Join(deployDir, "bin"),
				filepath.Join(deployDir, "conf"),
				filepath.Join(deployDir, "scripts")).
			Mkdir(base.User, inst.GetHost(), dataDirs...)

		srcPath := ""
		if patchedComponents.Exist(inst.ComponentName()) {
			srcPath = specManager.Path(clusterName, spec.PatchDirName, inst.ComponentName()+".tar.gz")
		}

		// copy dependency component if needed
		switch inst.ComponentName() {
		case spec.ComponentTiSpark:
			tb = tb.DeploySpark(inst, version, srcPath, deployDir)
		default:
			tb.CopyComponent(
				inst.ComponentName(),
				inst.OS(),
				inst.Arch(),
				version,
				srcPath,
				inst.GetHost(),
				deployDir,
			)
		}

		t := tb.ScaleConfig(clusterName,
			base.Version,
			d.specManager,
			topo,
			inst,
			base.User,
			meta.DirPaths{
				Deploy: deployDir,
				Data:   dataDirs,
				Log:    logDir,
			},
		).Build()
		deployCompTasks = append(deployCompTasks, t)
	})

	hasImported := false

	mergedTopo.IterInstance(func(inst spec.Instance) {
		deployDir := clusterutil.Abs(base.User, inst.DeployDir())
		// data dir would be empty for components which don't need it
		dataDirs := clusterutil.MultiDirAbs(base.User, inst.DataDir())
		// log dir will always be with values, but might not used by the component
		logDir := clusterutil.Abs(base.User, inst.LogDir())

		// Download and copy the latest component to remote if the cluster is imported from Ansible
		tb := task.NewBuilder()
		if inst.IsImported() {
			switch compName := inst.ComponentName(); compName {
			case spec.ComponentGrafana, spec.ComponentPrometheus, spec.ComponentAlertManager:
				version := spec.ComponentVersion(compName, base.Version)
				tb.Download(compName, inst.OS(), inst.Arch(), version).
					CopyComponent(compName, inst.OS(), inst.Arch(), version, "", inst.GetHost(), deployDir)
			}
			hasImported = true
		}

		// Refresh all configuration
		t := tb.InitConfig(clusterName,
			base.Version,
			d.specManager,
			inst,
			base.User,
			true, // always ignore config check result in scale out
			meta.DirPaths{
				Deploy: deployDir,
				Data:   dataDirs,
				Log:    logDir,
				Cache:  specManager.Path(clusterName, spec.TempConfigPath),
			},
		).Build()
		refreshConfigTasks = append(refreshConfigTasks, t)
	})

	// handle dir scheme changes
	if hasImported {
		if err := spec.HandleImportPathMigration(clusterName); err != nil {
			return task.NewBuilder().Build(), err
		}
	}

	// Deploy monitor relevant components to remote
	dlTasks, dpTasks := buildMonitoredDeployTask(
		specManager,
		clusterName,
		uninitializedHosts,
		topo.BaseTopo().GlobalOptions,
		topo.BaseTopo().MonitoredOptions,
		base.Version,
		sshTimeout,
	)
	downloadCompTasks = append(downloadCompTasks, convertStepDisplaysToTasks(dlTasks)...)
	deployCompTasks = append(deployCompTasks, convertStepDisplaysToTasks(dpTasks)...)

	builder := task.NewBuilder().
		SSHKeySet(
			specManager.Path(clusterName, "ssh", "id_rsa"),
			specManager.Path(clusterName, "ssh", "id_rsa.pub")).
		Parallel(downloadCompTasks...).
		Parallel(envInitTasks...).
		ClusterSSH(topo, base.User, sshTimeout).
		Parallel(deployCompTasks...)

	if afterDeploy != nil {
		afterDeploy(builder, newPart)
	}

	// TODO: find another way to make sure current cluster started
	builder.
		Serial(task.NewFunc("StartCluster", func(ctx *task.Context) error {
			return operator.Start(ctx, metadata.GetTopology(), operator.Options{OptTimeout: optTimeout})
		})).
		ClusterSSH(newPart, base.User, sshTimeout).
		Func("save meta", func(_ *task.Context) error {
			metadata.SetTopology(mergedTopo)
			return d.specManager.SaveMeta(clusterName, metadata)
		}).
		Serial(task.NewFunc("StartCluster", func(ctx *task.Context) error {
			return operator.Start(ctx, newPart, operator.Options{OptTimeout: optTimeout})
		})).
		Parallel(refreshConfigTasks...).
		Serial(task.NewFunc("RestartCluster", func(ctx *task.Context) error {
			return operator.Restart(ctx, metadata.GetTopology(), operator.Options{
				Roles:      []string{spec.ComponentPrometheus},
				OptTimeout: optTimeout,
			})
		}))

	if final != nil {
		final(builder, clusterName, metadata)
	}

	return builder.Build(), nil
}

type hostInfo struct {
	ssh  int    // ssh port of host
	os   string // operating system
	arch string // cpu architecture
	// vendor string
}

// Deprecated
func convertStepDisplaysToTasks(t []*task.StepDisplay) []task.Task {
	tasks := make([]task.Task, 0, len(t))
	for _, sd := range t {
		tasks = append(tasks, sd)
	}
	return tasks
}

func buildMonitoredDeployTask(
	specManager *spec.SpecManager,
	clusterName string,
	uniqueHosts map[string]hostInfo, // host -> ssh-port, os, arch
	globalOptions *spec.GlobalOptions,
	monitoredOptions *spec.MonitoredOptions,
	version string,
	sshTimeout int64,
) (downloadCompTasks []*task.StepDisplay, deployCompTasks []*task.StepDisplay) {
	uniqueCompOSArch := make(map[string]struct{}) // comp-os-arch -> {}
	// monitoring agents
	for _, comp := range []string{spec.ComponentNodeExporter, spec.ComponentBlackboxExporter} {
		version := spec.ComponentVersion(comp, version)

		for host, info := range uniqueHosts {
			// populate unique os/arch set
			key := fmt.Sprintf("%s-%s-%s", comp, info.os, info.arch)
			if _, found := uniqueCompOSArch[key]; !found {
				uniqueCompOSArch[key] = struct{}{}
				downloadCompTasks = append(downloadCompTasks, task.NewBuilder().
					Download(comp, info.os, info.arch, version).
					BuildAsStep(fmt.Sprintf("  - Download %s:%s (%s/%s)", comp, version, info.os, info.arch)))
			}

			deployDir := clusterutil.Abs(globalOptions.User, monitoredOptions.DeployDir)
			// data dir would be empty for components which don't need it
			dataDir := monitoredOptions.DataDir
			// the default data_dir is relative to deploy_dir
			if dataDir != "" && !strings.HasPrefix(dataDir, "/") {
				dataDir = filepath.Join(deployDir, dataDir)
			}
			// log dir will always be with values, but might not used by the component
			logDir := clusterutil.Abs(globalOptions.User, monitoredOptions.LogDir)
			// Deploy component
			t := task.NewBuilder().
				UserSSH(host, info.ssh, globalOptions.User, sshTimeout).
				Mkdir(globalOptions.User, host,
					deployDir, dataDir, logDir,
					filepath.Join(deployDir, "bin"),
					filepath.Join(deployDir, "conf"),
					filepath.Join(deployDir, "scripts")).
				CopyComponent(
					comp,
					info.os,
					info.arch,
					version,
					"",
					host,
					deployDir,
				).
				MonitoredConfig(
					clusterName,
					comp,
					host,
					globalOptions.ResourceControl,
					monitoredOptions,
					globalOptions.User,
					meta.DirPaths{
						Deploy: deployDir,
						Data:   []string{dataDir},
						Log:    logDir,
						Cache:  specManager.Path(clusterName, spec.TempConfigPath),
					},
				).
				BuildAsStep(fmt.Sprintf("  - Copy %s -> %s", comp, host))
			deployCompTasks = append(deployCompTasks, t)
		}
	}
	return
}
