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
}

// MetricAddr will be used by prometheus scrape_configs.
type MetricAddr struct {
	Targets []string          `json:"targets"`
	Labels  map[string]string `json:"labels"`
}

// Instance represent running component
type Instance interface {
	Pid() int
	// Start the instance process.
	// Will kill the process once the context is done.
	Start(ctx context.Context) error
	// Component Return the component name.
	Component() string
	// LogFile return the log file name
	LogFile() string
	// Uptime show uptime.
	Uptime() string
	// MetricAddr return the address to pull metrics.
	MetricAddr() MetricAddr
	// Wait Should only call this if the instance is started successfully.
	// The implementation should be safe to call Wait multi times.
	Wait() error
	// PrepareBinary use given binpath or download from tiup mirrors.
	PrepareBinary(componentName string, version utils.Version) error
}

func (inst *instance) MetricAddr() (r MetricAddr) {
	if inst.Host != "" && inst.StatusPort != 0 {
		r.Targets = append(r.Targets, utils.JoinHostPort(inst.Host, inst.StatusPort))
	}
	return
}

func (inst *instance) PrepareBinary(componentName string, version utils.Version) error {
	instanceBinPath, err := tiupexec.PrepareBinary(componentName, version, inst.BinPath)
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

func logIfErr(err error) {
	if err != nil {
		fmt.Println(err)
	}
}

func pdEndpoints(pds []*PDInstance, isHTTP bool) []string {
	var endpoints []string
	for _, pd := range pds {
		if pd.Role == PDRoleTSO || pd.Role == PDRoleScheduling {
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
