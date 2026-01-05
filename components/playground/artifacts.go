package main

import (
	"context"
	"encoding/json"
	stdErrors "errors"
	"fmt"
	"net"
	"net/http"
	"os/exec"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/pingcap/errors"
	"github.com/pingcap/tiup/components/playground/proc"
	pgservice "github.com/pingcap/tiup/components/playground/service"
	logprinter "github.com/pingcap/tiup/pkg/logger/printer"
	"github.com/pingcap/tiup/pkg/tui/colorstr"
	tuiv2output "github.com/pingcap/tiup/pkg/tuiv2/output"
	"github.com/pingcap/tiup/pkg/utils"
)

// MonitorInfo represent the monitor
type MonitorInfo struct {
	IP         string `json:"ip"`
	Port       int    `json:"port"`
	BinaryPath string `json:"binary_path"`
}

func (p *Playground) printProcExitError(inst proc.Process, err error) {
	if p == nil || inst == nil || err == nil {
		return
	}
	out := p.termWriter()
	logFile := inst.LogFile()

	exitCode := -1
	var exitErr *exec.ExitError
	if stdErrors.As(err, &exitErr) {
		if ps := exitErr.ProcessState; ps != nil {
			exitCode = ps.ExitCode()
		}
	}
	if exitCode < 0 {
		if info := inst.Info(); info != nil {
			proc := info.Proc
			if proc != nil {
				func() {
					defer func() { _ = recover() }()
					cmd := proc.Cmd()
					if cmd != nil && cmd.ProcessState != nil {
						exitCode = cmd.ProcessState.ExitCode()
					}
				}()
			}
		}
	}

	title := procDisplayName(inst, true)
	headerText := ""
	if exitCode >= 0 {
		headerText = fmt.Sprintf("%s Exit Code %d", title, exitCode)
	} else {
		headerText = fmt.Sprintf("%s Exited", title)
	}

	lines, _ := utils.TailN(logFile, 10)
	contentLines := make([]string, 0, 1+len(lines))
	contentLines = append(contentLines, fmt.Sprintf("Tail logs: %s\n", logFile))
	contentLines = append(contentLines, lines...)

	fmt.Fprint(out, tuiv2output.Callout{
		Style:      tuiv2output.CalloutFailed,
		StatusText: headerText,
		Content:    strings.Join(contentLines, "\n"),
	}.Render(out))
}

func (p *Playground) printClusterInfoCallout(tidbSucc, tiproxySucc []string) bool {
	if p == nil {
		return false
	}

	out := p.termWriter()
	mysql := mysqlCommand()
	dashboardURL, grafanaURL := p.clusterInfoMonitorURLs()

	type block = []string
	var blocks []block

	blocks = append(blocks, block{colorstr.Sprintf("[bold]TiDB Playground Cluster is started, enjoy! 🎉[reset]")})
	if p.deleteWhenExit {
		blocks = append(blocks, block{
			colorstr.Sprintf("[dark_gray]Cluster data will be destroyed after exit.[reset]"),
			colorstr.Sprintf("[dark_gray]To persist data after exit, use [cyan]--tag <name>[reset][dark_gray].[reset]"),
		})
	}

	if rows := p.clusterInfoCalloutRows(mysql, dashboardURL, grafanaURL, tidbSucc, tiproxySucc); len(rows) > 0 {
		blocks = append(blocks, tuiv2output.Labels{Rows: rows}.Lines(out))
	}

	content := joinNonEmptyBlocks(blocks)
	fmt.Fprint(out, tuiv2output.Callout{
		Style:      tuiv2output.CalloutSucceeded,
		StatusText: "Cluster info",
		Content:    content,
	}.Render(out))
	return true
}

func (p *Playground) clusterInfoBasicRows() [][2]string {
	if p == nil {
		return nil
	}

	bootVer := utils.LatestVersionAlias
	if p.bootOptions != nil && p.bootOptions.Version != "" {
		bootVer = p.bootOptions.Version
	}

	var rows [][2]string
	if bootVer != "" {
		rows = append(rows, [2]string{"Version:", bootVer})
	}
	if p.dataDir != "" {
		value := prettifyUserPath(p.dataDir)
		if p.deleteWhenExit {
			value += " " + colorstr.Sprintf("[yellow][bold](Destroy after exit)[reset]")
		}
		rows = append(rows, [2]string{"Data dir:", value})
	}
	return rows
}

func (p *Playground) clusterInfoCalloutRows(mysql, dashboardURL, grafanaURL string, tidbSucc, tiproxySucc []string) [][2]string {
	if p == nil {
		return nil
	}

	basic := p.clusterInfoBasicRows()

	var rest [][2]string
	rest = append(rest, clusterInfoMySQLConnectRows(mysql, "Connect TiDB:", tidbSucc)...)
	rest = append(rest, clusterInfoMySQLConnectRows(mysql, "Connect TiProxy:", tiproxySucc)...)
	rest = append(rest, p.clusterInfoDMConnectRows()...)
	if dashboardURL != "" {
		rest = append(rest, [2]string{"TiDB Dashboard:", dashboardURL})
	}
	if grafanaURL != "" {
		rest = append(rest, [2]string{"Grafana:", grafanaURL})
	}
	rest = append(rest, p.clusterInfoTiKVSlimRows()...)

	var rows [][2]string
	rows = append(rows, basic...)
	if len(basic) > 0 && len(rest) > 0 {
		rows = append(rows, [2]string{"", ""})
	}
	rows = append(rows, rest...)
	return rows
}

