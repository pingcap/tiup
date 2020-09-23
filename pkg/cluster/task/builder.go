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

package task

import (
	"crypto/tls"
	"fmt"
	"path/filepath"

	"github.com/pingcap/tiup/pkg/cluster/executor"
	operator "github.com/pingcap/tiup/pkg/cluster/operation"
	"github.com/pingcap/tiup/pkg/cluster/spec"
	"github.com/pingcap/tiup/pkg/crypto"
	"github.com/pingcap/tiup/pkg/meta"
)

// Builder is used to build TiOps task
type Builder struct {
	tasks []Task
}

// NewBuilder returns a *Builder instance
func NewBuilder() *Builder {
	return &Builder{}
}

// RootSSH appends a RootSSH task to the current task collection
func (b *Builder) RootSSH(
	host string,
	port int,
	user, password, keyFile, passphrase string,
	sshTimeout int64,
	sshType executor.SSHType,
	defaultSSHType executor.SSHType,
) *Builder {
	if sshType == "" {
		sshType = defaultSSHType
	}
	b.tasks = append(b.tasks, &RootSSH{
		host:       host,
		port:       port,
		user:       user,
		password:   password,
		keyFile:    keyFile,
		passphrase: passphrase,
		timeout:    sshTimeout,
		sshType:    sshType,
	})
	return b
}

// UserSSH append a UserSSH task to the current task collection
func (b *Builder) UserSSH(host string, port int, deployUser string, sshTimeout int64, sshType, defaultSSHType executor.SSHType) *Builder {
	if sshType == "" {
		sshType = defaultSSHType
	}
	b.tasks = append(b.tasks, &UserSSH{
		host:       host,
		port:       port,
		deployUser: deployUser,
		timeout:    sshTimeout,
		sshType:    sshType,
	})
	return b
}

// Func append a func task.
func (b *Builder) Func(name string, fn func(ctx *Context) error) *Builder {
	b.tasks = append(b.tasks, &Func{
		name: name,
		fn:   fn,
	})
	return b
}

// ClusterSSH init all UserSSH need for the cluster.
func (b *Builder) ClusterSSH(spec spec.Topology, deployUser string, sshTimeout int64, sshType, defaultSSHType executor.SSHType) *Builder {
	if sshType == "" {
		sshType = defaultSSHType
	}
	var tasks []Task
	for _, com := range spec.ComponentsByStartOrder() {
		for _, in := range com.Instances() {
			tasks = append(tasks, &UserSSH{
				host:       in.GetHost(),
				port:       in.GetSSHPort(),
				deployUser: deployUser,
				timeout:    sshTimeout,
				sshType:    sshType,
			})
		}
	}

	b.tasks = append(b.tasks, &Parallel{inner: tasks})

	return b
}

// UpdateMeta maintain the meta information
func (b *Builder) UpdateMeta(cluster string, metadata *spec.ClusterMeta, deletedNodeIds []string) *Builder {
	b.tasks = append(b.tasks, &UpdateMeta{
		cluster:        cluster,
		metadata:       metadata,
		deletedNodesID: deletedNodeIds,
	})
	return b
}

// UpdateTopology maintain the topology information
func (b *Builder) UpdateTopology(cluster, profile string, metadata *spec.ClusterMeta, deletedNodeIds []string) *Builder {
	b.tasks = append(b.tasks, &UpdateTopology{
		metadata:       metadata,
		cluster:        cluster,
		profileDir:     profile,
		deletedNodesID: deletedNodeIds,
	})
	return b
}

// CopyFile appends a CopyFile task to the current task collection
func (b *Builder) CopyFile(src, dst, server string, download bool) *Builder {
	b.tasks = append(b.tasks, &CopyFile{
		src:      src,
		dst:      dst,
		remote:   server,
		download: download,
	})
	return b
}

// Download appends a Downloader task to the current task collection
func (b *Builder) Download(component, os, arch string, version string) *Builder {
	b.tasks = append(b.tasks, NewDownloader(component, os, arch, version))
	return b
}

// CopyComponent appends a CopyComponent task to the current task collection
func (b *Builder) CopyComponent(component, os, arch string,
	version string,
	srcPath, dstHost, dstDir string,
) *Builder {
	b.tasks = append(b.tasks, &CopyComponent{
		component: component,
		os:        os,
		arch:      arch,
		version:   version,
		srcPath:   srcPath,
		host:      dstHost,
		dstDir:    dstDir,
	})
	return b
}

