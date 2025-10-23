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

package instance

import (
	"context"
	"fmt"
	"net"
	"os"
	"path/filepath"

	"github.com/BurntSushi/toml"
	"github.com/pingcap/errors"
	"github.com/pingcap/tiup/pkg/cluster/spec"
	tiupexec "github.com/pingcap/tiup/pkg/exec"
	"github.com/pingcap/tiup/pkg/tui/colorstr"
	"github.com/pingcap/tiup/pkg/utils"
)

// Config of the instance.
type Config struct {
	ConfigPath string `yaml:"config_path"`
	BinPath    string `yaml:"bin_path"`
	Num        int    `yaml:"num"`
	Host       string `yaml:"host"`
	Port       int    `yaml:"port"`
	UpTimeout  int    `yaml:"up_timeout"`
	Version    string `yaml:"version"`
}

// SharedOptions contains some commonly used, tunable options for most components.
// Unlike Config, these options are shared for all instances of all components.
type SharedOptions struct {
	/// Whether or not to tune the cluster in order to run faster (instead of easier to debug).
	HighPerf           bool       `yaml:"high_perf"`
	CSE                CSEOptions `yaml:"cse"` // Only available when mode == tidb-cse or tiflash-disagg
	PDMode             string     `yaml:"pd_mode"`
	Mode               string     `yaml:"mode"`
	PortOffset         int        `yaml:"port_offset"`
	EnableTiKVColumnar bool       `yaml:"enable_tikv_columnar"` // Only available when mode == tidb-cse
}

// CSEOptions contains configs to run TiDB cluster in CSE mode.
type CSEOptions struct {
	S3Endpoint string `yaml:"s3_endpoint"`
	Bucket     string `yaml:"bucket"`
	AccessKey  string `yaml:"access_key"`
	SecretKey  string `yaml:"secret_key"`
}

type instance struct {
	ID         int
	Dir        string
	Host       string
	Port       int
	StatusPort int // client port for PD
	ConfigPath string
	BinPath    string
	Version    utils.Version
	proc       Process
	role       string
}

// MetricAddr will be used by prometheus scrape_configs.
type MetricAddr struct {
	Targets []string          `json:"targets"`
	Labels  map[string]string `json:"labels"`
}

// Instance represent running component
type Instance interface {
	// Start the instance process.
	// Will kill the process once the context is done.
	Start(ctx context.Context) error
	// Name Return the display name.
	Name() string
	// Component returns the package name.
	Component() string
	// Role returns the role of the package.
	// It is used to start binaries differently for the same package.
	// For example, start PD in microservice mode, or start TiDB with another configuration.
	Role() string
	// LogFile return the log file name
	LogFile() string
	// MetricAddr return the address to pull metrics.
	MetricAddr() MetricAddr
	// Wait Should only call this if the instance is started successfully.
	// The implementation should be safe to call Wait multi times.
	Wait() error
	// Proc return the underlying process.
	Process() Process
	// PrepareBinary use given binpath or download from tiup mirrors.
	PrepareBinary(binaryName string, componentName string, version utils.Version) error
	// PrepareProcess construct the process used later.
	PrepareProcess(ctx context.Context, binPath string, args, envs []string, workDir string) error
}

func (inst *instance) Name() string {
	return fmt.Sprintf("%s-%d", inst.Role(), inst.ID)
}

func (inst *instance) Component() string {
	return inst.role
}

func (inst *instance) Role() string {
	return inst.role
}

func (inst *instance) Wait() error {
	return inst.proc.Wait()
}

func (inst *instance) PrepareProcess(ctx context.Context, binPath string, args, envs []string, workDir string) error {
	inst.proc = &process{cmd: PrepareCommand(ctx, binPath, args, envs, workDir)}
	return nil
}

func (inst *instance) Process() Process {
	return inst.proc
}

func (inst *instance) MetricAddr() (r MetricAddr) {
	if inst.Host != "" && inst.StatusPort != 0 {
		r.Targets = append(r.Targets, utils.JoinHostPort(inst.Host, inst.StatusPort))
	}
	return
}

func (inst *instance) PrepareBinary(binaryName string, componentName string, version utils.Version) error {
	instanceBinPath, err := tiupexec.PrepareBinary(binaryName, version, inst.BinPath)
	if err != nil {
		return err
	}
	// distinguish whether the instance is started by specific binary path.
	if inst.BinPath == "" {
		colorstr.Printf("[dark_gray]Start %s instance: %s[reset]\n", componentName, version)
	} else {
		colorstr.Printf("[dark_gray]Start %s instance: %s[reset]\n", componentName, instanceBinPath)
	}
	inst.Version = version
	inst.BinPath = instanceBinPath
	return nil
}

// CompVersion return the format to run specified version of a component.
func CompVersion(comp string, version utils.Version) string {
	if version.IsEmpty() {
		return comp
	}
	return fmt.Sprintf("%v:%v", comp, version)
}

// AdvertiseHost returns the interface's ip addr if listen host is 0.0.0.0
func AdvertiseHost(listen string) string {
	if listen == "0.0.0.0" {
		addrs, err := net.InterfaceAddrs()
		if err != nil || len(addrs) == 0 {
			return "localhost"
		}

		for _, addr := range addrs {
			if ip, ok := addr.(*net.IPNet); ok && !ip.IP.IsLoopback() && ip.IP.To4() != nil {
				return ip.IP.To4().String()
			}
		}
		return "localhost"
	}

	return listen
}

func pdEndpoints(pds []*PDInstance, isHTTP bool) []string {
	var endpoints []string
	for _, pd := range pds {
		if pd.Role() == PDRoleTSO || pd.Role() == PDRoleScheduling {
			continue
		}
		if isHTTP {
			endpoints = append(endpoints, "http://"+utils.JoinHostPort(AdvertiseHost(pd.Host), pd.StatusPort))
		} else {
			endpoints = append(endpoints, utils.JoinHostPort(AdvertiseHost(pd.Host), pd.StatusPort))
		}
	}
	return endpoints
}

// prepareConfig accepts a user specified config and merge user config with a
// pre-defined one.
func prepareConfig(outputConfigPath string, userConfigPath string, preDefinedConfig map[string]any) error {
	dir := filepath.Dir(outputConfigPath)
	if err := utils.MkdirAll(dir, 0755); err != nil {
		return err
	}

	userConfig, err := unmarshalConfig(userConfigPath)
	if err != nil {
		return errors.Trace(err)
	}
	if userConfig == nil {
		userConfig = make(map[string]any)
	}

	cf, err := os.Create(outputConfigPath)
	if err != nil {
		return errors.Trace(err)
	}

	enc := toml.NewEncoder(cf)
	enc.Indent = ""
	return enc.Encode(spec.MergeConfig(preDefinedConfig, userConfig))
}

func unmarshalConfig(path string) (map[string]any, error) {
	if path == "" {
		return nil, nil
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	c := make(map[string]any)
	err = toml.Unmarshal(data, &c)
	if err != nil {
		return nil, err
	}
	return c, nil
}