func joinNonEmptyBlocks(blocks [][]string) string {
	var lines []string
	for _, b := range blocks {
		if len(b) == 0 {
			continue
		}
		if len(lines) > 0 {
			lines = append(lines, "")
		}
		lines = append(lines, b...)
	}
	return strings.Join(lines, "\n")
}

func (p *Playground) clusterInfoMonitorURLs() (dashboardURL, grafanaURL string) {
	if p == nil {
		return "", ""
	}

	pdMembers := pgservice.ProcsOf[*proc.PDInstance](p, proc.ServicePD, proc.ServicePDAPI)
	tidbInstances := pgservice.ProcsOf[*proc.TiDBInstance](p, proc.ServiceTiDBSystem, proc.ServiceTiDB)

	if len(pdMembers) > 0 {
		pdAddr := pdMembers[0].Addr()
		if len(tidbInstances) > 0 && hasDashboard(pdAddr) {
			dashboardURL = fmt.Sprintf("http://%s/dashboard", pdAddr)
		}
	}

	if gs := pgservice.ProcsOf[*proc.GrafanaInstance](p, proc.ServiceGrafana); len(gs) > 0 && gs[0] != nil {
		grafanaURL = fmt.Sprintf("http://%s", utils.JoinHostPort(gs[0].Host, gs[0].Port))
	}

	return dashboardURL, grafanaURL
}

func clusterInfoMySQLConnectRows(mysql, label string, addrs []string) [][2]string {
	var rows [][2]string
	for _, dbAddr := range addrs {
		host, port, err := net.SplitHostPort(dbAddr)
		if err != nil {
			continue
		}
		cmd := fmt.Sprintf("%s --host %s --port %s -u root", mysql, host, port)
		rows = append(rows, [2]string{label, cmd})
	}
	return rows
}

func (p *Playground) clusterInfoDMConnectRows() [][2]string {
	if p == nil {
		return nil
	}
	dmMasters := pgservice.ProcsOf[*proc.DMMaster](p, proc.ServiceDMMaster)
	if len(dmMasters) == 0 {
		return nil
	}

	endpoints := make([]string, 0, len(dmMasters))
	for _, dmMaster := range dmMasters {
		endpoints = append(endpoints, dmMaster.Addr())
	}
	cmd := fmt.Sprintf("tiup dmctl --master-addr %s", strings.Join(endpoints, ","))
	return [][2]string{{"Connect DM:", cmd}}
}

func (p *Playground) clusterInfoTiKVSlimRows() [][2]string {
	if p == nil || p.bootOptions == nil || p.bootOptions.ShOpt.Mode != proc.ModeTiKVSlim {
		return nil
	}

	pdAddrs := func(serviceIDs ...proc.ServiceID) string {
		var addrs []string
		for _, pd := range pgservice.ProcsOf[*proc.PDInstance](p, serviceIDs...) {
			addrs = append(addrs, pd.Addr())
		}
		return strings.Join(addrs, ",")
	}

	if p.bootOptions.ShOpt.PDMode != "ms" {
		return [][2]string{{"PD Endpoints:", pdAddrs(proc.ServicePD, proc.ServicePDAPI)}}
	}

	return [][2]string{
		{"PD API Endpoints:", pdAddrs(proc.ServicePDAPI)},
		{"PD TSO Endpoints:", pdAddrs(proc.ServicePDTSO)},
		{"PD Scheduling Endpoints:", pdAddrs(proc.ServicePDScheduling)},
		{"PD Router Endpoints:", pdAddrs(proc.ServicePDRouter)},
		{"PD Resource Manager Endpoints:", pdAddrs(proc.ServicePDResourceManager)},
	}
}

func hasDashboard(pdAddr string) bool {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	url := fmt.Sprintf("http://%s/dashboard", pdAddr)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return false
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return false
	}
	defer resp.Body.Close()

	return resp.StatusCode == 200
}

