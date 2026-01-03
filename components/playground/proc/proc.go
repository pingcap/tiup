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

package proc

import (
	"context"
	"fmt"
	"net"
	"os"
	"path/filepath"

	"github.com/BurntSushi/toml"
	"github.com/pingcap/errors"
	"github.com/pingcap/tiup/pkg/cluster/spec"
	"github.com/pingcap/tiup/pkg/utils"
)

// Mode of playground
type Mode = string

var (
	// ModeNormal is the default mode.
	ModeNormal = "tidb"
	// ModeCSE is for CSE testing.
	ModeCSE = "tidb-cse"
	// ModeNextGen is for NG testing.
	ModeNextGen = "tidb-x"
	// ModeDisAgg is for tiflash testing.
	ModeDisAgg = "tiflash-disagg"
	// ModeTiKVSlim is for special tikv testing.
	ModeTiKVSlim = "tikv-slim"
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
	CSE                CSEOptions `yaml:"cse"` // Only available when mode == ModeCSE or ModeDisAgg
	PDMode             string     `yaml:"pd_mode"`
	Mode               string     `yaml:"mode"`
	PortOffset         int        `yaml:"port_offset"`
	EnableTiKVColumnar bool       `yaml:"enable_tikv_columnar"` // Only available when mode == ModeCSE
	ForcePull          bool       `yaml:"force_pull"`
}

// CSEOptions contains configs to run TiDB cluster in CSE mode.
type CSEOptions struct {
	S3Endpoint string `yaml:"s3_endpoint"`
	Bucket     string `yaml:"bucket"`
	AccessKey  string `yaml:"access_key"`
	SecretKey  string `yaml:"secret_key"`
}

// ProcessInfo holds the shared, low-level fields for a running playground
// instance.
//
// Concrete instances embed it and expose it via the Process.Info method.
type ProcessInfo struct {
	ID         int
	Dir        string
	Host       string
	Port       int
	StatusPort int // client port for PD
	// UpTimeout is the maximum wait time (in seconds) for the instance to become
	// ready.
	//
	// It is only used by components that implement readiness checks (e.g. TiDB,
	// TiProxy, TiFlash). A value <= 0 means no limit.
	UpTimeout  int
	ConfigPath string
	// UserBinPath is the binary path provided by the user (if any).
	//
	// It is treated as input and must not be overwritten by binary resolution.
	UserBinPath string
	// BinPath is the resolved executable path used to start the instance.
	//
	// It is set by the playground planner/preloader and may come from either a
	// user-provided path or a repository-installed component.
	BinPath         string
	Version         utils.Version
	Proc            OSProcess
	RepoComponentID RepoComponentID
	Service         ServiceID
}

func (info *ProcessInfo) Info() *ProcessInfo { return info }

// MetricAddr will be used by prometheus scrape_configs.
type MetricAddr struct {
	Targets []string          `json:"targets"`
	Labels  map[string]string `json:"labels"`
}

// Process represent running component
type Process interface {
	Info() *ProcessInfo

	// Prepare builds the process command (config + args) for later start.
	//
	// It does NOT start the underlying process. Playground owns the actual
	// proc.Start() call so it can consistently wire logs, waiters and readiness.
	Prepare(ctx context.Context) error
	// LogFile return the log file name
	LogFile() string
}

// Name returns the stable instance name used by the underlying component flags
// and topology.
func (info *ProcessInfo) Name() string {
	if info == nil {
		return ""
	}

	prefix := info.Service.String()
	if prefix == "" {
		prefix = info.RepoComponentID.String()
	}
	if prefix == "" {
		prefix = "instance"
	}
	return fmt.Sprintf("%s-%d", prefix, info.ID)
}

// DisplayName returns the user-facing display name for an instance (no index
// included).
func (info *ProcessInfo) DisplayName() string {
	if info == nil {
		return ""
	}
	if info.Service != "" {
		if s := ServiceDisplayName(info.Service); s != "" {
			return s
		}
	}
	return ComponentDisplayName(info.RepoComponentID)
}

// MetricAddr returns the default address to pull metrics.
//
// Specific instances can override it by defining their own MetricAddr method.
func (info *ProcessInfo) MetricAddr() (r MetricAddr) {
	if info == nil {
		return r
	}
	if info.Host != "" && info.StatusPort != 0 {
		r.Targets = append(r.Targets, utils.JoinHostPort(info.Host, info.StatusPort))
	}
	return r
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
		switch pd.Service {
		case ServicePDTSO, ServicePDScheduling, ServicePDRouter, ServicePDResourceManager:
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

// prepareConfig writes a merged config to outputConfigPath.
//
// Merge order:
// 1) preDefinedConfig provides defaults
// 2) userConfigPath overrides defaults
// 3) forceOverride overwrites both (for runtime-required fields)
func prepareConfig(outputConfigPath string, userConfigPath string, preDefinedConfig map[string]any, forceOverride map[string]any) (err error) {
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
	defer func() {
		cerr := cf.Close()
		if err == nil {
			err = cerr
		}
	}()

	enc := toml.NewEncoder(cf)
	enc.Indent = ""
	merged := spec.MergeConfig(preDefinedConfig, userConfig)
	if len(forceOverride) > 0 {
		merged = spec.MergeConfig(merged, forceOverride)
	}
	return enc.Encode(merged)
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
