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
	"path/filepath"
	"strings"

	"github.com/pingcap/errors"
	tiupexec "github.com/pingcap/tiup/pkg/exec"
	"github.com/pingcap/tiup/pkg/utils"
)

// PDRole is the role of PD.
type PDRole string

const (
	// PDRoleNormal is the default role of PD
	PDRoleNormal PDRole = "pd"
	// PDRoleAPI is the role of PD API
	PDRoleAPI PDRole = "api"
	// PDRoleTSO is the role of PD TSO
	PDRoleTSO PDRole = "tso"
	// PDRoleResourceManager is the role of PD resource manager
	PDRoleResourceManager PDRole = "resource manager"
)

// PDInstance represent a running pd-server
type PDInstance struct {
	instance
	Role          PDRole
	initEndpoints []*PDInstance
	joinEndpoints []*PDInstance
	pds           []*PDInstance
	Process
}

// NewPDInstance return a PDInstance
func NewPDInstance(role PDRole, binPath, dir, host, configPath string, id int, pds []*PDInstance, port int) *PDInstance {
	if port <= 0 {
		port = 2379
	}
	return &PDInstance{
		instance: instance{
			BinPath:    binPath,
			ID:         id,
			Dir:        dir,
			Host:       host,
			Port:       utils.MustGetFreePort(host, 2380),
			StatusPort: utils.MustGetFreePort(host, port),
			ConfigPath: configPath,
		},
		Role: role,
		pds:  pds,
	}
}

// Join set endpoints field of PDInstance
func (inst *PDInstance) Join(pds []*PDInstance) *PDInstance {
	inst.joinEndpoints = pds
	return inst
}

// InitCluster set the init cluster instance.
func (inst *PDInstance) InitCluster(pds []*PDInstance) *PDInstance {
	inst.initEndpoints = pds
	return inst
}

// Name return the name of pd.
func (inst *PDInstance) Name() string {
	return fmt.Sprintf("pd-%d", inst.ID)
}

// Start calls set inst.cmd and Start
func (inst *PDInstance) Start(ctx context.Context, version utils.Version) error {
	uid := inst.Name()
	var args []string
	switch inst.Role {
	case PDRoleNormal, PDRoleAPI:
		if inst.Role == PDRoleAPI {
			args = []string{"services", "api"}
		}
		args = append(args, []string{
			"--name=" + uid,
			fmt.Sprintf("--data-dir=%s", filepath.Join(inst.Dir, "data")),
			fmt.Sprintf("--peer-urls=http://%s", utils.JoinHostPort(inst.Host, inst.Port)),
			fmt.Sprintf("--advertise-peer-urls=http://%s", utils.JoinHostPort(AdvertiseHost(inst.Host), inst.Port)),
			fmt.Sprintf("--client-urls=http://%s", utils.JoinHostPort(inst.Host, inst.StatusPort)),
			fmt.Sprintf("--advertise-client-urls=http://%s", utils.JoinHostPort(AdvertiseHost(inst.Host), inst.StatusPort)),
			fmt.Sprintf("--log-file=%s", inst.LogFile()),
		}...)
		if inst.ConfigPath != "" {
			args = append(args, fmt.Sprintf("--config=%s", inst.ConfigPath))
		}
		switch {
		case len(inst.initEndpoints) > 0:
			endpoints := make([]string, 0)
			for _, pd := range inst.initEndpoints {
				uid := fmt.Sprintf("pd-%d", pd.ID)
				endpoints = append(endpoints, fmt.Sprintf("%s=http://%s", uid, utils.JoinHostPort(AdvertiseHost(inst.Host), pd.Port)))
			}
			args = append(args, fmt.Sprintf("--initial-cluster=%s", strings.Join(endpoints, ",")))
		case len(inst.joinEndpoints) > 0:
			endpoints := make([]string, 0)
			for _, pd := range inst.joinEndpoints {
				endpoints = append(endpoints, fmt.Sprintf("http://%s", utils.JoinHostPort(AdvertiseHost(inst.Host), pd.Port)))
			}
			args = append(args, fmt.Sprintf("--join=%s", strings.Join(endpoints, ",")))
		default:
			return errors.Errorf("must set the init or join instances")
		}
	case PDRoleTSO:
		endpoints := pdEndpoints(inst.pds, true)
		args = []string{
			"services",
			"tso",
			fmt.Sprintf("--listen-addr=http://%s", utils.JoinHostPort(inst.Host, inst.Port)),
			fmt.Sprintf("--advertise-listen-addr=http://%s", utils.JoinHostPort(AdvertiseHost(inst.Host), inst.Port)),
			fmt.Sprintf("--backend-endpoints=%s", strings.Join(endpoints, ",")),
			fmt.Sprintf("--log-file=%s", inst.LogFile()),
		}
		if inst.ConfigPath != "" {
			args = append(args, fmt.Sprintf("--config=%s", inst.ConfigPath))
		}
	case PDRoleResourceManager:
		endpoints := pdEndpoints(inst.pds, true)
		args = []string{
			"services",
			"resource-manager",
			fmt.Sprintf("--listen-addr=http://%s", utils.JoinHostPort(inst.Host, inst.Port)),
			fmt.Sprintf("--advertise-listen-addr=http://%s", utils.JoinHostPort(AdvertiseHost(inst.Host), inst.Port)),
			fmt.Sprintf("--backend-endpoints=%s", strings.Join(endpoints, ",")),
			fmt.Sprintf("--log-file=%s", inst.LogFile()),
		}
		if inst.ConfigPath != "" {
			args = append(args, fmt.Sprintf("--config=%s", inst.ConfigPath))
		}
	}

	var err error
	if inst.BinPath, err = tiupexec.PrepareBinary("pd", version, inst.BinPath); err != nil {
		return err
	}
	inst.Process = &process{cmd: PrepareCommand(ctx, inst.BinPath, args, nil, inst.Dir)}

	logIfErr(inst.Process.SetOutputFile(inst.LogFile()))
	return inst.Process.Start()
}

// Component return the component name.
func (inst *PDInstance) Component() string {
	if inst.Role == PDRoleNormal {
		return "pd"
	}
	return fmt.Sprintf("pd %s", inst.Role)
}

// LogFile return the log file.
func (inst *PDInstance) LogFile() string {
	return filepath.Join(inst.Dir, fmt.Sprintf("%s.log", string(inst.Role)))
}

// Addr return the listen address of PD
func (inst *PDInstance) Addr() string {
	return utils.JoinHostPort(AdvertiseHost(inst.Host), inst.StatusPort)
}
