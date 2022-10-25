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
	"crypto/tls"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"net/url"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/fatih/color"
	perrs "github.com/pingcap/errors"
	"github.com/pingcap/tiup/pkg/checkpoint"
	"github.com/pingcap/tiup/pkg/cluster/api"
	"github.com/pingcap/tiup/pkg/cluster/clusterutil"
	"github.com/pingcap/tiup/pkg/cluster/ctxt"
	"github.com/pingcap/tiup/pkg/cluster/executor"
	operator "github.com/pingcap/tiup/pkg/cluster/operation"
	"github.com/pingcap/tiup/pkg/cluster/spec"
	"github.com/pingcap/tiup/pkg/crypto"
	logprinter "github.com/pingcap/tiup/pkg/logger/printer"
	"github.com/pingcap/tiup/pkg/meta"
	"github.com/pingcap/tiup/pkg/set"
	"github.com/pingcap/tiup/pkg/tui"
	"github.com/pingcap/tiup/pkg/utils"
	"go.uber.org/zap"
)

// DisplayOption represents option of display command
type DisplayOption struct {
	ClusterName string
	ShowUptime  bool
	ShowProcess bool
}

// InstInfo represents an instance info
type InstInfo struct {
	ID          string `json:"id"`
	Role        string `json:"role"`
	Host        string `json:"host"`
	Ports       string `json:"ports"`
	OsArch      string `json:"os_arch"`
	Status      string `json:"status"`
	Memory      string `json:"memory"`
	MemoryLimit string `json:"memory_limit"`
	CPUquota    string `json:"cpu_quota"`
	Since       string `json:"since"`
	DataDir     string `json:"data_dir"`
	DeployDir   string `json:"deploy_dir"`

	ComponentName string
	Port          int
}

// LabelInfo represents an instance label info
type LabelInfo struct {
	Machine   string `json:"machine"`
	Port      string `json:"port"`
	Store     string `json:"store"`
	Status    string `json:"status"`
	Leaders   string `json:"leaders"`
	Regions   string `json:"regions"`
	Capacity  string `json:"capacity"`
	Available string `json:"available"`
	Labels    string `json:"labels"`
}

// ClusterMetaInfo hold the structure for the JSON output of the dashboard info
type ClusterMetaInfo struct {
	ClusterType    string   `json:"cluster_type"`
	ClusterName    string   `json:"cluster_name"`
	ClusterVersion string   `json:"cluster_version"`
	DeployUser     string   `json:"deploy_user"`
	SSHType        string   `json:"ssh_type"`
	TLSEnabled     bool     `json:"tls_enabled"`
	TLSCACert      string   `json:"tls_ca_cert,omitempty"`
	TLSClientCert  string   `json:"tls_client_cert,omitempty"`
	TLSClientKey   string   `json:"tls_client_key,omitempty"`
	DashboardURL   string   `json:"dashboard_url,omitempty"`
	GrafanaURLS    []string `json:"grafana_urls,omitempty"`
}

// JSONOutput holds the structure for the JSON output of `tiup cluster display --json`
type JSONOutput struct {
	ClusterMetaInfo ClusterMetaInfo `json:"cluster_meta"`
	InstanceInfos   []InstInfo      `json:"instances,omitempty"`
	LocationLabel   string          `json:"location_label,omitempty"`
	LabelInfos      []api.LabelInfo `json:"labels,omitempty"`
}

