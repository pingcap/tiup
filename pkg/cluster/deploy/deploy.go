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
	"sort"
	"strings"

	"github.com/fatih/color"
	"github.com/joomcode/errorx"
	perrs "github.com/pingcap/errors"
	"github.com/pingcap/tiup/pkg/cliutil"
	"github.com/pingcap/tiup/pkg/cluster/clusterutil"
	operator "github.com/pingcap/tiup/pkg/cluster/operation"
	"github.com/pingcap/tiup/pkg/cluster/spec"
	"github.com/pingcap/tiup/pkg/cluster/task"
	"github.com/pingcap/tiup/pkg/logger/log"
	"github.com/pingcap/tiup/pkg/meta"
	"github.com/pingcap/tiup/pkg/set"
	"github.com/pingcap/tiup/pkg/utils"
	"github.com/pingcap/tiup/pkg/version"
	"golang.org/x/mod/semver"
	"gopkg.in/yaml.v2"
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
			spec.ClusterPath(clusterName, "ssh", "id_rsa"),
			spec.ClusterPath(clusterName, "ssh", "id_rsa.pub")).
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
	err = ctx.SetSSHKeySet(spec.ClusterPath(clusterName, "ssh", "id_rsa"),
		spec.ClusterPath(clusterName, "ssh", "id_rsa.pub"))
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
			inst, base.User,
			opt.IgnoreConfigCheck,
			meta.DirPaths{
				Deploy: deployDir,
				Data:   dataDirs,
				Log:    logDir,
				Cache:  spec.ClusterPath(clusterName, spec.TempConfigPath),
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
			spec.ClusterPath(clusterName, "ssh", "id_rsa"),
			spec.ClusterPath(clusterName, "ssh", "id_rsa.pub")).
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
				inst,
				base.User,
				opt.IgnoreConfigCheck,
				meta.DirPaths{
					Deploy: deployDir,
					Data:   dataDirs,
					Log:    logDir,
					Cache:  spec.ClusterPath(clusterName, spec.TempConfigPath),
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
			spec.ClusterPath(clusterName, "ssh", "id_rsa"),
			spec.ClusterPath(clusterName, "ssh", "id_rsa.pub")).
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
	if err := checkPackage(clusterName, insts[0].ComponentName(), insts[0].OS(), insts[0].Arch(), packagePath); err != nil {
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
			spec.ClusterPath(clusterName, "ssh", "id_rsa"),
			spec.ClusterPath(clusterName, "ssh", "id_rsa.pub")).
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
		if err := overwritePatch(clusterName, insts[0].ComponentName(), packagePath); err != nil {
			return err
		}
	}

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

func checkPackage(clusterName, comp, nodeOS, arch, packagePath string) error {
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
	cacheDir := spec.ClusterPath(clusterName, "cache", comp+"-"+checksum[:7])
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

func overwritePatch(clusterName, comp, packagePath string) error {
	if err := os.MkdirAll(spec.ClusterPath(clusterName, spec.PatchDirName), 0755); err != nil {
		return err
	}

	checksum, err := utils.Checksum(packagePath)
	if err != nil {
		return err
	}

	tg := spec.ClusterPath(clusterName, spec.PatchDirName, comp+"-"+checksum[:7]+".tar.gz")
	if !utils.IsExist(tg) {
		if err := utils.CopyFile(packagePath, tg); err != nil {
			return err
		}
	}

	symlink := spec.ClusterPath(clusterName, spec.PatchDirName, comp+".tar.gz")
	if utils.IsSymExist(symlink) {
		os.Remove(symlink)
	}
	return os.Symlink(tg, symlink)
}
