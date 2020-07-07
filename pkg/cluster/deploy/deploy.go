package deploy

import (
	"errors"

	"github.com/joomcode/errorx"
	perrs "github.com/pingcap/errors"
	"github.com/pingcap/tiup/pkg/cliutil"
	operator "github.com/pingcap/tiup/pkg/cluster/operation"
	"github.com/pingcap/tiup/pkg/cluster/spec"
	"github.com/pingcap/tiup/pkg/cluster/task"
	"github.com/pingcap/tiup/pkg/logger/log"
	"github.com/pingcap/tiup/pkg/meta"
)

// Deployer to deploy a cluster.
type Deployer struct {
	sysName     string
	specManager *meta.SpecManager
	newMeta     func() spec.Metadata
}

// NewDeployer create a Deployer.
func NewDeployer(sysName string, specManager *meta.SpecManager, newMeta func() spec.Metadata) *Deployer {
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
	if err != nil && !errors.Is(perrs.Cause(err), meta.ErrValidate) {
		return nil, err
	}

	return metadata, nil
}