// Display cluster meta and topology.
func (m *Manager) Display(dopt DisplayOption, opt operator.Options) error {
	name := dopt.ClusterName
	if err := clusterutil.ValidateClusterNameOrError(name); err != nil {
		return err
	}

	clusterInstInfos, err := m.GetClusterTopology(dopt, opt)
	if err != nil {
		return err
	}

	metadata, _ := m.meta(name)
	topo := metadata.GetTopology()
	base := metadata.GetBaseMeta()
	cyan := color.New(color.FgCyan, color.Bold)

	statusTimeout := time.Duration(opt.APITimeout) * time.Second
	// display cluster meta
	var j *JSONOutput
	if m.logger.GetDisplayMode() == logprinter.DisplayModeJSON {
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
				nil,
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
	rowHead := []string{"ID", "Role", "Host", "Ports", "OS/Arch", "Status"}
	if dopt.ShowProcess {
		rowHead = append(rowHead, "Memory", "Memory Limit", "CPU Quota")
	}
	if dopt.ShowUptime {
		rowHead = append(rowHead, "Since")
	}
	rowHead = append(rowHead, "Data Dir", "Deploy Dir")
	clusterTable = append(clusterTable, rowHead)

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
		if dopt.ShowProcess {
			row = append(row, v.Memory, v.MemoryLimit, v.CPUquota)
		}
		if dopt.ShowUptime {
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
	ctx := ctxt.New(
		context.Background(),
		opt.Concurrency,
		m.logger,
	)
	if t, ok := topo.(*spec.Specification); ok {
		var err error
		dashboardAddr, err = t.GetDashboardAddress(ctx, tlsCfg, statusTimeout, masterActive...)
		if err == nil && !set.NewStringSet("", "auto", "none").Exist(dashboardAddr) {
			scheme := "http"
			if tlsCfg != nil {
				scheme = "https"
			}
			if m.logger.GetDisplayMode() == logprinter.DisplayModeJSON {
				j.ClusterMetaInfo.DashboardURL = fmt.Sprintf("%s://%s/dashboard", scheme, dashboardAddr)
			} else {
				fmt.Printf("Dashboard URL:      %s\n", cyan.Sprintf("%s://%s/dashboard", scheme, dashboardAddr))
			}
		}
	}

	if m.logger.GetDisplayMode() == logprinter.DisplayModeJSON {
		grafanaURLs := getGrafanaURL(clusterInstInfos)
		if len(grafanaURLs) != 0 {
			j.ClusterMetaInfo.GrafanaURLS = grafanaURLs
		}
	} else {
		urls, exist := getGrafanaURLStr(clusterInstInfos)
		if exist {
			fmt.Printf("Grafana URL:        %s\n", cyan.Sprintf("%s", urls))
		}
	}

	if m.logger.GetDisplayMode() == logprinter.DisplayModeJSON {
		d, err := json.MarshalIndent(j, "", "  ")
		if err != nil {
			return err
		}
		fmt.Println(string(d))
		return nil
	}

	tui.PrintTable(clusterTable, true)
	fmt.Printf("Total nodes: %d\n", len(clusterTable)-1)

	if t, ok := topo.(*spec.Specification); ok {
		// Check if TiKV's label set correctly
		pdClient := api.NewPDClient(
			context.WithValue(ctx, logprinter.ContextKeyLogger, m.logger),
			masterActive,
			10*time.Second,
			tlsCfg,
		)

		if lbs, placementRule, err := pdClient.GetLocationLabels(); err != nil {
			m.logger.Debugf("get location labels from pd failed: %v", err)
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

func getGrafanaURL(clusterInstInfos []InstInfo) (result []string) {
	var grafanaURLs []string
	for _, instance := range clusterInstInfos {
		if instance.Role == "grafana" {
			grafanaURLs = append(grafanaURLs, fmt.Sprintf("http://%s:%d", instance.Host, instance.Port))
		}
	}
	return grafanaURLs
}

func getGrafanaURLStr(clusterInstInfos []InstInfo) (result string, exist bool) {
	grafanaURLs := getGrafanaURL(clusterInstInfos)
	if len(grafanaURLs) == 0 {
		return "", false
	}
	return strings.Join(grafanaURLs, ","), true
}

// DisplayTiKVLabels display cluster tikv labels
func (m *Manager) DisplayTiKVLabels(dopt DisplayOption, opt operator.Options) error {
	name := dopt.ClusterName
	if err := clusterutil.ValidateClusterNameOrError(name); err != nil {
		return err
	}

	clusterInstInfos, err := m.GetClusterTopology(dopt, opt)
	if err != nil {
		return err
	}

	metadata, _ := m.meta(name)
	topo := metadata.GetTopology()
	base := metadata.GetBaseMeta()
	statusTimeout := time.Duration(opt.APITimeout) * time.Second
	// display cluster meta
	cyan := color.New(color.FgCyan, color.Bold)

	var j *JSONOutput
	if strings.ToLower(opt.DisplayMode) == "json" {
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
				nil,
			},
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
		fmt.Printf("SSH type:           %s\n", cyan.Sprint(topo.BaseTopo().GlobalOptions.SSHType))
		fmt.Printf("Component name:     %s\n", cyan.Sprint("TiKV"))

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
	}

	// display topology
	var clusterTable [][]string
	clusterTable = append(clusterTable, []string{"Machine", "Port", "Store", "Status", "Leaders", "Regions", "Capacity", "Available", "Labels"})

	masterActive := make([]string, 0)
	tikvStoreIP := make(map[string]struct{})
	for _, v := range clusterInstInfos {
		if v.ComponentName == spec.ComponentTiKV {
			tikvStoreIP[v.Host] = struct{}{}
		}
	}

	ctx := ctxt.New(
		context.Background(),
		opt.Concurrency,
		m.logger,
	)

	masterList := topo.BaseTopo().MasterList
	tlsCfg, err := topo.TLSConfig(m.specManager.Path(name, spec.TLSCertKeyDir))
	if err != nil {
		return err
	}

	var mu sync.Mutex
	topo.IterInstance(func(ins spec.Instance) {
		if ins.ComponentName() == spec.ComponentPD {
			status := ins.Status(ctx, statusTimeout, tlsCfg, masterList...)
			if strings.HasPrefix(status, "Up") || strings.HasPrefix(status, "Healthy") {
				instAddr := fmt.Sprintf("%s:%d", ins.GetHost(), ins.GetPort())
				mu.Lock()
				masterActive = append(masterActive, instAddr)
				mu.Unlock()
			}
		}
	}, opt.Concurrency)

	var (
		labelInfoArr  []api.LabelInfo
		locationLabel []string
	)

	if _, ok := topo.(*spec.Specification); ok {
		// Check if TiKV's label set correctly
		pdClient := api.NewPDClient(ctx, masterActive, 10*time.Second, tlsCfg)
		// No
		locationLabel, _, err = pdClient.GetLocationLabels()
		if err != nil {
			m.logger.Debugf("get location labels from pd failed: %v", err)
		}

		_, storeInfos, err := pdClient.GetTiKVLabels()
		if err != nil {
			m.logger.Debugf("get tikv state and labels from pd failed: %v", err)
		}

		for storeIP := range tikvStoreIP {
			row := []string{
				color.CyanString(storeIP),
				"",
				"",
				"",
				"",
				"",
				"",
				"",
				"",
			}
			clusterTable = append(clusterTable, row)

			for _, val := range storeInfos {
				if store, ok := val[storeIP]; ok {
					row := []string{
						"",
						store.Port,
						strconv.FormatUint(store.Store, 10),
						color.CyanString(store.Status),
						fmt.Sprintf("%v", store.Leaders),
						fmt.Sprintf("%v", store.Regions),
						store.Capacity,
						store.Available,
						store.Labels,
					}
					clusterTable = append(clusterTable, row)

					labelInfoArr = append(labelInfoArr, store)
				}
			}
		}
	}

	if strings.ToLower(opt.DisplayMode) == "json" {
		j.LocationLabel = strings.Join(locationLabel, ",")
		j.LabelInfos = labelInfoArr
		d, err := json.MarshalIndent(j, "", "  ")
		if err != nil {
			return err
		}
		fmt.Println(string(d))
		return nil
	}
	fmt.Printf("Location labels:    %s\n", cyan.Sprint(strings.Join(locationLabel, ",")))
	tui.PrintTable(clusterTable, true)
	fmt.Printf("Total nodes: %d\n", len(clusterTable)-1)

	return nil
}

// GetClusterTopology get the topology of the cluster.
func (m *Manager) GetClusterTopology(dopt DisplayOption, opt operator.Options) ([]InstInfo, error) {
	ctx := ctxt.New(
		context.Background(),
		opt.Concurrency,
		m.logger,
	)
	name := dopt.ClusterName
	metadata, err := m.meta(name)
	if err != nil && !errors.Is(perrs.Cause(err), meta.ErrValidate) &&
		!errors.Is(perrs.Cause(err), spec.ErrNoTiSparkMaster) {
		return nil, err
	}

	topo := metadata.GetTopology()
	base := metadata.GetBaseMeta()

	statusTimeout := time.Duration(opt.APITimeout) * time.Second

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

	var mu sync.Mutex
	topo.IterInstance(func(ins spec.Instance) {
		if ins.ComponentName() != spec.ComponentPD && ins.ComponentName() != spec.ComponentDMMaster {
			return
		}

		status := ins.Status(ctx, statusTimeout, tlsCfg, masterList...)
		mu.Lock()
		if strings.HasPrefix(status, "Up") || strings.HasPrefix(status, "Healthy") {
			instAddr := fmt.Sprintf("%s:%d", ins.GetHost(), ins.GetPort())
			masterActive = append(masterActive, instAddr)
		}
		masterStatus[ins.ID()] = status
		mu.Unlock()
	}, opt.Concurrency)

	var dashboardAddr string
	if t, ok := topo.(*spec.Specification); ok {
		dashboardAddr, _ = t.GetDashboardAddress(ctx, tlsCfg, statusTimeout, masterActive...)
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

		var status, memory string
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
			status = ins.Status(ctx, statusTimeout, tlsCfg, masterActive...)
		}

		since := "-"
		if dopt.ShowUptime {
			since = formatInstanceSince(ins.Uptime(ctx, statusTimeout, tlsCfg))
		}

		// Query the service status and uptime
		if status == "-" || (dopt.ShowUptime && since == "-") || dopt.ShowProcess {
			e, found := ctxt.GetInner(ctx).GetExecutor(ins.GetHost())
			if found {
				var active string
				nctx := checkpoint.NewContext(ctx)
				active, memory, _ = operator.GetServiceStatus(nctx, e, ins.ServiceName())
				if status == "-" {
					if active == "active" {
						status = "Up"
					} else {
						status = active
					}
				}
				if dopt.ShowUptime && since == "-" {
					since = formatInstanceSince(parseSystemctlSince(active))
				}
			}
		}

		// check if the role is patched
		roleName := ins.Role()
		if ins.IsPatched() {
			roleName += " (patched)"
		}
		rc := ins.ResourceControl()
		mu.Lock()
		clusterInstInfos = append(clusterInstInfos, InstInfo{
			ID:            ins.ID(),
			Role:          roleName,
			Host:          ins.GetHost(),
			Ports:         utils.JoinInt(ins.UsedPorts(), "/"),
			OsArch:        tui.OsArch(ins.OS(), ins.Arch()),
			Status:        status,
			Memory:        utils.Ternary(memory == "", "-", memory).(string),
			MemoryLimit:   utils.Ternary(rc.MemoryLimit == "", "-", rc.MemoryLimit).(string),
			CPUquota:      utils.Ternary(rc.CPUQuota == "", "-", rc.CPUQuota).(string),
			DataDir:       dataDir,
			DeployDir:     deployDir,
			ComponentName: ins.ComponentName(),
			Port:          ins.GetPort(),
			Since:         since,
		})
		mu.Unlock()
	}, opt.Concurrency)

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
	case startsWith("tombstone", "disconnected", "n/a"), strings.Contains(strings.ToLower(status), "offline"):
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
			zap.L().Warn("failed to parse systemctl since", zap.String("value", str))
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

// DisplayDashboardInfo prints the dashboard address of cluster
func (m *Manager) DisplayDashboardInfo(clusterName string, timeout time.Duration, tlsCfg *tls.Config) error {
	metadata, err := spec.ClusterMetadata(clusterName)
	if err != nil && !errors.Is(perrs.Cause(err), meta.ErrValidate) &&
		!errors.Is(perrs.Cause(err), spec.ErrNoTiSparkMaster) {
		return err
	}

	pdEndpoints := make([]string, 0)
	for _, pd := range metadata.Topology.PDServers {
		pdEndpoints = append(pdEndpoints, fmt.Sprintf("%s:%d", pd.Host, pd.ClientPort))
	}

	ctx := context.WithValue(context.Background(), logprinter.ContextKeyLogger, m.logger)
	pdAPI := api.NewPDClient(ctx, pdEndpoints, timeout, tlsCfg)
	dashboardAddr, err := pdAPI.GetDashboardAddress()
	if err != nil {
		return fmt.Errorf("failed to retrieve TiDB Dashboard instance from PD: %s", err)
	}
	if dashboardAddr == "auto" {
		return fmt.Errorf("TiDB Dashboard is not initialized, please start PD and try again")
	} else if dashboardAddr == "none" {
		return fmt.Errorf("TiDB Dashboard is disabled")
	}

	u, err := url.Parse(dashboardAddr)
	if err != nil {
		return fmt.Errorf("unknown TiDB Dashboard PD instance: %s", dashboardAddr)
	}

	u.Path = "/dashboard/"

	if tlsCfg != nil {
		fmt.Println(
			"Client certificate:",
			color.CyanString(m.specManager.Path(clusterName, spec.TLSCertKeyDir, spec.PFXClientCert)),
		)
		fmt.Println(
			"Certificate password:",
			color.CyanString(crypto.PKCS12Password),
		)
	}
	fmt.Println(
		"Dashboard URL:",
		color.CyanString(u.String()),
	)

	return nil
}
