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
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"sort"
	"strings"
	"time"

	"github.com/fatih/color"
	perrs "github.com/pingcap/errors"
	"github.com/pingcap/tiup/pkg/cluster/api"
	"github.com/pingcap/tiup/pkg/cluster/clusterutil"
	"github.com/pingcap/tiup/pkg/cluster/ctxt"
	"github.com/pingcap/tiup/pkg/cluster/executor"
	operator "github.com/pingcap/tiup/pkg/cluster/operation"
	"github.com/pingcap/tiup/pkg/cluster/spec"
	"github.com/pingcap/tiup/pkg/logger/log"
	"github.com/pingcap/tiup/pkg/meta"
	"github.com/pingcap/tiup/pkg/set"
	"github.com/pingcap/tiup/pkg/tui"
	"github.com/pingcap/tiup/pkg/utils"
)

// InstInfo represents an instance info
type InstInfo struct {
	ID        string `json:"id"`
	Role      string `json:"role"`
	Host      string `json:"host"`
	Ports     string `json:"ports"`
	OsArch    string `json:"os_arch"`
	Status    string `json:"status"`
	Since     string `json:"since"`
	DataDir   string `json:"data_dir"`
	DeployDir string `json:"deploy_dir"`

	ComponentName string
	Port          int
}

// ClusterMetaInfo hold the structure for the JSON output of the dashboard info
type ClusterMetaInfo struct {
	ClusterType    string `json:"cluster_type"`
	ClusterName    string `json:"cluster_name"`
	ClusterVersion string `json:"cluster_version"`
	DeployUser     string `json:"deploy_user"`
	SSHType        string `json:"ssh_type"`
	TLSEnabled     bool   `json:"tls_enabled"`
	TLSCACert      string `json:"tls_ca_cert,omitempty"`
	TLSClientCert  string `json:"tls_client_cert,omitempty"`
	TLSClientKey   string `json:"tls_client_key,omitempty"`
	DashboardURL   string `json:"dashboard_url,omitempty"`
}

// JSONOutput holds the structure for the JSON output of `tiup cluster display --json`
type JSONOutput struct {
	ClusterMetaInfo ClusterMetaInfo `json:"cluster_meta"`
	InstanceInfos   []InstInfo      `json:"instances"`
}