// InstallPackage appends a InstallPackage task to the current task collection
func (b *Builder) InstallPackage(srcPath, dstHost, dstDir string) *Builder {
	b.tasks = append(b.tasks, &InstallPackage{
		srcPath: srcPath,
		host:    dstHost,
		dstDir:  dstDir,
	})
	return b
}

// BackupComponent appends a BackupComponent task to the current task collection
func (b *Builder) BackupComponent(component, fromVer string, host, deployDir string) *Builder {
	b.tasks = append(b.tasks, &BackupComponent{
		component: component,
		fromVer:   fromVer,
		host:      host,
		deployDir: deployDir,
	})
	return b
}

// InitConfig appends a CopyComponent task to the current task collection
func (b *Builder) InitConfig(clusterName, clusterVersion string, specManager *spec.SpecManager, inst spec.Instance, deployUser string, ignoreCheck bool, paths meta.DirPaths) *Builder {
	b.tasks = append(b.tasks, &InitConfig{
		specManager:    specManager,
		clusterName:    clusterName,
		clusterVersion: clusterVersion,
		instance:       inst,
		deployUser:     deployUser,
		ignoreCheck:    ignoreCheck,
		paths:          paths,
	})
	return b
}

// ScaleConfig generate temporary config on scaling
func (b *Builder) ScaleConfig(clusterName, clusterVersion string, specManager *spec.SpecManager, topo spec.Topology, inst spec.Instance, deployUser string, paths meta.DirPaths) *Builder {
	b.tasks = append(b.tasks, &ScaleConfig{
		specManager:    specManager,
		clusterName:    clusterName,
		clusterVersion: clusterVersion,
		base:           topo,
		instance:       inst,
		deployUser:     deployUser,
		paths:          paths,
	})
	return b
}

// MonitoredConfig appends a CopyComponent task to the current task collection
func (b *Builder) MonitoredConfig(name, comp, host string, globResCtl meta.ResourceControl, options *spec.MonitoredOptions, deployUser string, paths meta.DirPaths) *Builder {
	b.tasks = append(b.tasks, &MonitoredConfig{
		name:       name,
		component:  comp,
		host:       host,
		globResCtl: globResCtl,
		options:    options,
		deployUser: deployUser,
		paths:      paths,
	})
	return b
}

// SSHKeyGen appends a SSHKeyGen task to the current task collection
func (b *Builder) SSHKeyGen(keypath string) *Builder {
	b.tasks = append(b.tasks, &SSHKeyGen{
		keypath: keypath,
	})
	return b
}

// SSHKeySet appends a SSHKeySet task to the current task collection
func (b *Builder) SSHKeySet(privKeyPath, pubKeyPath string) *Builder {
	b.tasks = append(b.tasks, &SSHKeySet{
		privateKeyPath: privKeyPath,
		publicKeyPath:  pubKeyPath,
	})
	return b
}

// EnvInit appends a EnvInit task to the current task collection
func (b *Builder) EnvInit(host, deployUser string, userGroup string, skipCreateUser bool) *Builder {
	b.tasks = append(b.tasks, &EnvInit{
		host:           host,
		deployUser:     deployUser,
		userGroup:      userGroup,
		skipCreateUser: skipCreateUser,
	})
	return b
}

// ClusterOperate appends a cluster operation task.
// All the UserSSH needed must be init first.
func (b *Builder) ClusterOperate(
	spec *spec.Specification,
	op operator.Operation,
	options operator.Options,
	tlsCfg *tls.Config,
) *Builder {
	b.tasks = append(b.tasks, &ClusterOperate{
		spec:    spec,
		op:      op,
		options: options,
		tlsCfg:  tlsCfg,
	})

	return b
}

// Mkdir appends a Mkdir task to the current task collection
func (b *Builder) Mkdir(user, host string, dirs ...string) *Builder {
	b.tasks = append(b.tasks, &Mkdir{
		user: user,
		host: host,
		dirs: dirs,
	})
	return b
}

// Rmdir appends a Rmdir task to the current task collection
func (b *Builder) Rmdir(host string, dirs ...string) *Builder {
	b.tasks = append(b.tasks, &Rmdir{
		host: host,
		dirs: dirs,
	})
	return b
}

// Shell command on cluster host
func (b *Builder) Shell(host, command string, sudo bool) *Builder {
	b.tasks = append(b.tasks, &Shell{
		host:    host,
		command: command,
		sudo:    sudo,
	})
	return b
}

