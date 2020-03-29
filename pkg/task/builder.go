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
	"github.com/pingcap-incubator/tiops/pkg/meta"
	operator "github.com/pingcap-incubator/tiops/pkg/operation"
	"github.com/pingcap-incubator/tiup/pkg/repository"
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
func (b *Builder) RootSSH(host string, port int, user, password, keyFile, passphrase string) *Builder {
	b.tasks = append(b.tasks, &RootSSH{
		host:       host,
		port:       port,
		user:       user,
		password:   password,
		keyFile:    keyFile,
		passphrase: passphrase,
	})
	return b
}

// UserSSH append a UserSSH task to the current task collection
func (b *Builder) UserSSH(host, deployUser string) *Builder {
	b.tasks = append(b.tasks, &UserSSH{
		host:       host,
		deployUser: deployUser,
	})
	return b
}

// ClusterSSH init all UserSSH need for the cluster.
func (b *Builder) ClusterSSH(spec *meta.Specification, deployUser string) *Builder {
	var tasks []Task
	for _, com := range spec.ComponentsByStartOrder() {
		for _, in := range com.Instances() {
			tasks = append(tasks, &UserSSH{
				host:       in.GetHost(),
				deployUser: deployUser,
			})
		}
	}

	b.tasks = append(b.tasks, Parallel(tasks))

	return b
}

// UpdateMeta maintain the meta information
func (b *Builder) UpdateMeta(cluster string, metadata *meta.ClusterMeta, deletedNodeIds []string) *Builder {
	b.tasks = append(b.tasks, &UpdateMeta{
		cluster:        cluster,
		metadata:       metadata,
		deletedNodesID: deletedNodeIds,
	})
	return b
}

// CopyFile appends a CopyFile task to the current task collection
func (b *Builder) CopyFile(src, dstHost, dstPath string) *Builder {
	b.tasks = append(b.tasks, &CopyFile{
		src:     src,
		dstHost: dstHost,
		dstPath: dstPath,
	})
	return b
}

// Download appends a Downloader task to the current task collection
func (b *Builder) Download(component string, version repository.Version) *Builder {
	b.tasks = append(b.tasks, &Downloader{
		component: component,
		version:   version,
	})
	return b
}

// CopyComponent appends a CopyComponent task to the current task collection
func (b *Builder) CopyComponent(component string, version repository.Version, dstHost, dstDir string) *Builder {
	b.tasks = append(b.tasks, &CopyComponent{
		component: component,
		version:   version,
		host:      dstHost,
		dstDir:    dstDir,
	})
	return b
}

// BackupComponent appends a BackupComponent task to the current task collection
func (b *Builder) BackupComponent(component, fromVer string, dstHost, dstDir string) *Builder {
	b.tasks = append(b.tasks, &BackupComponent{
		component: component,
		fromVer:   fromVer,
		host:      dstHost,
		dstDir:    dstDir,
	})
	return b
}

// InitConfig appends a CopyComponent task to the current task collection
func (b *Builder) InitConfig(name string, inst meta.Instance, deployUser, deployDir string) *Builder {
	b.tasks = append(b.tasks, &InitConfig{
		name:       name,
		instance:   inst,
		deployUser: deployUser,
		deployDir:  deployDir,
	})
	return b
}

// ScaleConfig generate temporary config on scaling
func (b *Builder) ScaleConfig(name string, base *meta.TopologySpecification, inst meta.Instance, deployUser, deployDir string) *Builder {
	b.tasks = append(b.tasks, &ScaleConfig{
		name:       name,
		base:       base,
		instance:   inst,
		deployUser: deployUser,
		deployDir:  deployDir,
	})
	return b
}

// MonitoredConfig appends a CopyComponent task to the current task collection
func (b *Builder) MonitoredConfig(name string, options meta.MonitoredOptions, deployUser, deployDir string) *Builder {
	b.tasks = append(b.tasks, &MonitoredConfig{
		name:       name,
		options:    options,
		deployUser: deployUser,
		deployDir:  deployDir,
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
func (b *Builder) EnvInit(host, deployUser string) *Builder {
	b.tasks = append(b.tasks, &EnvInit{
		host:       host,
		deployUser: deployUser,
	})
	return b
}

// ClusterOperate appends a cluster operation task.
// All the UserSSH needed must be init first.
func (b *Builder) ClusterOperate(
	spec *meta.Specification,
	op operator.Operation,
	options operator.Options,
) *Builder {
	b.tasks = append(b.tasks, &ClusterOperate{
		spec:    spec,
		op:      op,
		options: options,
	})

	return b
}

// Mkdir appends a Mkdir task to the current task collection
func (b *Builder) Mkdir(host string, dirs ...string) *Builder {
	b.tasks = append(b.tasks, &Mkdir{
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
		sudo:    false,
	})
	return b
}

// Parallel appends a parallel task to the current task collection
func (b *Builder) Parallel(tasks ...Task) *Builder {
	b.tasks = append(b.tasks, Parallel(tasks))
	return b
}

// Serial appends the tasks to the tail of queue
func (b *Builder) Serial(tasks ...Task) *Builder {
	b.tasks = append(b.tasks, tasks...)
	return b
}

// Build returns a task that contains all tasks appended by previous operation
func (b *Builder) Build() Task {
	if len(b.tasks) == 1 {
		return b.tasks[0]
	}
	return Serial(b.tasks)
}
