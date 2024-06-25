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

package spec

import (
	"context"
	"crypto/tls"
	"fmt"
	"os"
	"path"
	"path/filepath"
	"reflect"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/pingcap/errors"
	"github.com/pingcap/tiup/pkg/checkpoint"
	"github.com/pingcap/tiup/pkg/cluster/ctxt"
	"github.com/pingcap/tiup/pkg/cluster/module"
	system "github.com/pingcap/tiup/pkg/cluster/template/systemd"
	"github.com/pingcap/tiup/pkg/meta"
	"github.com/pingcap/tiup/pkg/utils"
	"go.uber.org/zap"
)

// Components names
const (
	ComponentTiDB             = "tidb"
	ComponentTiKV             = "tikv"
	ComponentPD               = "pd"
	ComponentTSO              = "tso"
	ComponentScheduling       = "scheduling"
	ComponentTiFlash          = "tiflash"
	ComponentTiProxy          = "tiproxy"
	ComponentGrafana          = "grafana"
	ComponentDrainer          = "drainer"
	ComponentDashboard        = "tidb-dashboard"
	ComponentPump             = "pump"
	ComponentCDC              = "cdc"
	ComponentTiKVCDC          = "tikv-cdc"
	ComponentTiSpark          = "tispark"
	ComponentSpark            = "spark"
	ComponentAlertmanager     = "alertmanager"
	ComponentDMMaster         = "dm-master"
	ComponentDMWorker         = "dm-worker"
	ComponentPrometheus       = "prometheus"
	ComponentBlackboxExporter = "blackbox_exporter"
	ComponentNodeExporter     = "node_exporter"
	ComponentCheckCollector   = "insight"
)

var (
	// CopyConfigFile is the checkpoint to cache config file transfer action
	CopyConfigFile = checkpoint.Register(
		checkpoint.Field("config-file", reflect.DeepEqual),
	)
)

// Component represents a component of the cluster.
type Component interface {
	Name() string
	Role() string
	Source() string
	Instances() []Instance
	CalculateVersion(string) string
	SetVersion(string)
}

// RollingUpdateInstance represent a instance need to transfer state when restart.
// e.g transfer leader.
type RollingUpdateInstance interface {
	PreRestart(ctx context.Context, topo Topology, apiTimeoutSeconds int, tlsCfg *tls.Config) error
	PostRestart(ctx context.Context, topo Topology, tlsCfg *tls.Config) error
}

// Instance represents the instance.
type Instance interface {
	InstanceSpec
	ID() string
	Ready(context.Context, ctxt.Executor, uint64, *tls.Config) error
	InitConfig(ctx context.Context, e ctxt.Executor, clusterName string, clusterVersion string, deployUser string, paths meta.DirPaths) error
	ScaleConfig(ctx context.Context, e ctxt.Executor, topo Topology, clusterName string, clusterVersion string, deployUser string, paths meta.DirPaths) error
	PrepareStart(ctx context.Context, tlsCfg *tls.Config) error
	ComponentName() string
	ComponentSource() string
	InstanceName() string
	ServiceName() string
	ResourceControl() meta.ResourceControl
	GetHost() string
	GetManageHost() string
	GetPort() int
	GetSSHPort() int
	GetNumaNode() string
	GetNumaCores() string
	DeployDir() string
	UsedPorts() []int
	UsedDirs() []string
	Status(ctx context.Context, timeout time.Duration, tlsCfg *tls.Config, pdList ...string) string
	Uptime(ctx context.Context, timeout time.Duration, tlsCfg *tls.Config) time.Duration
	DataDir() string
	LogDir() string
	OS() string // only linux supported now
	Arch() string
	IsPatched() bool
	SetPatched(bool)
	CalculateVersion(string) string
	// SetVersion(string)
	setTLSConfig(ctx context.Context, enableTLS bool, configs map[string]any, paths meta.DirPaths) (map[string]any, error)
}

// PortStarted wait until a port is being listened
func PortStarted(ctx context.Context, e ctxt.Executor, port int, timeout uint64) error {
	c := module.WaitForConfig{
		Port:    port,
		State:   "started",
		Timeout: time.Second * time.Duration(timeout),
	}
	w := module.NewWaitFor(c)
	return w.Execute(ctx, e)
}

// PortStopped wait until a port is being released
func PortStopped(ctx context.Context, e ctxt.Executor, port int, timeout uint64) error {
	c := module.WaitForConfig{
		Port:    port,
		State:   "stopped",
		Timeout: time.Second * time.Duration(timeout),
	}
	w := module.NewWaitFor(c)
	return w.Execute(ctx, e)
}

