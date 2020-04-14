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

package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"github.com/fatih/color"
	_ "github.com/go-sql-driver/mysql"
	"github.com/pingcap-incubator/tiup/components/playground/instance"
	"github.com/pingcap-incubator/tiup/pkg/localdata"
	"github.com/pingcap-incubator/tiup/pkg/meta"
	"github.com/pingcap-incubator/tiup/pkg/repository"
	"github.com/pingcap-incubator/tiup/pkg/utils"
	"github.com/spf13/cobra"
	"go.etcd.io/etcd/clientv3"
	"go.uber.org/zap"
)

type bootOptions struct {
	version           string
	pdConfigPath      string
	tidbConfigPath    string
	tikvConfigPath    string
	tiflashConfigPath string
	pdBinPath         string
	tidbBinPath       string
	tikvBinPath       string
	tiflashBinPath    string
	pdNum             int
	tidbNum           int
	tikvNum           int
	tiflashNum        int
	host              string
	monitor           bool
}

func installIfMissing(profile *localdata.Profile, component, version string) error {
	versions, err := meta.Profile().InstalledVersions(component)
	if err != nil {
		return err
	}
	if len(versions) > 0 {
		if repository.Version(version).IsEmpty() {
			return nil
		}
		found := false
		for _, v := range versions {
			if v == version {
				found = true
				break
			}
		}
		if found {
			return nil
		}
	}
	spec := component
	if !repository.Version(version).IsEmpty() {
		spec = fmt.Sprintf("%s:%s", component, version)
	}
	c := exec.Command("tiup", "install", spec)
	c.Stdout = os.Stdout
	c.Stderr = os.Stderr
	return c.Run()
}

func execute() error {
	tidbNum := 1
	tikvNum := 1
	pdNum := 1
	tiflashNum := 1
	host := "127.0.0.1"
	monitor := false
	tidbConfigPath := ""
	tikvConfigPath := ""
	pdConfigPath := ""
	tiflashConfigPath := ""
	tidbBinPath := ""
	tikvBinPath := ""
	pdBinPath := ""
	tiflashBinPath := ""

	rootCmd := &cobra.Command{
		Use: "tiup playground [version]",
		Long: `Bootstrap a TiDB cluster in your local host, the latest release version will be chosen
if you don't specified a version.

Examples:
  $ tiup playground nightly                         # Start a TiDB nightly version local cluster
  $ tiup playground v3.0.10 --db 3 --pd 3 --kv 3    # Start a local cluster with 10 nodes
  $ tiup playground nightly --monitor               # Start a local cluster with monitor system
  $ tiup playground --pd.config ~/config/pd.toml    # Start a local cluster with specified configuration file,
  $ tiup playground --db.binpath /xx/tidb-server    # Start a local cluster with component binary path`,
		SilenceUsage: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			version := ""
			if len(args) > 0 {
				version = args[0]
			}
			options := &bootOptions{
				version:           version,
				pdConfigPath:      pdConfigPath,
				tidbConfigPath:    tidbConfigPath,
				tikvConfigPath:    tikvConfigPath,
				tiflashConfigPath: tiflashConfigPath,
				pdBinPath:         pdBinPath,
				tidbBinPath:       tidbBinPath,
				tikvBinPath:       tikvBinPath,
				tiflashBinPath:    tiflashBinPath,
				pdNum:             pdNum,
				tidbNum:           tidbNum,
				tikvNum:           tikvNum,
				tiflashNum:        tiflashNum,
				host:              host,
				monitor:           monitor,
			}
			return bootCluster(options)
		},
	}

	rootCmd.Flags().IntVarP(&tidbNum, "db", "", 1, "TiDB instance number")
	rootCmd.Flags().IntVarP(&tikvNum, "kv", "", 1, "TiKV instance number")
	rootCmd.Flags().IntVarP(&pdNum, "pd", "", 1, "PD instance number")
	rootCmd.Flags().IntVarP(&tiflashNum, "tiflash", "", 1, "TiFlash instance number")
	rootCmd.Flags().StringVarP(&host, "host", "", host, "Playground cluster host")
	rootCmd.Flags().BoolVar(&monitor, "monitor", false, "Start prometheus component")
	rootCmd.Flags().StringVarP(&tidbConfigPath, "db.config", "", tidbConfigPath, "TiDB instance configuration file")
	rootCmd.Flags().StringVarP(&tikvConfigPath, "kv.config", "", tikvConfigPath, "TiKV instance configuration file")
	rootCmd.Flags().StringVarP(&pdConfigPath, "pd.config", "", pdConfigPath, "PD instance configuration file")
	rootCmd.Flags().StringVarP(&tiflashConfigPath, "tiflash.config", "", tiflashConfigPath, "TiFlash instance configuration file")
	rootCmd.Flags().StringVarP(&tidbBinPath, "db.binpath", "", tidbBinPath, "TiDB instance binary path")
	rootCmd.Flags().StringVarP(&tikvBinPath, "kv.binpath", "", tikvBinPath, "TiKV instance binary path")
	rootCmd.Flags().StringVarP(&pdBinPath, "pd.binpath", "", pdBinPath, "PD instance binary path")
	rootCmd.Flags().StringVarP(&tiflashBinPath, "tiflash.binpath", "", tiflashBinPath, "TiFlash instance binary path")

	return rootCmd.Execute()
}