// SystemCtl run systemctl on host
func (b *Builder) SystemCtl(host, unit, action string, daemonReload bool) *Builder {
	b.tasks = append(b.tasks, &SystemCtl{
		host:         host,
		unit:         unit,
		action:       action,
		daemonReload: daemonReload,
	})
	return b
}

// Sysctl set a kernel parameter
func (b *Builder) Sysctl(host, key, val string) *Builder {
	b.tasks = append(b.tasks, &Sysctl{
		host: host,
		key:  key,
		val:  val,
	})
	return b
}

// Limit set a system limit
func (b *Builder) Limit(host, domain, limit, item, value string) *Builder {
	b.tasks = append(b.tasks, &Limit{
		host:   host,
		domain: domain,
		limit:  limit,
		item:   item,
		value:  value,
	})
	return b
}

// CheckSys checks system information of deploy server
func (b *Builder) CheckSys(host, dataDir, checkType string, topo *spec.Specification, opt *operator.CheckOptions) *Builder {
	b.tasks = append(b.tasks, &CheckSys{
		host:    host,
		topo:    topo,
		opt:     opt,
		dataDir: dataDir,
		check:   checkType,
	})
	return b
}

// DeploySpark deployes spark as dependency of TiSpark
func (b *Builder) DeploySpark(inst spec.Instance, sparkVersion, srcPath, deployDir string) *Builder {
	sparkSubPath := spec.ComponentSubDir(spec.ComponentSpark, sparkVersion)
	return b.CopyComponent(
		spec.ComponentSpark,
		inst.OS(),
		inst.Arch(),
		sparkVersion,
		srcPath,
		inst.GetHost(),
		deployDir,
	).Shell( // spark is under a subdir, move it to deploy dir
		inst.GetHost(),
		fmt.Sprintf(
			"cp -rf %[1]s %[2]s/ && cp -rf %[3]s/* %[2]s/ && rm -rf %[1]s %[3]s",
			filepath.Join(deployDir, "bin", sparkSubPath),
			deployDir,
			filepath.Join(deployDir, sparkSubPath),
		),
		false, // (not) sudo
	).CopyComponent(
		inst.ComponentName(),
		inst.OS(),
		inst.Arch(),
		"", // use the latest stable version
		srcPath,
		inst.GetHost(),
		deployDir,
	).Shell( // move tispark jar to correct path
		inst.GetHost(),
		fmt.Sprintf(
			"cp -f %[1]s/*.jar %[2]s/jars/ && rm -f %[1]s/*.jar",
			filepath.Join(deployDir, "bin"),
			deployDir,
		),
		false, // (not) sudo
	)
}

// TLSCert geenrates certificate for instance and transfer it to the server
func (b *Builder) TLSCert(inst spec.Instance, ca *crypto.CertificateAuthority, paths meta.DirPaths) *Builder {
	b.tasks = append(b.tasks, &TLSCert{
		ca:    ca,
		inst:  inst,
		paths: paths,
	})
	return b
}

// Parallel appends a parallel task to the current task collection
func (b *Builder) Parallel(ignoreError bool, tasks ...Task) *Builder {
	if len(tasks) > 0 {
		b.tasks = append(b.tasks, &Parallel{ignoreError: ignoreError, inner: tasks})
	}
	return b
}

// Serial appends the tasks to the tail of queue
func (b *Builder) Serial(tasks ...Task) *Builder {
	if len(tasks) > 0 {
		b.tasks = append(b.tasks, tasks...)
	}
	return b
}

// Build returns a task that contains all tasks appended by previous operation
func (b *Builder) Build() Task {
	// Serial handles event internally. So the following 3 lines are commented out.
	//if len(b.tasks) == 1 {
	//	return b.tasks[0]
	//}
	return &Serial{inner: b.tasks}
}

// Step appends a new StepDisplay task, which will print single line progress for inner tasks.
func (b *Builder) Step(prefix string, inner Task) *Builder {
	b.Serial(newStepDisplay(prefix, inner))
	return b
}

// ParallelStep appends a new ParallelStepDisplay task, which will print multi line progress in parallel
// for inner tasks. Inner tasks must be a StepDisplay task.
func (b *Builder) ParallelStep(prefix string, ignoreError bool, tasks ...*StepDisplay) *Builder {
	b.tasks = append(b.tasks, newParallelStepDisplay(prefix, ignoreError, tasks...))
	return b
}

// BuildAsStep returns a task that is wrapped by a StepDisplay. The task will print single line progress.
func (b *Builder) BuildAsStep(prefix string) *StepDisplay {
	inner := b.Build()
	return newStepDisplay(prefix, inner)
}