// BaseInstance implements some method of Instance interface..
type BaseInstance struct {
	InstanceSpec

	Name       string
	Host       string
	ManageHost string
	ListenHost string
	Port       int
	SSHP       int
	Source     string
	NumaNode   string
	NumaCores  string

	Ports    []int
	Dirs     []string
	StatusFn func(ctx context.Context, timeout time.Duration, tlsCfg *tls.Config, pdHosts ...string) string
	UptimeFn func(ctx context.Context, timeout time.Duration, tlsCfg *tls.Config) time.Duration

	Component Component
}

// Ready implements Instance interface
func (i *BaseInstance) Ready(ctx context.Context, e ctxt.Executor, timeout uint64, _ *tls.Config) error {
	return PortStarted(ctx, e, i.Port, timeout)
}

// InitConfig init the service configuration.
func (i *BaseInstance) InitConfig(ctx context.Context, e ctxt.Executor, opt GlobalOptions, user string, paths meta.DirPaths) (err error) {
	comp := i.ComponentName()
	host := i.GetHost()
	port := i.GetPort()
	sysCfg := filepath.Join(paths.Cache, fmt.Sprintf("%s-%s-%d.service", comp, host, port))

	// insert checkpoint
	point := checkpoint.Acquire(ctx, CopyConfigFile, map[string]any{"config-file": sysCfg})
	defer func() {
		point.Release(err, zap.String("config-file", sysCfg))
	}()

	if point.Hit() != nil {
		return nil
	}

	systemdMode := opt.SystemdMode
	if len(systemdMode) == 0 {
		systemdMode = SystemMode
	}

	resource := MergeResourceControl(opt.ResourceControl, i.ResourceControl())
	systemCfg := system.NewConfig(comp, user, paths.Deploy).
		WithMemoryLimit(resource.MemoryLimit).
		WithCPUQuota(resource.CPUQuota).
		WithLimitCORE(resource.LimitCORE).
		WithIOReadBandwidthMax(resource.IOReadBandwidthMax).
		WithIOWriteBandwidthMax(resource.IOWriteBandwidthMax).
		WithSystemdMode(string(systemdMode))

	// For not auto start if using binlogctl to offline.
	// bad design
	if comp == ComponentPump || comp == ComponentDrainer {
		systemCfg.Restart = "on-failure"
	}

	if err := systemCfg.ConfigToFile(sysCfg); err != nil {
		return errors.Trace(err)
	}
	tgt := filepath.Join("/tmp", comp+"_"+uuid.New().String()+".service")
	if err := e.Transfer(ctx, sysCfg, tgt, false, 0, false); err != nil {
		return errors.Annotatef(err, "transfer from %s to %s failed", sysCfg, tgt)
	}
	systemdDir := "/etc/systemd/system/"
	sudo := true
	if opt.SystemdMode == UserMode {
		systemdDir = "~/.config/systemd/user/"
		sudo = false
	}
	cmd := fmt.Sprintf("mv %s %s%s-%d.service", tgt, systemdDir, comp, port)
	if _, _, err := e.Execute(ctx, cmd, sudo); err != nil {
		return errors.Annotatef(err, "execute: %s", cmd)
	}

	// doesn't work
	if _, err := i.setTLSConfig(ctx, false, nil, paths); err != nil {
		return err
	}

	return nil
}

// setTLSConfig set TLS Config to support enable/disable TLS
// baseInstance no need to configure TLS
func (i *BaseInstance) setTLSConfig(ctx context.Context, enableTLS bool, configs map[string]any, paths meta.DirPaths) (map[string]any, error) {
	return nil, nil
}

// TransferLocalConfigFile scp local config file to remote
// Precondition: the user on remote have permission to access & mkdir of dest files
func (i *BaseInstance) TransferLocalConfigFile(ctx context.Context, e ctxt.Executor, local, remote string) error {
	remoteDir := filepath.Dir(remote)
	// make sure the directory exists
	cmd := fmt.Sprintf("mkdir -p %s", remoteDir)
	if _, _, err := e.Execute(ctx, cmd, false); err != nil {
		return errors.Annotatef(err, "execute: %s", cmd)
	}

	if err := e.Transfer(ctx, local, remote, false, 0, false); err != nil {
		return errors.Annotatef(err, "transfer from %s to %s failed", local, remote)
	}

	return nil
}

// TransferLocalConfigDir scp local config directory to remote
// Precondition: the user on remote have right to access & mkdir of dest files
func (i *BaseInstance) TransferLocalConfigDir(ctx context.Context, e ctxt.Executor, local, remote string, filter func(string) bool) error {
	return i.IteratorLocalConfigDir(ctx, local, filter, func(fname string) error {
		localPath := path.Join(local, fname)
		remotePath := path.Join(remote, fname)
		if err := i.TransferLocalConfigFile(ctx, e, localPath, remotePath); err != nil {
			return errors.Annotatef(err, "transfer local config (%s -> %s) failed", localPath, remotePath)
		}
		return nil
	})
}

