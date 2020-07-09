package deploy

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"os"
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
	"sigs.k8s.io/yaml"
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
			return operator.Start(ctx, topo, options)
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
			return operator.Start(ctx, topo, options)
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
