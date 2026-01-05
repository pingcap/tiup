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
	"net/http"
	"path/filepath"
	"strings"
	"time"

	"github.com/pingcap/tiup/pkg/utils"
)

const (
	ServicePump ServiceID = "pump"

	ComponentPump RepoComponentID = "pump"
)

// Pump represent a pump instance.
type Pump struct {
	ProcessInfo
	PDs []*PDInstance
}

var _ Process = &Pump{}
var _ ReadyWaiter = &Pump{}

func init() {
	RegisterComponentDisplayName(ComponentPump, "Pump")
	RegisterServiceDisplayName(ServicePump, "Pump")
}

// Ready return nil when pump is ready to serve.
func (p *Pump) Ready(ctx context.Context) error {
	if ctx == nil {
		ctx = context.Background()
	}

	url := fmt.Sprintf("http://%s/status", utils.JoinHostPort(p.Host, p.Port))
	client := &http.Client{}

	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()

	for {
		perAttempt := 5 * time.Second
		if deadline, ok := ctx.Deadline(); ok {
			remain := time.Until(deadline)
			if remain <= 0 {
				return ctx.Err()
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
			_ = resp.Body.Close()
		}
		cancel()

		if err == nil && resp != nil && resp.StatusCode == http.StatusOK {
			return nil
		}

		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-ticker.C:
		}
	}
}

// WaitReady implements ReadyWaiter.
//
// Pump does not expose a richer "ready" probe, so we reuse the /status endpoint.
// Historically playground used a fixed 120s timeout for this step; keep the
// same default when UpTimeout is not configured.
func (p *Pump) WaitReady(ctx context.Context) error {
	ctx = withLogger(ctx)

	timeoutSec := p.UpTimeout
	if timeoutSec <= 0 {
		timeoutSec = 120
	}
	ctx, cancel := withTimeoutSeconds(ctx, timeoutSec)
	defer cancel()

	err := p.Ready(ctx)
	if err == context.DeadlineExceeded {
		return readyTimeoutError(timeoutSec)
	}
	return err
}

// Addr return the address of Pump.
func (p *Pump) Addr() string {
	return utils.JoinHostPort(AdvertiseHost(p.Host), p.Port)
}

// Prepare builds the Pump process command.
func (p *Pump) Prepare(ctx context.Context) error {
	info := p.Info()
	endpoints := pdEndpoints(p.PDs, true)

	args := []string{
		fmt.Sprintf("--node-id=%s", info.Name()),
		fmt.Sprintf("--addr=%s", utils.JoinHostPort(p.Host, p.Port)),
		fmt.Sprintf("--advertise-addr=%s", utils.JoinHostPort(AdvertiseHost(p.Host), p.Port)),
		fmt.Sprintf("--pd-urls=%s", strings.Join(endpoints, ",")),
		fmt.Sprintf("--log-file=%s", p.LogFile()),
	}
	if p.ConfigPath != "" {
		args = append(args, fmt.Sprintf("--config=%s", p.ConfigPath))
	}

	info.Proc = &cmdProcess{cmd: PrepareCommand(ctx, p.BinPath, args, nil, p.Dir)}
	return nil
}

// LogFile return the log file.
func (p *Pump) LogFile() string {
	return filepath.Join(p.Dir, "pump.log")
}