// IteratorLocalConfigDir iterators the local dir with filter, then invoke f for each found fileName
func (i *BaseInstance) IteratorLocalConfigDir(ctx context.Context, local string, filter func(string) bool, f func(string) error) error {
	files, err := os.ReadDir(local)
	if err != nil {
		return errors.Annotatef(err, "read local directory %s failed", local)
	}

	for _, file := range files {
		if filter != nil && !filter(file.Name()) {
			continue
		}
		if err := f(file.Name()); err != nil {
			return err
		}
	}

	return nil
}

// MergeServerConfig merges the server configuration and overwrite the global configuration
func (i *BaseInstance) MergeServerConfig(ctx context.Context, e ctxt.Executor, globalConf, instanceConf map[string]any, paths meta.DirPaths) error {
	fp := filepath.Join(paths.Cache, fmt.Sprintf("%s-%s-%d.toml", i.ComponentName(), i.GetHost(), i.GetPort()))
	conf, err := Merge2Toml(i.ComponentName(), globalConf, instanceConf)
	if err != nil {
		return err
	}
	err = utils.WriteFile(fp, conf, os.ModePerm)
	if err != nil {
		return err
	}
	dst := filepath.Join(paths.Deploy, "conf", fmt.Sprintf("%s.toml", i.ComponentName()))
	// transfer config
	return e.Transfer(ctx, fp, dst, false, 0, false)
}

// mergeTiFlashLearnerServerConfig merges the server configuration and overwrite the global configuration
func (i *BaseInstance) mergeTiFlashLearnerServerConfig(ctx context.Context, e ctxt.Executor, globalConf, instanceConf map[string]any, paths meta.DirPaths) error {
	fp := filepath.Join(paths.Cache, fmt.Sprintf("%s-learner-%s-%d.toml", i.ComponentName(), i.GetHost(), i.GetPort()))
	conf, err := Merge2Toml(i.ComponentName()+"-learner", globalConf, instanceConf)
	if err != nil {
		return err
	}
	err = utils.WriteFile(fp, conf, os.ModePerm)
	if err != nil {
		return err
	}
	dst := filepath.Join(paths.Deploy, "conf", fmt.Sprintf("%s-learner.toml", i.ComponentName()))
	// transfer config
	return e.Transfer(ctx, fp, dst, false, 0, false)
}

// ID returns the identifier of this instance, the ID is constructed by host:port
func (i *BaseInstance) ID() string {
	return utils.JoinHostPort(i.Host, i.Port)
}

// ComponentName implements Instance interface
func (i *BaseInstance) ComponentName() string {
	return i.Name
}

// ComponentSource implements Instance interface
func (i *BaseInstance) ComponentSource() string {
	if i.Source != "" {
		return i.Source
	} else if i.Component.Source() != "" {
		return i.Component.Source()
	}
	return i.ComponentName()
}

// InstanceName implements Instance interface
func (i *BaseInstance) InstanceName() string {
	if i.Port > 0 {
		return fmt.Sprintf("%s%d", i.Name, i.Port)
	}
	return i.ComponentName()
}

// ServiceName implements Instance interface
func (i *BaseInstance) ServiceName() string {
	var name string
	switch i.ComponentName() {
	case ComponentSpark, ComponentTiSpark:
		name = i.Role()
	default:
		name = i.Name
	}
	if i.Port > 0 {
		return fmt.Sprintf("%s-%d.service", name, i.Port)
	}
	return fmt.Sprintf("%s.service", name)
}

// GetHost implements Instance interface
func (i *BaseInstance) GetHost() string {
	return i.Host
}

// GetManageHost implements Instance interface
func (i *BaseInstance) GetManageHost() string {
	if i.ManageHost != "" {
		return i.ManageHost
	}
	return i.Host
}

// GetListenHost implements Instance interface
func (i *BaseInstance) GetListenHost() string {
	if i.ListenHost == "" {
		// ipv6 address
		if strings.Contains(i.Host, ":") {
			return "::"
		}
		return "0.0.0.0"
	}
	return i.ListenHost
}

// GetSSHPort implements Instance interface
func (i *BaseInstance) GetSSHPort() int {
	return i.SSHP
}

// GetNumaNode implements Instance interface
func (i *BaseInstance) GetNumaNode() string {
	return i.NumaNode
}

// GetNumaCores implements Instance interface
func (i *BaseInstance) GetNumaCores() string {
	return i.NumaCores
}