func tryConnect(dsn string) error {
	cli, err := sql.Open("mysql", dsn)
	if err != nil {
		return err
	}
	defer cli.Close()

	conn, err := cli.Conn(context.Background())
	if err != nil {
		return err
	}
	defer conn.Close()

	return nil
}

func checkDB(dbAddr string) bool {
	dsn := fmt.Sprintf("root:@tcp(%s)/", dbAddr)
	for i := 0; i < 60; i++ {
		if err := tryConnect(dsn); err != nil {
			time.Sleep(time.Second)
			fmt.Print(".")
		} else {
			if i != 0 {
				fmt.Println()
			}
			return true
		}
	}
	return false
}

func hasDashboard(pdAddr string) bool {
	resp, err := http.Get(fmt.Sprintf("http://%s/dashboard", pdAddr))
	if err != nil {
		return false
	}
	defer resp.Body.Close()

	if resp.StatusCode == 200 {
		return true
	}
	return false
}

func getAbsolutePath(binPath string) string {
	if !strings.HasPrefix(binPath, "/") && !strings.HasPrefix(binPath, "~") {
		binPath = filepath.Join(os.Getenv(localdata.EnvNameWorkDir), binPath)
	}
	return binPath
}

func bootCluster(options *bootOptions) error {
	if options.pdNum < 1 || options.tidbNum < 1 || options.tikvNum < 1 {
		return fmt.Errorf("all components count must be great than 0 (tidb=%v, tikv=%v, pd=%v)",
			options.tidbNum, options.tikvNum, options.pdNum)
	}

	var pathMap = make(map[string]string)
	if options.tidbBinPath != "" {
		pathMap["tidb"] = getAbsolutePath(options.tidbBinPath)
	}
	if options.tikvBinPath != "" {
		pathMap["tikv"] = getAbsolutePath(options.tikvBinPath)
	}
	if options.pdBinPath != "" {
		pathMap["pd"] = getAbsolutePath(options.pdBinPath)
	}
	if options.tiflashNum > 0 && options.tiflashBinPath != "" {
		pathMap["tiflash"] = getAbsolutePath(options.tiflashBinPath)
	}

	if options.tidbConfigPath != "" {
		options.tidbConfigPath = getAbsolutePath(options.tidbConfigPath)
	}
	if options.tikvConfigPath != "" {
		options.tikvConfigPath = getAbsolutePath(options.tikvConfigPath)
	}
	if options.pdConfigPath != "" {
		options.pdConfigPath = getAbsolutePath(options.pdConfigPath)
	}
	if options.tiflashNum > 0 && options.tiflashConfigPath != "" {
		options.tiflashConfigPath = getAbsolutePath(options.tiflashConfigPath)
	}

	// Initialize the profile
	profileRoot := os.Getenv(localdata.EnvNameHome)
	if profileRoot == "" {
		return fmt.Errorf("cannot read environment variable %s", localdata.EnvNameHome)
	}
	profile := localdata.NewProfile(profileRoot)
	for _, comp := range []string{"pd", "tikv", "tidb", "tiflash"} {
		if pathMap[comp] != "" {
			continue
		}
		if comp == "tiflash" && options.tiflashNum == 0 {
			continue
		}
		if err := installIfMissing(profile, comp, options.version); err != nil {
			return err
		}
	}
	dataDir := os.Getenv(localdata.EnvNameInstanceDataDir)
	if dataDir == "" {
		return fmt.Errorf("cannot read environment variable %s", localdata.EnvNameInstanceDataDir)
	}

	all := make([]instance.Instance, 0, options.pdNum+options.tikvNum+options.tidbNum+options.tiflashNum)
	allRole := make([]string, 0, options.pdNum+options.tikvNum+options.tidbNum+options.tiflashNum)
	pds := make([]*instance.PDInstance, 0, options.pdNum)
	kvs := make([]*instance.TiKVInstance, 0, options.tikvNum)
	dbs := make([]*instance.TiDBInstance, 0, options.tidbNum)
	flashs := make([]*instance.TiFlashInstance, 0, options.tiflashNum)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	for i := 0; i < options.pdNum; i++ {
		dir := filepath.Join(dataDir, fmt.Sprintf("pd-%d", i))
		inst := instance.NewPDInstance(dir, options.host, options.pdConfigPath, i)
		pds = append(pds, inst)
		all = append(all, inst)
		allRole = append(allRole, "pd")
	}
	for _, pd := range pds {
		pd.Join(pds)
	}

	for i := 0; i < options.tikvNum; i++ {
		dir := filepath.Join(dataDir, fmt.Sprintf("tikv-%d", i))
		inst := instance.NewTiKVInstance(dir, options.host, options.tikvConfigPath, i, pds)
		kvs = append(kvs, inst)
		all = append(all, inst)
		allRole = append(allRole, "tikv")
	}

	for i := 0; i < options.tidbNum; i++ {
		dir := filepath.Join(dataDir, fmt.Sprintf("tidb-%d", i))
		inst := instance.NewTiDBInstance(dir, options.host, options.tidbConfigPath, i, pds)
		dbs = append(dbs, inst)
		all = append(all, inst)
		allRole = append(allRole, "tidb")
	}

	for i := 0; i < options.tiflashNum; i++ {
		dir := filepath.Join(dataDir, fmt.Sprintf("tiflash-%d", i))
		inst := instance.NewTiFlashInstance(dir, options.host, options.tiflashConfigPath, i, pds, dbs)
		flashs = append(flashs, inst)
		all = append(all, inst)
		allRole = append(allRole, "tiflash")
	}

	fmt.Println("Playground Bootstrapping...")

	monitorInfo := struct {
		IP         string `json:"ip"`
		Port       int    `json:"port"`
		BinaryPath string `json:"binary_path"`
	}{}

	var monitorCmd *exec.Cmd
	if options.monitor {
		if err := installIfMissing(profile, "prometheus", ""); err != nil {
			return err
		}
		var pdAddrs, tidbAddrs, tikvAddrs, tiflashAddrs []string
		for _, pd := range pds {
			pdAddrs = append(pdAddrs, fmt.Sprintf("%s:%d", pd.Host, pd.StatusPort))
		}
		for _, db := range dbs {
			tidbAddrs = append(tidbAddrs, fmt.Sprintf("%s:%d", db.Host, db.StatusPort))
		}
		for _, kv := range kvs {
			tikvAddrs = append(tikvAddrs, fmt.Sprintf("%s:%d", kv.Host, kv.StatusPort))
		}
		for _, flash := range flashs {
			tiflashAddrs = append(tiflashAddrs, fmt.Sprintf("%s:%d", flash.Host, flash.StatusPort))
			tiflashAddrs = append(tiflashAddrs, fmt.Sprintf("%s:%d", flash.Host, flash.ProxyStatusPort))
		}

		promDir := filepath.Join(dataDir, "prometheus")
		port, cmd, err := startMonitor(ctx, options.host, promDir, tidbAddrs, tikvAddrs, pdAddrs, tiflashAddrs)
		if err != nil {
			return err
		}

		monitorInfo.IP = options.host
		monitorInfo.BinaryPath = promDir
		monitorInfo.Port = port

		monitorCmd = cmd
		go func() {
			log, err := os.OpenFile(filepath.Join(promDir, "prom.log"), os.O_WRONLY|os.O_APPEND|os.O_CREATE, os.ModePerm)
			if err != nil {
				fmt.Println("Monitor system start failed", err)
				return
			}
			defer log.Close()

			cmd.Stderr = log
			cmd.Stdout = os.Stdout

			if err := cmd.Start(); err != nil {
				fmt.Println("Monitor system start failed", err)
				return
			}
		}()
	}

	for i, inst := range all {
		if err := inst.Start(context.WithValue(ctx, "has_tiflash", options.tiflashNum > 0), repository.Version(options.version), pathMap[allRole[i]]); err != nil {
			return err
		}
	}

	var succ []string
	for _, db := range dbs {
		if s := checkDB(db.Addr()); s {
			succ = append(succ, db.Addr())
		}
	}

	if len(succ) > 0 {
		fmt.Println(color.GreenString("CLUSTER START SUCCESSFULLY, Enjoy it ^-^"))
		for _, dbAddr := range succ {
			ss := strings.Split(dbAddr, ":")
			fmt.Println(color.GreenString("To connect TiDB: mysql --host %s --port %s -u root", ss[0], ss[1]))
		}
		tag := os.Getenv(localdata.EnvTag)
		if len(tag) > 0 {
			fmt.Println(color.GreenString("Cluster tag: %s, you can restore this cluster via: tiup -T %s playground", tag, tag))
		}
	}

	if pdAddr := pds[0].Addr(); hasDashboard(pdAddr) {
		fmt.Println(color.GreenString("To view the dashboard: http://%s/dashboard", pdAddr))
	}

	if monitorInfo.IP != "" && len(pds) != 0 {
		client, err := newEtcdClient(pds[0].Addr())
		if err == nil && client != nil {
			promBinary, err := json.Marshal(monitorInfo)
			if err == nil {
				_, err = client.Put(context.TODO(), "/topology/prometheus", string(promBinary))
				if err != nil {
					fmt.Println("Set the PD metrics storage failed")
				}
				fmt.Printf(color.GreenString("To view the monitor: http://%s:%d\n", monitorInfo.IP, monitorInfo.Port))
			}
		}
	}

	dumpDSN(dbs)

	go func() {
		sc := make(chan os.Signal, 1)
		signal.Notify(sc,
			syscall.SIGHUP,
			syscall.SIGINT,
			syscall.SIGTERM,
			syscall.SIGQUIT)
		sig := (<-sc).(syscall.Signal)
		for _, inst := range all {
			_ = syscall.Kill(inst.Pid(), sig)
		}
		if monitorCmd != nil {
			_ = syscall.Kill(monitorCmd.Process.Pid, sig)
		}
	}()

	for _, inst := range all {
		if err := inst.Wait(); err != nil {
			return err
		}
	}

	if monitorCmd != nil {
		if err := monitorCmd.Wait(); err != nil {
			fmt.Println("Monitor system wait failed", err)
		}
	}

	return nil
}

