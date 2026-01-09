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
	"io"
	"net/http"
	"path/filepath"
	"strings"
	"time"

	"github.com/pingcap/tiup/pkg/utils"
)

const (
	// ServiceTiKV is the service ID for TiKV.
	ServiceTiKV ServiceID = "tikv"

	// ComponentTiKV is the repository component ID for TiKV.
	ComponentTiKV RepoComponentID = "tikv"
)

// TiKVInstance represent a running tikv-server
type TiKVInstance struct {
	ProcessInfo
	ShOpt SharedOptions
	PDs   []*PDInstance
	TSOs  []*PDInstance
}

var _ Process = &TiKVInstance{}
var _ ReadyWaiter = &TiKVInstance{}

func init() {
	RegisterComponentDisplayName(ComponentTiKV, "TiKV")
	RegisterServiceDisplayName(ServiceTiKV, "TiKV")
}

// Addr return the address of tikv.
func (inst *TiKVInstance) Addr() string {
	return utils.JoinHostPort(inst.Host, inst.Port)
}

// Prepare builds the TiKV process command.
func (inst *TiKVInstance) Prepare(ctx context.Context) error {
	info := inst.Info()

	configPath := filepath.Join(inst.Dir, "tikv.toml")
	if err := prepareConfig(
		configPath,
		inst.ConfigPath,
		inst.getConfig(),
		nil,
	); err != nil {
		return err
	}

	endpoints := pdEndpoints(inst.PDs, true)
	args := []string{
		fmt.Sprintf("--addr=%s", utils.JoinHostPort(inst.Host, inst.Port)),
		fmt.Sprintf("--advertise-addr=%s", utils.JoinHostPort(AdvertiseHost(inst.Host), inst.Port)),
		fmt.Sprintf("--status-addr=%s", utils.JoinHostPort(inst.Host, inst.StatusPort)),
		fmt.Sprintf("--pd-endpoints=%s", strings.Join(endpoints, ",")),
		fmt.Sprintf("--config=%s", configPath),
		fmt.Sprintf("--data-dir=%s", filepath.Join(inst.Dir, "data")),
		fmt.Sprintf("--log-file=%s", inst.LogFile()),
	}

	envs := []string{"MALLOC_CONF=prof:true,prof_active:false"}
	info.Proc = &cmdProcess{cmd: PrepareCommand(ctx, inst.BinPath, args, envs, inst.Dir)}
	return nil
}

// WaitReady implements ReadyWaiter.
//
// In PD microservices mode, TiKV depends on a healthy TSO service.
// Historically playground performed this check in Prepare(), but Prepare()
// should only build the process command. Keep the behavior as a cancelable
// readiness step.
func (inst *TiKVInstance) WaitReady(ctx context.Context) error {
	if inst == nil || inst.ShOpt.PDMode != "ms" {
		return nil
	}

	ctx = withLogger(ctx)

	timeoutSec := inst.UpTimeout
	if timeoutSec <= 0 {
		timeoutSec = 300
	}
	ctx, cancel := withTimeoutSeconds(ctx, timeoutSec)
	defer cancel()

	var tsoAddrs []string
	for _, tso := range inst.TSOs {
		if tso == nil {
			continue
		}
		if addr := tso.Addr(); addr != "" {
			tsoAddrs = append(tsoAddrs, addr)
		}
	}
	if len(tsoAddrs) == 0 {
		return fmt.Errorf("no pd-tso instance available")
	}

	const pollInterval = 5 * time.Second
	ticker := time.NewTicker(pollInterval)
	defer ticker.Stop()

	client := &http.Client{}

	for {
		if err := checkTSOHealth(ctx, client, tsoAddrs); err == nil {
			return nil
		}

		select {
		case <-ctx.Done():
			err := ctx.Err()
			if err == context.DeadlineExceeded && timeoutSec > 0 {
				return readyTimeoutError(timeoutSec)
			}
			return err
		case <-ticker.C:
		}
	}
}

func checkTSOHealth(ctx context.Context, client *http.Client, tsoAddrs []string) error {
	if ctx == nil {
		ctx = context.Background()
	}
	if client == nil {
		client = &http.Client{}
	}
	if len(tsoAddrs) == 0 {
		return fmt.Errorf("no pd-tso endpoints")
	}

	for _, addr := range tsoAddrs {
		if addr == "" {
			return fmt.Errorf("empty pd-tso endpoint")
		}
		url := fmt.Sprintf("http://%s/tso/api/v1/health", addr)

		perAttempt := 5 * time.Second
		if deadline, ok := ctx.Deadline(); ok {
			remain := time.Until(deadline)
			if remain <= 0 {
				return context.DeadlineExceeded
			}
			if remain < perAttempt {
				perAttempt = remain
			}
		}

		attemptCtx, cancel := context.WithTimeout(ctx, perAttempt)
		req, err := http.NewRequestWithContext(attemptCtx, http.MethodGet, url, nil)
		if err != nil {
			cancel()
			return err
		}

		resp, err := client.Do(req)
		if resp != nil {
			_, _ = io.Copy(io.Discard, resp.Body)
			_ = resp.Body.Close()
		}
		cancel()

		if err != nil {
			return err
		}
		if resp == nil || resp.StatusCode != http.StatusOK {
			if resp != nil {
				return fmt.Errorf("pd-tso is not healthy (%s)", resp.Status)
			}
			return fmt.Errorf("pd-tso is not healthy")
		}
	}

	return nil
}

// LogFile return the log file name.
func (inst *TiKVInstance) LogFile() string {
	return filepath.Join(inst.Dir, "tikv.log")
}

// StoreAddr return the store address of TiKV
func (inst *TiKVInstance) StoreAddr() string {
	return utils.JoinHostPort(AdvertiseHost(inst.Host), inst.Port)
}
