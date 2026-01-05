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
	"path/filepath"
	"strings"
	"time"

	"github.com/pingcap/errors"
	"github.com/pingcap/tiup/pkg/cluster/api"
	"github.com/pingcap/tiup/pkg/tidbver"
	"github.com/pingcap/tiup/pkg/utils"
)

const (
	ServiceTiFlash        ServiceID = "tiflash"
	ServiceTiFlashWrite   ServiceID = "tiflash-write"
	ServiceTiFlashCompute ServiceID = "tiflash-compute"

	ComponentTiFlash RepoComponentID = "tiflash"
)

func init() {
	RegisterComponentDisplayName(ComponentTiFlash, "TiFlash")
	RegisterServiceDisplayName(ServiceTiFlash, "TiFlash")
	RegisterServiceDisplayName(ServiceTiFlashCompute, "TiFlash CN")
	RegisterServiceDisplayName(ServiceTiFlashWrite, "TiFlash WN")
}

// TiFlashInstance represent a running TiFlash
type TiFlashInstance struct {
	ProcessInfo
	ShOpt           SharedOptions
	TCPPort         int
	ServicePort     int
	ProxyPort       int
	ProxyStatusPort int
	PDs             []*PDInstance
}

var _ Process = &TiFlashInstance{}

// Addr return the address of tiflash
func (inst *TiFlashInstance) Addr() string {
	return utils.JoinHostPort(AdvertiseHost(inst.Host), inst.ServicePort)
}

// WaitReady implements ReadyWaiter.
//
// TiFlash is considered ready once it is reported as "up" by PD.
func (inst *TiFlashInstance) WaitReady(ctx context.Context) error {
	ctx = withLogger(ctx)
	ctx, cancel := withTimeoutSeconds(ctx, inst.UpTimeout)
	defer cancel()

	endpoints := pdEndpoints(inst.PDs, false)
	pdClient := api.NewPDClient(ctx, endpoints, 10*time.Second, nil)

	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()

	for {
		proc := inst.Info().Proc
		if proc == nil {
			return fmt.Errorf("initialize command failed")
		}
		cmd := proc.Cmd()
		if cmd == nil {
			return fmt.Errorf("initialize command failed")
		}
		if state := cmd.ProcessState; state != nil && state.Exited() {
			return fmt.Errorf("process exited with code: %d", state.ExitCode())
		}

		if up, err := pdClient.IsUp(inst.Addr()); err == nil && up {
			return nil
		}

		select {
		case <-ctx.Done():
			err := ctx.Err()
			if err == context.DeadlineExceeded && inst.UpTimeout > 0 {
				return readyTimeoutError(inst.UpTimeout)
			}
			return err
		case <-ticker.C:
		}
	}
}

// MetricAddr returns the address(es) to pull metrics.
func (inst *TiFlashInstance) MetricAddr() (r MetricAddr) {
	r.Targets = append(r.Targets, utils.JoinHostPort(inst.Host, inst.StatusPort))
	r.Targets = append(r.Targets, utils.JoinHostPort(inst.Host, inst.ProxyStatusPort))
	return
}

// Prepare builds the TiFlash process command.
func (inst *TiFlashInstance) Prepare(ctx context.Context) error {
	info := inst.Info()
	if v := inst.Version.String(); v != "" && !tidbver.TiFlashPlaygroundNewStartMode(v) {
		return fmt.Errorf("tiflash is only supported in TiDB >= v7.1.0 (or nightly), got %s", v)
	}

	proxyConfigPath := filepath.Join(inst.Dir, "tiflash_proxy.toml")
	if err := prepareConfig(
		proxyConfigPath,
		"",
		inst.getProxyConfig(),
		nil,
	); err != nil {
		return err
	}

	configPath := filepath.Join(inst.Dir, "tiflash.toml")
	if err := prepareConfig(
		configPath,
		inst.ConfigPath,
		inst.getConfig(),
		nil,
	); err != nil {
		return err
	}

	endpoints := pdEndpoints(inst.PDs, false)

	args := []string{
		"server",
		fmt.Sprintf("--config-file=%s", configPath),
		"--",
	}
	runtimeConfig := [][]string{
		{"path", filepath.Join(inst.Dir, "data")},
		{"listen_host", inst.Host},
		{"logger.log", inst.LogFile()},
		{"logger.errorlog", filepath.Join(inst.Dir, "tiflash_error.log")},
		{"status.metrics_port", fmt.Sprintf("%d", inst.StatusPort)},
		{"flash.service_addr", utils.JoinHostPort(AdvertiseHost(inst.Host), inst.ServicePort)},
		{"raft.pd_addr", strings.Join(endpoints, ",")},
		{"flash.proxy.addr", utils.JoinHostPort(inst.Host, inst.ProxyPort)},
		{"flash.proxy.advertise-addr", utils.JoinHostPort(AdvertiseHost(inst.Host), inst.ProxyPort)},
		{"flash.proxy.status-addr", utils.JoinHostPort(inst.Host, inst.ProxyStatusPort)},
		{"flash.proxy.data-dir", filepath.Join(inst.Dir, "proxy_data")},
		{"flash.proxy.log-file", filepath.Join(inst.Dir, "tiflash_tikv.log")},
	}
	userConfig, err := unmarshalConfig(configPath)
	if err != nil {
		return errors.Trace(err)
	}
	for _, arg := range runtimeConfig {
		// if user has set the config, skip it
		if !isKeyPresentInMap(userConfig, arg[0]) {
			args = append(args, fmt.Sprintf("--%s=%s", arg[0], arg[1]))
		}
	}

	info.Proc = &cmdProcess{cmd: PrepareCommand(ctx, inst.BinPath, args, nil, inst.Dir)}
	return nil
}

func isKeyPresentInMap(m map[string]any, key string) bool {
	if len(m) == 0 || key == "" {
		return false
	}

	keys := strings.Split(key, ".")
	var current any = m
	for i, k := range keys {
		currentMap, ok := current.(map[string]any)
		if !ok {
			return false
		}

		v, ok := currentMap[k]
		if !ok {
			return false
		}
		if i == len(keys)-1 {
			return true
		}
		current = v
	}

	return true
}

// LogFile return the log file name.
func (inst *TiFlashInstance) LogFile() string {
	return filepath.Join(inst.Dir, "tiflash.log")
}

// StoreAddr return the store address of TiFlash
func (inst *TiFlashInstance) StoreAddr() string {
	return utils.JoinHostPort(AdvertiseHost(inst.Host), inst.ServicePort)
}