func dumpDSN(dbs []*instance.TiDBInstance) {
	dsn := []string{}
	for _, db := range dbs {
		dsn = append(dsn, fmt.Sprintf("mysql://root@%s", db.Addr()))
	}
	_ = ioutil.WriteFile("dsn", []byte(strings.Join(dsn, "\n")), 0644)
}

func addrsToString(addrs []string) string {
	return strings.Join(addrs, "','")
}

func startMonitor(ctx context.Context, host, dir string, dbs, kvs, pds, flashs []string) (int, *exec.Cmd, error) {
	if err := os.MkdirAll(dir, 0755); err != nil {
		return 0, nil, err
	}

	port, err := utils.GetFreePort(host, 9090)
	if err != nil {
		return 0, nil, err
	}
	addr := fmt.Sprintf("%s:%d", host, port)

	tmpl := fmt.Sprintf(`
global:
  scrape_interval:     15s # Set the scrape interval to every 15 seconds. Default is every 1 minute.
  evaluation_interval: 15s # Evaluate rules every 15 seconds. The default is every 1 minute.
  # scrape_timeout is set to the global default (10s).

# Alertmanager configuration
alerting:
  alertmanagers:
  - static_configs:
    - targets:
      # - alertmanager:9093

# Load rules once and periodically evaluate them according to the global 'evaluation_interval'.
rule_files:
  # - "first_rules.yml"
  # - "second_rules.yml"

# A scrape configuration containing exactly one endpoint to scrape:
# Here it's Prometheus itself.
scrape_configs:
  - job_name: 'prometheus'
    static_configs:
    - targets: ['%s']
  - job_name: 'tidb'
    static_configs:
    - targets: ['%s']
  - job_name: 'tikv'
    static_configs:
    - targets: ['%s']
  - job_name: 'pd'
    static_configs:
    - targets: ['%s']
  - job_name: 'tiflash'
    static_configs:
    - targets: ['%s']
`, addr, addrsToString(dbs), addrsToString(kvs), addrsToString(pds), addrsToString(flashs))

	if err := ioutil.WriteFile(filepath.Join(dir, "prometheus.yml"), []byte(tmpl), os.ModePerm); err != nil {
		return 0, nil, err
	}

	args := []string{
		"tiup", "prometheus",
		fmt.Sprintf("--config.file=%s", filepath.Join(dir, "prometheus.yml")),
		fmt.Sprintf("--web.external-url=http://%s", addr),
		fmt.Sprintf("--web.listen-address=0.0.0.0:%d", port),
		fmt.Sprintf("--storage.tsdb.path='%s'", filepath.Join(dir, "data")),
	}
	cmd := exec.CommandContext(ctx, args[0], args[1:]...)
	cmd.Env = append(
		os.Environ(),
		fmt.Sprintf("%s=%s", localdata.EnvNameInstanceDataDir, dir),
	)
	return port, cmd, nil
}

func newEtcdClient(endpoint string) (*clientv3.Client, error) {
	// Because etcd client does not support setting logger directly,
	// the configuration of pingcap/log is copied here.
	zapCfg := zap.NewProductionConfig()
	zapCfg.OutputPaths = []string{"stderr"}
	zapCfg.ErrorOutputPaths = []string{"stderr"}

	client, err := clientv3.New(clientv3.Config{
		Endpoints:   []string{endpoint},
		DialTimeout: 5 * time.Second,
		LogConfig:   &zapCfg,
	})
	if err != nil {
		return nil, err
	}
	return client, nil
}

func main() {
	if err := execute(); err != nil {
		fmt.Println("Playground bootstrapping failed:", err)
		os.Exit(1)
	}
}
