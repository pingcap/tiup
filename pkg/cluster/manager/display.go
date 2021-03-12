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

package manager

import (
	"errors"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/fatih/color"
	perrs "github.com/pingcap/errors"
	"github.com/pingcap/tiup/pkg/cliutil"
	"github.com/pingcap/tiup/pkg/cluster/api"
	operator "github.com/pingcap/tiup/pkg/cluster/operation"
	"github.com/pingcap/tiup/pkg/cluster/spec"
	"github.com/pingcap/tiup/pkg/cluster/task"
	"github.com/pingcap/tiup/pkg/logger/log"
	"github.com/pingcap/tiup/pkg/meta"
	"github.com/pingcap/tiup/pkg/set"
	"github.com/pingcap/tiup/pkg/utils"
)

// Display cluster meta and topology.
func (m *Manager) Display(name string, opt operator.Options) error {
	metadata, err := m.meta(name)
	if err != nil && !errors.Is(perrs.Cause(err), meta.ErrValidate) &&
		!errors.Is(perrs.Cause(err), spec.ErrNoTiSparkMaster) {
		return err
	}

	topo := metadata.GetTopology()
	base := metadata.GetBaseMeta()
	// display cluster meta
	cyan := color.New(color.FgCyan, color.Bold)
	fmt.Printf("Cluster type:       %s\n", cyan.Sprint(m.sysName))
	fmt.Printf("Cluster name:       %s\n", cyan.Sprint(name))
	fmt.Printf("Cluster version:    %s\n", cyan.Sprint(base.Version))
	fmt.Printf("SSH type:           %s\n", cyan.Sprint(topo.BaseTopo().GlobalOptions.SSHType))

	// display TLS info
	if topo.BaseTopo().GlobalOptions.TLSEnabled {
		fmt.Printf("TLS encryption:  	%s\n", cyan.Sprint("enabled"))
		fmt.Printf("CA certificate:     %s\n", cyan.Sprint(
			m.specManager.Path(name, spec.TLSCertKeyDir, spec.TLSCACert),
		))
		fmt.Printf("Client private key: %s\n", cyan.Sprint(
			m.specManager.Path(name, spec.TLSCertKeyDir, spec.TLSClientKey),
		))
		fmt.Printf("Client certificate: %s\n", cyan.Sprint(
			m.specManager.Path(name, spec.TLSCertKeyDir, spec.TLSClientCert),
		))
	}

	// display topology
	clusterTable := [][]string{
		// Header
		{"ID", "Role", "Host", "Ports", "OS/Arch", "Status", "Data Dir", "Deploy Dir"},
	}

	ctx := task.NewContext()
	err = ctx.SetSSHKeySet(m.specManager.Path(name, "ssh", "id_rsa"), m.specManager.Path(name, "ssh", "id_rsa.pub"))
	if err != nil {
		return err
	}

	err = ctx.SetClusterSSH(topo, base.User, opt.SSHTimeout, opt.SSHType, topo.BaseTopo().GlobalOptions.SSHType)
	if err != nil {
		return err
	}

	filterRoles := set.NewStringSet(opt.Roles...)
	filterNodes := set.NewStringSet(opt.Nodes...)
	masterList := topo.BaseTopo().MasterList
	tlsCfg, err := topo.TLSConfig(m.specManager.Path(name, spec.TLSCertKeyDir))
	if err != nil {
		return err
	}

	masterActive := make([]string, 0)
	masterStatus := make(map[string]string)

	topo.IterInstance(func(ins spec.Instance) {
		if ins.ComponentName() != spec.ComponentPD && ins.ComponentName() != spec.ComponentDMMaster {
			return
		}
		status := ins.Status(tlsCfg, masterList...)
		if strings.HasPrefix(status, "Up") || strings.HasPrefix(status, "Healthy") {
			instAddr := fmt.Sprintf("%s:%d", ins.GetHost(), ins.GetPort())
			masterActive = append(masterActive, instAddr)
		}
		masterStatus[ins.ID()] = status
	})

	var dashboardAddr string
	if t, ok := topo.(*spec.Specification); ok {
		var err error
		dashboardAddr, err = t.GetDashboardAddress(tlsCfg, masterActive...)
		if dashboardAddr != "" && err == nil {
			scheme := "http"
			if tlsCfg != nil {
				scheme = "https"
			}
			fmt.Printf("Dashboard URL:      %s\n", cyan.Sprintf("%s://%s/dashboard", scheme, dashboardAddr))
		}
	}

	topo.IterInstance(func(ins spec.Instance) {
		// apply role filter
		if len(filterRoles) > 0 && !filterRoles.Exist(ins.Role()) {
			return
		}
		// apply node filter
		if len(filterNodes) > 0 && !filterNodes.Exist(ins.ID()) {
			return
		}

		dataDir := "-"
		insDirs := ins.UsedDirs()
		deployDir := insDirs[0]
		if len(insDirs) > 1 {
			dataDir = insDirs[1]
		}

		var status string
		switch ins.ComponentName() {
		case spec.ComponentPD:
			status = masterStatus[ins.ID()]
			instAddr := fmt.Sprintf("%s:%d", ins.GetHost(), ins.GetPort())
			if dashboardAddr == instAddr {
				status += "|UI"
			}
		case spec.ComponentDMMaster:
			status = masterStatus[ins.ID()]
		default:
			status = ins.Status(tlsCfg, masterActive...)
		}

		// Query the service status
		if status == "-" {
			e, found := ctx.GetExecutor(ins.GetHost())
			if found {
				active, _ := operator.GetServiceStatus(e, ins.ServiceName())
				if parts := strings.Split(strings.TrimSpace(active), " "); len(parts) > 2 {
					if parts[1] == "active" {
						status = "Up"
					} else {
						status = parts[1]
					}
				}
			}
		}
		clusterTable = append(clusterTable, []string{
			color.CyanString(ins.ID()),
			ins.Role(),
			ins.GetHost(),
			utils.JoinInt(ins.UsedPorts(), "/"),
			cliutil.OsArch(ins.OS(), ins.Arch()),
			formatInstanceStatus(status),
			dataDir,
			deployDir,
		})
	})

	// Sort by role,host,ports
	sort.Slice(clusterTable[1:], func(i, j int) bool {
		lhs, rhs := clusterTable[i+1], clusterTable[j+1]
		// column: 1 => role, 2 => host, 3 => ports
		for _, col := range []int{1, 2} {
			if lhs[col] != rhs[col] {
				return lhs[col] < rhs[col]
			}
		}
		return lhs[3] < rhs[3]
	})

	cliutil.PrintTable(clusterTable, true)
	fmt.Printf("Total nodes: %d\n", len(clusterTable)-1)

	if t, ok := topo.(*spec.Specification); ok {
		// Check if TiKV's label set correctly
		pdClient := api.NewPDClient(masterActive, 10*time.Second, tlsCfg)
		if lbs, err := pdClient.GetLocationLabels(); err != nil {
			log.Debugf("get location labels from pd failed: %v", err)
		} else if err := spec.CheckTiKVLabels(lbs, pdClient); err != nil {
			color.Yellow("\nWARN: there is something wrong with TiKV labels, which may cause data losing:\n%v", err)
		}

		// Check if there is some instance in tombstone state
		nodes, _ := operator.DestroyTombstone(ctx, t, true /* returnNodesOnly */, opt, tlsCfg)
		if len(nodes) != 0 {
			color.Green("There are some nodes can be pruned: \n\tNodes: %+v\n\tYou can destroy them with the command: `tiup cluster prune %s`", nodes, name)
		}
	}

	return nil
}

func formatInstanceStatus(status string) string {
	lowercaseStatus := strings.ToLower(status)

	startsWith := func(prefixs ...string) bool {
		for _, prefix := range prefixs {
			if strings.HasPrefix(lowercaseStatus, prefix) {
				return true
			}
		}
		return false
	}

	switch {
	case startsWith("up|l", "healthy|l"): // up|l, up|l|ui, healthy|l
		return color.HiGreenString(status)
	case startsWith("up", "healthy", "free"):
		return color.GreenString(status)
	case startsWith("down", "err"): // down, down|ui
		return color.RedString(status)
	case startsWith("tombstone", "disconnected", "n/a"), strings.Contains(status, "offline"):
		return color.YellowString(status)
	default:
		return status
	}
}