// DeployDir implements Instance interface
func (i *BaseInstance) DeployDir() string {
	return reflect.Indirect(reflect.ValueOf(i.InstanceSpec)).FieldByName("DeployDir").String()
}

// TLSDir implements Instance interface
func (i *BaseInstance) TLSDir() string {
	return i.DeployDir()
}

// DataDir implements Instance interface
func (i *BaseInstance) DataDir() string {
	dataDir := reflect.Indirect(reflect.ValueOf(i.InstanceSpec)).FieldByName("DataDir")
	if !dataDir.IsValid() {
		return ""
	}

	// the default data_dir is relative to deploy_dir
	if dataDir.String() != "" && !strings.HasPrefix(dataDir.String(), "/") {
		return filepath.Join(i.DeployDir(), dataDir.String())
	}

	return dataDir.String()
}

// LogDir implements Instance interface
func (i *BaseInstance) LogDir() string {
	logDir := ""

	field := reflect.Indirect(reflect.ValueOf(i.InstanceSpec)).FieldByName("LogDir")
	if field.IsValid() {
		logDir = field.Interface().(string)
	}

	if logDir == "" {
		logDir = "log"
	}
	if !strings.HasPrefix(logDir, "/") {
		logDir = filepath.Join(i.DeployDir(), logDir)
	}
	return logDir
}

// OS implements Instance interface
func (i *BaseInstance) OS() string {
	v := reflect.Indirect(reflect.ValueOf(i.InstanceSpec)).FieldByName("OS")
	if !v.IsValid() {
		return ""
	}
	return v.Interface().(string)
}

// Arch implements Instance interface
func (i *BaseInstance) Arch() string {
	v := reflect.Indirect(reflect.ValueOf(i.InstanceSpec)).FieldByName("Arch")
	if !v.IsValid() {
		return ""
	}
	return v.Interface().(string)
}

// IsPatched implements Instance interface
func (i *BaseInstance) IsPatched() bool {
	v := reflect.Indirect(reflect.ValueOf(i.InstanceSpec)).FieldByName("Patched")
	if !v.IsValid() {
		return false
	}
	return v.Bool()
}

// SetPatched implements the Instance interface
func (i *BaseInstance) SetPatched(p bool) {
	v := reflect.Indirect(reflect.ValueOf(i.InstanceSpec)).FieldByName("Patched")
	if !v.CanSet() {
		return
	}
	v.SetBool(p)
}

// CalculateVersion implements the Instance interface
func (i *BaseInstance) CalculateVersion(globalVersion string) string {
	return i.Component.CalculateVersion(globalVersion)
}

// PrepareStart checks instance requirements before starting
func (i *BaseInstance) PrepareStart(ctx context.Context, tlsCfg *tls.Config) error {
	return nil
}

// MergeResourceControl merge the rhs into lhs and overwrite rhs if lhs has value for same field
func MergeResourceControl(lhs, rhs meta.ResourceControl) meta.ResourceControl {
	if rhs.MemoryLimit != "" {
		lhs.MemoryLimit = rhs.MemoryLimit
	}
	if rhs.CPUQuota != "" {
		lhs.CPUQuota = rhs.CPUQuota
	}
	if rhs.IOReadBandwidthMax != "" {
		lhs.IOReadBandwidthMax = rhs.IOReadBandwidthMax
	}
	if rhs.IOWriteBandwidthMax != "" {
		lhs.IOWriteBandwidthMax = rhs.IOWriteBandwidthMax
	}
	if rhs.LimitCORE != "" {
		lhs.LimitCORE = rhs.LimitCORE
	}
	return lhs
}

// ResourceControl return cgroups config of instance
func (i *BaseInstance) ResourceControl() meta.ResourceControl {
	if v := reflect.Indirect(reflect.ValueOf(i.InstanceSpec)).FieldByName("ResourceControl"); v.IsValid() {
		return v.Interface().(meta.ResourceControl)
	}
	return meta.ResourceControl{}
}

// GetPort implements Instance interface
func (i *BaseInstance) GetPort() int {
	return i.Port
}

// UsedPorts implements Instance interface
func (i *BaseInstance) UsedPorts() []int {
	return i.Ports
}

// UsedDirs implements Instance interface
func (i *BaseInstance) UsedDirs() []string {
	return i.Dirs
}

// Status implements Instance interface
func (i *BaseInstance) Status(ctx context.Context, timeout time.Duration, tlsCfg *tls.Config, pdList ...string) string {
	return i.StatusFn(ctx, timeout, tlsCfg, pdList...)
}

// Uptime implements Instance interface
func (i *BaseInstance) Uptime(ctx context.Context, timeout time.Duration, tlsCfg *tls.Config) time.Duration {
	return i.UptimeFn(ctx, timeout, tlsCfg)
}
