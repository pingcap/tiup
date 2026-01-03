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

	"github.com/pingcap/errors"
	"github.com/pingcap/tiup/pkg/tidbver"
	"github.com/pingcap/tiup/pkg/utils"
)

const (
	ServicePD                ServiceID = "pd"
	ServicePDAPI             ServiceID = "pd-api"
	ServicePDTSO             ServiceID = "pd-tso"
	ServicePDScheduling      ServiceID = "pd-scheduling"
	ServicePDRouter          ServiceID = "pd-router"
	ServicePDResourceManager ServiceID = "pd-resource-manager"

	ComponentPD RepoComponentID = "pd"
)

func init() {
	RegisterComponentDisplayName(ComponentPD, "PD")
	RegisterServiceDisplayName(ServicePD, "PD")
	RegisterServiceDisplayName(ServicePDAPI, "PD API")
	RegisterServiceDisplayName(ServicePDTSO, "PD TSO")
	RegisterServiceDisplayName(ServicePDScheduling, "PD Scheduling")
	RegisterServiceDisplayName(ServicePDRouter, "PD Router")
	RegisterServiceDisplayName(ServicePDResourceManager, "PD Resource Manager")
}

// PDInstance represent a running pd-server
type PDInstance struct {
	ProcessInfo
	ShOpt             SharedOptions
	initEndpoints     []*PDInstance
	joinEndpoints     []*PDInstance
	PDs               []*PDInstance
	KVIsSingleReplica bool
}

var _ Process = &PDInstance{}

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

// Prepare builds the PD process command.
func (inst *PDInstance) Prepare(ctx context.Context) error {
	info := inst.Info()
	if inst.Service == "" {
		return errors.New("pd service is empty")
	}

	configPath := filepath.Join(inst.Dir, inst.Service.String()+".toml")
	if err := prepareConfig(
		configPath,
		inst.ConfigPath,
		inst.getConfig(),
		nil,
	); err != nil {
		return err
	}

	uid := info.Name()
	var args []string
	switch inst.Service {
	case ServicePD, ServicePDAPI:
		if inst.Service == ServicePDAPI {
			args = []string{"services", "api"}
		}
		args = append(args, []string{
			"--name=" + uid,
			fmt.Sprintf("--config=%s", configPath),
			fmt.Sprintf("--data-dir=%s", filepath.Join(inst.Dir, "data")),
			fmt.Sprintf("--peer-urls=http://%s", utils.JoinHostPort(inst.Host, inst.Port)),
			fmt.Sprintf("--advertise-peer-urls=http://%s", utils.JoinHostPort(AdvertiseHost(inst.Host), inst.Port)),
			fmt.Sprintf("--client-urls=http://%s", utils.JoinHostPort(inst.Host, inst.StatusPort)),
			fmt.Sprintf("--advertise-client-urls=http://%s", utils.JoinHostPort(AdvertiseHost(inst.Host), inst.StatusPort)),
			fmt.Sprintf("--log-file=%s", inst.LogFile()),
		}...)
		switch {
		case len(inst.initEndpoints) > 0:
			endpoints := make([]string, 0)
			for _, pd := range inst.initEndpoints {
				uid := pd.Info().Name()
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
	case ServicePDTSO, ServicePDScheduling, ServicePDRouter, ServicePDResourceManager:
		endpoints := pdEndpoints(inst.PDs, true)
		subservice := strings.TrimPrefix(inst.Service.String(), "pd-")
		args = []string{
			"services",
			subservice,
			fmt.Sprintf("--listen-addr=http://%s", utils.JoinHostPort(inst.Host, inst.StatusPort)),
			fmt.Sprintf("--advertise-listen-addr=http://%s", utils.JoinHostPort(AdvertiseHost(inst.Host), inst.StatusPort)),
			fmt.Sprintf("--backend-endpoints=%s", strings.Join(endpoints, ",")),
			fmt.Sprintf("--log-file=%s", inst.LogFile()),
			fmt.Sprintf("--config=%s", configPath),
		}
		if tidbver.PDSupportMicroservicesWithName(inst.Version.String()) {
			args = append(args, fmt.Sprintf("--name=%s", uid))
		}
	default:
		return errors.Errorf("unknown pd service %s", inst.Service)
	}

	info.Proc = &cmdProcess{cmd: PrepareCommand(ctx, inst.BinPath, args, nil, inst.Dir)}
	return nil
}

// LogFile return the log file.
func (inst *PDInstance) LogFile() string {
	if inst == nil || inst.Dir == "" {
		return ""
	}
	name := inst.Service.String()
	if name == "" {
		name = "pd"
	}
	return filepath.Join(inst.Dir, name+".log")
}

// Addr return the listen address of PD
func (inst *PDInstance) Addr() string {
	return utils.JoinHostPort(AdvertiseHost(inst.Host), inst.StatusPort)
}