// Display cluster meta and topology.
func (m *Manager) Display(name string, opt operator.Options) error {
	if err := clusterutil.ValidateClusterNameOrError(name); err != nil {
		return err
	}

	clusterInstInfos, err := m.GetClusterTopology(name, opt)
	if err != nil {
		return err
	}

	metadata, _ := m.meta(name)
	topo := metadata.GetTopology()
	base := metadata.GetBaseMeta()
	cyan := color.New(color.FgCyan, color.Bold)
	// display cluster meta
	var j *JSONOutput
	if opt.JSON {
		j = &JSONOutput{
			ClusterMetaInfo: ClusterMetaInfo{
				m.sysName,
				name,
				base.Version,
				topo.BaseTopo().GlobalOptions.User,
				string(topo.BaseTopo().GlobalOptions.SSHType),
				topo.BaseTopo().GlobalOptions.TLSEnabled,
				"", // CA Cert
				"", // Client Cert
				"", // Client Key
				"",
			},
			InstanceInfos: clusterInstInfos,
		}

		if topo.BaseTopo().GlobalOptions.TLSEnabled {
			j.ClusterMetaInfo.TLSCACert = m.specManager.Path(name, spec.TLSCertKeyDir, spec.TLSCACert)
			j.ClusterMetaInfo.TLSClientKey = m.specManager.Path(name, spec.TLSCertKeyDir, spec.TLSClientKey)
			j.ClusterMetaInfo.TLSClientCert = m.specManager.Path(name, spec.TLSCertKeyDir, spec.TLSClientCert)
		}
	} else {
		fmt.Printf("Cluster type:       %s\n", cyan.Sprint(m.sysName))
		fmt.Printf("Cluster name:       %s\n", cyan.Sprint(name))
		fmt.Printf("Cluster version:    %s\n", cyan.Sprint(base.Version))
		fmt.Printf("Deploy user:        %s\n", cyan.Sprint(topo.BaseTopo().GlobalOptions.User))
		fmt.Printf("SSH type:           %s\n", cyan.Sprint(topo.BaseTopo().GlobalOptions.SSHType))

		// display TLS info
		if topo.BaseTopo().GlobalOptions.TLSEnabled {
			fmt.Printf("TLS encryption:     %s\n", cyan.Sprint("enabled"))
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
	}

	// display topology
	var clusterTable [][]string
	if opt.ShowUptime {
		clusterTable = append(clusterTable, []string{"ID", "Role", "Host", "Ports", "OS/Arch", "Status", "Since", "Data Dir", "Deploy Dir"})
	} else {
		clusterTable = append(clusterTable, []string{"ID", "Role", "Host", "Ports", "OS/Arch", "Status", "Data Dir", "Deploy Dir"})
	}

	masterActive := make([]string, 0)
	for _, v := range clusterInstInfos {
		row := []string{
			color.CyanString(v.ID),
			v.Role,
			v.Host,
			v.Ports,
			v.OsArch,
			formatInstanceStatus(v.Status),
		}
		if opt.ShowUptime {
			row = append(row, v.Since)
		}
		row = append(row, v.DataDir, v.DeployDir)
		clusterTable = append(clusterTable, row)

		if v.ComponentName != spec.ComponentPD && v.ComponentName != spec.ComponentDMMaster {
			continue
		}
		if strings.HasPrefix(v.Status, "Up") || strings.HasPrefix(v.Status, "Healthy") {
			instAddr := fmt.Sprintf("%s:%d", v.Host, v.Port)
			masterActive = append(masterActive, instAddr)
		}
	}

	tlsCfg, err := topo.TLSConfig(m.specManager.Path(name, spec.TLSCertKeyDir))
	if err != nil {
		return err
	}

	var dashboardAddr string
	if t, ok := topo.(*spec.Specification); ok {
		var err error
		dashboardAddr, err = t.GetDashboardAddress(tlsCfg, masterActive...)
		if err == nil && !set.NewStringSet("", "auto", "none").Exist(dashboardAddr) {
			scheme := "http"
			if tlsCfg != nil {
				scheme = "https"
			}
			if opt.JSON {
				j.ClusterMetaInfo.DashboardURL = fmt.Sprintf("%s://%s/dashboard", scheme, dashboardAddr)
			} else {
				fmt.Printf("Dashboard URL:      %s\n", cyan.Sprintf("%s://%s/dashboard", scheme, dashboardAddr))
			}
		}
	}

	if opt.JSON {
		d, err := json.MarshalIndent(j, "", "  ")
		if err != nil {
			return err
		}
		fmt.Println(string(d))
		return nil
	}

	tui.PrintTable(clusterTable, true)
	fmt.Printf("Total nodes: %d\n", len(clusterTable)-1)

	ctx := ctxt.New(context.Background(), opt.Concurrency)
	if t, ok := topo.(*spec.Specification); ok {
		// Check if TiKV's label set correctly
		pdClient := api.NewPDClient(masterActive, 10*time.Second, tlsCfg)

		if lbs, placementRule, err := pdClient.GetLocationLabels(); err != nil {
			log.Debugf("get location labels from pd failed: %v", err)
		} else if !placementRule {
			if err := spec.CheckTiKVLabels(lbs, pdClient); err != nil {
				color.Yellow("\nWARN: there is something wrong with TiKV labels, which may cause data losing:\n%v", err)
			}
		}

		// Check if there is some instance in tombstone state
		nodes, _ := operator.DestroyTombstone(ctx, t, true /* returnNodesOnly */, opt, tlsCfg)
		if len(nodes) != 0 {
			color.Green("There are some nodes can be pruned: \n\tNodes: %+v\n\tYou can destroy them with the command: `tiup cluster prune %s`", nodes, name)
		}
	}

	return nil
}

// GetClusterTopology get the topology of the cluster.
func (m *Manager) GetClusterTopology(name string, opt operator.Options) ([]InstInfo, error) {
	ctx := ctxt.New(context.Background(), opt.Concurrency)
	metadata, err := m.meta(name)
	if err != nil && !errors.Is(perrs.Cause(err), meta.ErrValidate) &&
		!errors.Is(perrs.Cause(err), spec.ErrNoTiSparkMaster) {
		return nil, err
	}

	topo := metadata.GetTopology()
	base := metadata.GetBaseMeta()

	err = SetSSHKeySet(ctx, m.specManager.Path(name, "ssh", "id_rsa"), m.specManager.Path(name, "ssh", "id_rsa.pub"))
	if err != nil {
		return nil, err
	}

	err = SetClusterSSH(ctx, topo, base.User, opt.SSHTimeout, opt.SSHType, topo.BaseTopo().GlobalOptions.SSHType)
	if err != nil {
		return nil, err
	}

	filterRoles := set.NewStringSet(opt.Roles...)
	filterNodes := set.NewStringSet(opt.Nodes...)
	masterList := topo.BaseTopo().MasterList
	tlsCfg, err := topo.TLSConfig(m.specManager.Path(name, spec.TLSCertKeyDir))
	if err != nil {
		return nil, err
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
		dashboardAddr, _ = t.GetDashboardAddress(tlsCfg, masterActive...)
	}

	clusterInstInfos := []InstInfo{}

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

		since := "-"
		if opt.ShowUptime {
			since = formatInstanceSince(ins.Uptime(tlsCfg))
		}

		// Query the service status and uptime
		if status == "-" || (opt.ShowUptime && since == "-") {
			e, found := ctxt.GetInner(ctx).GetExecutor(ins.GetHost())
			if found {
				active, _ := operator.GetServiceStatus(ctx, e, ins.ServiceName())
				if status == "-" {
					if parts := strings.Split(strings.TrimSpace(active), " "); len(parts) > 2 {
						if parts[1] == "active" {
							status = "Up"
						} else {
							status = parts[1]
						}
					}
				}
				if opt.ShowUptime && since == "-" {
					since = formatInstanceSince(parseSystemctlSince(active))
				}
			}
		}

		// check if the role is patched
		roleName := ins.Role()
		if ins.IsPatched() {
			roleName += " (patched)"
		}
		clusterInstInfos = append(clusterInstInfos, InstInfo{
			ID:            ins.ID(),
			Role:          roleName,
			Host:          ins.GetHost(),
			Ports:         utils.JoinInt(ins.UsedPorts(), "/"),
			OsArch:        tui.OsArch(ins.OS(), ins.Arch()),
			Status:        status,
			DataDir:       dataDir,
			DeployDir:     deployDir,
			ComponentName: ins.ComponentName(),
			Port:          ins.GetPort(),
			Since:         since,
		})
	})

	// Sort by role,host,ports
	sort.Slice(clusterInstInfos, func(i, j int) bool {
		lhs, rhs := clusterInstInfos[i], clusterInstInfos[j]
		if lhs.Role != rhs.Role {
			return lhs.Role < rhs.Role
		}
		if lhs.Host != rhs.Host {
			return lhs.Host < rhs.Host
		}
		return lhs.Ports < rhs.Ports
	})

	return clusterInstInfos, nil
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
	case startsWith("down", "err", "inactive"): // down, down|ui
		return color.RedString(status)
	case startsWith("tombstone", "disconnected", "n/a"), strings.Contains(status, "offline"):
		return color.YellowString(status)
	default:
		return status
	}
}

