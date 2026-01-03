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
	"encoding/pem"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/pingcap/tiup/pkg/crypto"
	"github.com/pingcap/tiup/pkg/utils"
)

const (
	ServiceTiProxy ServiceID = "tiproxy"

	ComponentTiProxy RepoComponentID = "tiproxy"
)

// TiProxyInstance represent a ticdc instance.
type TiProxyInstance struct {
	ProcessInfo
	PDs []*PDInstance
}

var _ Process = &TiProxyInstance{}

func init() {
	RegisterComponentDisplayName(ComponentTiProxy, "TiProxy")
	RegisterServiceDisplayName(ServiceTiProxy, "TiProxy")
}

// GenTiProxySessionCerts will create a self-signed certs for TiProxy session migration. NOTE that this cert is directly used by TiDB.
func GenTiProxySessionCerts(dir string) error {
	if _, err := os.Stat(filepath.Join(dir, "tiproxy.crt")); err == nil {
		return nil
	}

	ca, err := crypto.NewCA("tiproxy")
	if err != nil {
		return err
	}
	privKey, err := crypto.NewKeyPair(crypto.KeyTypeRSA, crypto.KeySchemeRSASSAPSSSHA256)
	if err != nil {
		return err
	}
	csr, err := privKey.CSR("tiproxy", "tiproxy", nil, nil)
	if err != nil {
		return err
	}
	cert, err := ca.Sign(csr)
	if err != nil {
		return err
	}
	if err := utils.SaveFileWithBackup(filepath.Join(dir, "tiproxy.key"), privKey.Pem(), ""); err != nil {
		return err
	}
	return utils.SaveFileWithBackup(filepath.Join(dir, "tiproxy.crt"), pem.EncodeToMemory(&pem.Block{
		Type:  "CERTIFICATE",
		Bytes: cert,
	}), "")
}

// MetricAddr implements Process interface.
func (c *TiProxyInstance) MetricAddr() (r MetricAddr) {
	r.Targets = append(r.Targets, utils.JoinHostPort(c.Host, c.StatusPort))
	r.Labels = map[string]string{
		"__metrics_path__": "/api/metrics",
	}
	return
}

// Prepare builds the TiProxy process command.
func (c *TiProxyInstance) Prepare(ctx context.Context) error {
	info := c.Info()
	endpoints := pdEndpoints(c.PDs, false)

	configPath := filepath.Join(c.Dir, "config", "proxy.toml")
	if err := prepareConfig(configPath, c.ConfigPath, nil, map[string]any{
		"proxy.pd-addrs":        strings.Join(endpoints, ","),
		"proxy.addr":            utils.JoinHostPort(c.Host, c.Port),
		"proxy.advertise-addr":  AdvertiseHost(c.Host),
		"api.addr":              utils.JoinHostPort(c.Host, c.StatusPort),
		"log.log-file.filename": c.LogFile(),
	}); err != nil {
		return err
	}

	args := []string{
		fmt.Sprintf("--config=%s", configPath),
	}

	info.Proc = &cmdProcess{cmd: PrepareCommand(ctx, c.BinPath, args, nil, c.Dir)}
	return nil
}

// Addr return addresses that can be connected by MySQL clients.
func (c *TiProxyInstance) Addr() string {
	return utils.JoinHostPort(AdvertiseHost(c.Host), c.Port)
}

// WaitReady implements ReadyWaiter.
//
// TiProxy is considered ready when its MySQL TCP port is connectable.
func (c *TiProxyInstance) WaitReady(ctx context.Context) error {
	return tcpAddrReady(ctx, c.Addr(), c.UpTimeout)
}

// LogFile return the log file.
func (c *TiProxyInstance) LogFile() string {
	return filepath.Join(c.Dir, "tiproxy.log")
}