func (p *Playground) updateMonitorTopology(componentID string, info MonitorInfo) {
	info.IP = proc.AdvertiseHost(info.IP)
	pdMembers := pgservice.ProcsOf[*proc.PDInstance](p, proc.ServicePD, proc.ServicePDAPI)
	if len(pdMembers) == 0 {
		return
	}

	client, err := newEtcdClient(pdMembers[0].Addr())
	if err != nil {
		return
	}
	defer client.Close()

	promBinary, err := json.Marshal(info)
	if err != nil {
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if _, err := client.Put(ctx, "/topology/"+componentID, string(promBinary)); err != nil {
		fmt.Fprintf(p.termWriter(), "Set the PD metrics storage failed: %v\n", err)
	}
}

type procWalker func(fn func(serviceID proc.ServiceID, inst proc.Process) error) error

func renderPrometheusSDFile(prom *proc.PrometheusInstance, walk procWalker) error {
	if prom == nil || walk == nil {
		return nil
	}
	return prom.RenderSDFile(collectMetricTargets(walk))
}

func collectMetricTargets(walk procWalker) map[proc.ServiceID]proc.MetricAddr {
	sid2targets := make(map[proc.ServiceID]proc.MetricAddr)
	if walk == nil {
		return sid2targets
	}

	_ = walk(func(serviceID proc.ServiceID, inst proc.Process) error {
		if inst == nil {
			return nil
		}
		info := inst.Info()
		if serviceID == proc.ServicePrometheus || (info != nil && info.Service == proc.ServicePrometheus) {
			return nil
		}

		var v proc.MetricAddr
		if m, ok := inst.(interface{ MetricAddr() proc.MetricAddr }); ok {
			v = m.MetricAddr()
		} else if info != nil {
			v = info.MetricAddr()
		} else {
			return nil
		}
		if len(v.Targets) == 0 {
			return nil
		}

		if serviceID == "" && info != nil {
			serviceID = info.Service
		}
		if serviceID == "" {
			return nil
		}

		if t, ok := sid2targets[serviceID]; ok {
			v.Targets = append(v.Targets, t.Targets...)
			if v.Labels == nil && len(t.Labels) > 0 {
				v.Labels = t.Labels
			}
		}
		sid2targets[serviceID] = v
		return nil
	})

	return sid2targets
}

func (p *Playground) renderSDFile() error {
	proms := pgservice.ProcsOf[*proc.PrometheusInstance](p, proc.ServicePrometheus)
	if len(proms) == 0 || proms[0] == nil {
		return nil
	}
	prom := proms[0]
	return renderPrometheusSDFile(prom, p.WalkProcs)
}

func logIfErr(err error) {
	if err != nil {
		logprinter.Warnf("%v", err)
	}
}

// Check the MySQL Client version
//
// Since v8.1.0 `--comments` is the default, so we don't need to specify it.
// Without `--comments` the MySQL client strips TiDB specific comments
// like `/*T![clustered_index] CLUSTERED */`
//
// This returns `mysql --comments` for older versions or in case we failed to check
// the version for any reason as `mysql --comments` is the safe option.
// For newer MySQL versions it returns just `mysql`.
//
// For MariaDB versions of the MySQL Client it is expected to return `mysql --comments`.
func mysqlCommand() (cmd string) {
	cmd = "mysql --comments"
	mysqlVerOutput, err := exec.Command("mysql", "--version").Output()
	if err != nil {
		return
	}
	vMaj, vMin, _, err := parseMysqlVersion(string(mysqlVerOutput))
	if err == nil {
		// MySQL Client 8.1.0 and newer
		if vMaj == 8 && vMin >= 1 {
			return "mysql"
		}
		// MySQL Client 9.x.x. Note that 10.x is likely to be MariaDB, so not using >= here.
		if vMaj == 9 {
			return "mysql"
		}
	}
	return
}

// parseMysqlVersion parses the output from `mysql --version` that is in `versionOutput`
// and returns the major, minor and patch version.
//
// New format example: `mysql  Ver 8.2.0 for Linux on x86_64 (MySQL Community Server - GPL)`
// Old format example: `mysql  Ver 14.14 Distrib 5.7.36, for linux-glibc2.12 (x86_64) using  EditLine wrapper`
// MariaDB 11.2 format: `/usr/bin/mysql from 11.2.2-MariaDB, client 15.2 for linux-systemd (x86_64) using readline 5.1`
//
// Note that MariaDB has `bin/mysql` (deprecated) and `bin/mariadb`. This is to parse the version from `bin/mysql`.
// As TiDB is a MySQL compatible database we recommend `bin/mysql` from MySQL.
// If we ever want to auto-detect other clients like `bin/mariadb`, `bin/mysqlsh`, `bin/mycli`, etc then
// each of them needs their own version detection and adjust for the right commandline options.
func parseMysqlVersion(versionOutput string) (vMaj int, vMin int, vPatch int, err error) {
	mysqlVerRegexp := regexp.MustCompile(`(Ver|Distrib|from) ([0-9]+)\.([0-9]+)\.([0-9]+)`)
	mysqlVerMatch := mysqlVerRegexp.FindStringSubmatch(versionOutput)
	if mysqlVerMatch == nil {
		return 0, 0, 0, errors.New("No match")
	}
	vMaj, err = strconv.Atoi(mysqlVerMatch[2])
	if err != nil {
		return 0, 0, 0, err
	}
	vMin, err = strconv.Atoi(mysqlVerMatch[3])
	if err != nil {
		return 0, 0, 0, err
	}
	vPatch, err = strconv.Atoi(mysqlVerMatch[4])
	if err != nil {
		return 0, 0, 0, err
	}
	return
}