func formatInstanceSince(uptime time.Duration) string {
	if uptime == 0 {
		return "-"
	}

	d := int64(uptime.Hours() / 24)
	h := int64(math.Mod(uptime.Hours(), 24))
	m := int64(math.Mod(uptime.Minutes(), 60))
	s := int64(math.Mod(uptime.Seconds(), 60))

	chunks := []struct {
		unit  string
		value int64
	}{
		{"d", d},
		{"h", h},
		{"m", m},
		{"s", s},
	}

	parts := []string{}

	for _, chunk := range chunks {
		switch chunk.value {
		case 0:
			continue
		default:
			parts = append(parts, fmt.Sprintf("%d%s", chunk.value, chunk.unit))
		}
	}

	return strings.Join(parts, "")
}

// `systemctl status xxx.service` returns as below
// Active: active (running) since Sat 2021-03-27 10:51:11 CST; 41min ago
func parseSystemctlSince(str string) (dur time.Duration) {
	// if service is not found or other error, don't need to parse it
	if str == "" {
		return 0
	}
	defer func() {
		if dur == 0 {
			log.Warnf("failed to parse systemctl since '%s'", str)
		}
	}()
	parts := strings.Split(str, ";")
	if len(parts) != 2 {
		return
	}
	parts = strings.Split(parts[0], " ")
	if len(parts) < 3 {
		return
	}

	dateStr := strings.Join(parts[len(parts)-3:], " ")

	tm, err := time.Parse("2006-01-02 15:04:05 MST", dateStr)
	if err != nil {
		return
	}

	return time.Since(tm)
}

// SetSSHKeySet set ssh key set.
func SetSSHKeySet(ctx context.Context, privateKeyPath string, publicKeyPath string) error {
	ctxt.GetInner(ctx).PrivateKeyPath = privateKeyPath
	ctxt.GetInner(ctx).PublicKeyPath = publicKeyPath
	return nil
}

// SetClusterSSH set cluster user ssh executor in context.
func SetClusterSSH(ctx context.Context, topo spec.Topology, deployUser string, sshTimeout uint64, sshType, defaultSSHType executor.SSHType) error {
	if sshType == "" {
		sshType = defaultSSHType
	}
	if len(ctxt.GetInner(ctx).PrivateKeyPath) == 0 {
		return perrs.Errorf("context has no PrivateKeyPath")
	}

	for _, com := range topo.ComponentsByStartOrder() {
		for _, in := range com.Instances() {
			cf := executor.SSHConfig{
				Host:    in.GetHost(),
				Port:    in.GetSSHPort(),
				KeyFile: ctxt.GetInner(ctx).PrivateKeyPath,
				User:    deployUser,
				Timeout: time.Second * time.Duration(sshTimeout),
			}

			e, err := executor.New(sshType, false, cf)
			if err != nil {
				return err
			}
			ctxt.GetInner(ctx).SetExecutor(in.GetHost(), e)
		}
	}

	return nil
}
