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
	"runtime"
	"strings"
	"syscall"
	"time"

	"github.com/fatih/color"
	_ "github.com/go-sql-driver/mysql"
	"github.com/pingcap/failpoint"
	"github.com/pingcap/tiup/components/playground/instance"
	"github.com/pingcap/tiup/pkg/cluster/api"
	"github.com/pingcap/tiup/pkg/localdata"
	"github.com/pingcap/tiup/pkg/repository/v0manifest"
	"github.com/pingcap/tiup/pkg/utils"
	"github.com/spf13/cobra"
	"go.etcd.io/etcd/clientv3"
	"go.uber.org/zap"
	"golang.org/x/mod/semver"
)

type bootOptions struct {
	version string
	pd      instance.Config
	tidb    instance.Config
	tikv    instance.Config
	tiflash instance.Config
	host    string
	monitor bool
}

func installIfMissing(profile *localdata.Profile, component, version string) error {
	versions, err := profile.InstalledVersions(component)
	if err != nil {
		return err
	}
	if len(versions) > 0 {
		if v0manifest.Version(version).IsEmpty() {
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
	if !v0manifest.Version(version).IsEmpty() {
		spec = fmt.Sprintf("%s:%s", component, version)
	}
	c := exec.Command("tiup", "install", spec)
	c.Stdout = os.Stdout
	c.Stderr = os.Stderr
	return c.Run()
}

func execute() error {
	var defaultTiflashNum int
	if runtime.GOOS == "linux" {
		defaultTiflashNum = 1
	} else {
		defaultTiflashNum = 0
	}

	opt := &bootOptions{
		tidb: instance.Config{
			Num: 1,
		},
		tikv: instance.Config{
			Num: 1,
		},
		pd: instance.Config{
			Num: 1,
		},
		tiflash: instance.Config{
			Num: defaultTiflashNum,
		},
		host:    "127.0.0.1",
		monitor: false,
		version: "",
	}

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
			if len(args) > 0 {
				opt.version = args[0]
			}
			return bootCluster(opt)
		},
	}

	rootCmd.Flags().IntVarP(&opt.tidb.Num, "db", "", opt.tidb.Num, "TiDB instance number")
	rootCmd.Flags().IntVarP(&opt.tikv.Num, "kv", "", opt.tikv.Num, "TiKV instance number")
	rootCmd.Flags().IntVarP(&opt.pd.Num, "pd", "", opt.pd.Num, "PD instance number")
	rootCmd.Flags().IntVarP(&opt.tiflash.Num, "tiflash", "", opt.tiflash.Num, "TiFlash instance number")
	rootCmd.Flags().StringVarP(&opt.host, "host", "", opt.host, "Playground cluster host")
	rootCmd.Flags().StringVarP(&opt.tidb.Host, "db.host", "", opt.tidb.Host, "Playground TiDB host. If not provided, TiDB will still use `host` flag as its host")
	rootCmd.Flags().StringVarP(&opt.pd.Host, "pd.host", "", opt.pd.Host, "Playground PD host. If not provided, PD will still use `host` flag as its host")
	rootCmd.Flags().BoolVar(&opt.monitor, "monitor", false, "Start prometheus component")
	rootCmd.Flags().StringVarP(&opt.tidb.ConfigPath, "db.config", "", opt.tidb.ConfigPath, "TiDB instance configuration file")
	rootCmd.Flags().StringVarP(&opt.tikv.ConfigPath, "kv.config", "", opt.tikv.ConfigPath, "TiKV instance configuration file")
	rootCmd.Flags().StringVarP(&opt.pd.ConfigPath, "pd.config", "", opt.pd.ConfigPath, "PD instance configuration file")
	rootCmd.Flags().StringVarP(&opt.tidb.ConfigPath, "tiflash.config", "", opt.tidb.ConfigPath, "TiFlash instance configuration file")
	rootCmd.Flags().StringVarP(&opt.tidb.BinPath, "db.binpath", "", opt.tidb.BinPath, "TiDB instance binary path")
	rootCmd.Flags().StringVarP(&opt.tikv.BinPath, "kv.binpath", "", opt.tikv.BinPath, "TiKV instance binary path")
	rootCmd.Flags().StringVarP(&opt.pd.BinPath, "pd.binpath", "", opt.pd.BinPath, "PD instance binary path")
	rootCmd.Flags().StringVarP(&opt.tiflash.BinPath, "tiflash.binpath", "", opt.tiflash.BinPath, "TiFlash instance binary path")

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

func checkStoreStatus(pdClient *api.PDClient, storeAddr string) error {
	for i := 0; i < 180; i++ {
		up, err := pdClient.IsUp(storeAddr)
		if err != nil || !up {
			time.Sleep(time.Second)
			fmt.Print(".")
		} else {
			if i != 0 {
				fmt.Println()
			}
			return nil
		}
	}
	return fmt.Errorf(fmt.Sprintf("store %s failed to up after timeout(180s)", storeAddr))
}

func hasDashboard(pdAddr string) bool {
	resp, err := http.Get(fmt.Sprintf("http://%s/dashboard", pdAddr))
	if err != nil {
		return false
	}
	defer resp.Body.Close()

	return resp.StatusCode == 200
}

func getAbsolutePath(binPath string) string {
	if !strings.HasPrefix(binPath, "/") && !strings.HasPrefix(binPath, "~") {
		binPath = filepath.Join(os.Getenv(localdata.EnvNameWorkDir), binPath)
	}
	return binPath
}

func bootCluster(options *bootOptions) error {
	if options.pd.Num < 1 || options.tidb.Num < 1 || options.tikv.Num < 1 {
		return fmt.Errorf("all components count must be great than 0 (tidb=%v, tikv=%v, pd=%v)",
			options.pd.Num, options.tikv.Num, options.pd.Num)
	}

	var pathMap = make(map[string]string)
	if options.tidb.BinPath != "" {
		pathMap["tidb"] = getAbsolutePath(options.tidb.BinPath)
	}
	if options.tikv.BinPath != "" {
		pathMap["tikv"] = getAbsolutePath(options.tikv.BinPath)
	}
	if options.pd.BinPath != "" {
		pathMap["pd"] = getAbsolutePath(options.pd.BinPath)
	}
	if options.tiflash.Num > 0 && options.tiflash.BinPath != "" {
		pathMap["tiflash"] = getAbsolutePath(options.tiflash.BinPath)
	}

	if options.tidb.ConfigPath != "" {
		options.tidb.ConfigPath = getAbsolutePath(options.tidb.ConfigPath)
	}
	if options.tikv.ConfigPath != "" {
		options.tikv.ConfigPath = getAbsolutePath(options.tikv.ConfigPath)
	}
	if options.pd.ConfigPath != "" {
		options.pd.ConfigPath = getAbsolutePath(options.pd.ConfigPath)
	}
	if options.tiflash.Num > 0 && options.tiflash.ConfigPath != "" {
		options.tiflash.ConfigPath = getAbsolutePath(options.tiflash.ConfigPath)
	}

	profile := localdata.InitProfile()

	if options.version != "" && semver.Compare("v3.1.0", options.version) > 0 && options.tiflash.Num != 0 {
		fmt.Println(color.YellowString("Warning: current version %s doesn't support TiFlash", options.version))
		options.tiflash.Num = 0
	}

	for _, comp := range []string{"pd", "tikv", "tidb", "tiflash"} {
		if pathMap[comp] != "" {
			continue
		}
		if comp == "tiflash" && options.tiflash.Num == 0 {
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

	all := make([]instance.Instance, 0)
	allButTiFlash := make([]instance.Instance, 0)
	allRolesButTiFlash := make([]string, 0)
	pds := make([]*instance.PDInstance, 0)
	kvs := make([]*instance.TiKVInstance, 0)
	dbs := make([]*instance.TiDBInstance, 0)
	flashs := make([]*instance.TiFlashInstance, 0)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	pdHost := options.host
	// If pdHost flag is specified, use it instead.
	if options.pd.Host != "" {
		pdHost = options.pd.Host
	}
	for i := 0; i < options.pd.Num; i++ {
		dir := filepath.Join(dataDir, fmt.Sprintf("pd-%d", i))
		inst := instance.NewPDInstance(dir, pdHost, options.pd.ConfigPath, i)
		pds = append(pds, inst)
		all = append(all, inst)
		allButTiFlash = append(allButTiFlash, inst)
		allRolesButTiFlash = append(allRolesButTiFlash, "pd")
	}
	for _, pd := range pds {
		pd.Join(pds)
	}

	for i := 0; i < options.tikv.Num; i++ {
		dir := filepath.Join(dataDir, fmt.Sprintf("tikv-%d", i))
		inst := instance.NewTiKVInstance(dir, options.host, options.tikv.ConfigPath, i, pds)
		kvs = append(kvs, inst)
		all = append(all, inst)
		allButTiFlash = append(allButTiFlash, inst)
		allRolesButTiFlash = append(allRolesButTiFlash, "tikv")
	}

	tidbHost := options.host
	// If tidbHost flag is specified, use it instead.
	if options.tidb.Host != "" {
		tidbHost = options.tidb.Host
	}
	for i := 0; i < options.tidb.Num; i++ {
		dir := filepath.Join(dataDir, fmt.Sprintf("tidb-%d", i))
		inst := instance.NewTiDBInstance(dir, tidbHost, options.tidb.ConfigPath, i, pds)
		dbs = append(dbs, inst)
		allButTiFlash = append(allButTiFlash, inst)
		allRolesButTiFlash = append(allRolesButTiFlash, "tidb")
	}

	for i := 0; i < options.tiflash.Num; i++ {
		dir := filepath.Join(dataDir, fmt.Sprintf("tiflash-%d", i))
		inst := instance.NewTiFlashInstance(dir, options.host, options.tiflash.ConfigPath, i, pds, dbs)
		flashs = append(flashs, inst)
		all = append(all, inst)
	}

	fmt.Println("Playground Bootstrapping...")

	monitorInfo := struct {
		IP         string `json:"ip"`
		Port       int    `json:"port"`
		BinaryPath string `json:"binary_path"`
	}{}

	var monitorCmd *exec.Cmd
	if options.monitor {
		if err := installIfMissing(profile, "prometheus", options.version); err != nil {
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

	for i, inst := range allButTiFlash {
		if err := inst.Start(ctx, v0manifest.Version(options.version), pathMap[allRolesButTiFlash[i]]); err != nil {
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
		// start TiFlash after at least one TiDB is up.
		var lastErr error

		var endpoints []string
		for _, pd := range pds {
			endpoints = append(endpoints, pd.Addr())
		}
		pdClient := api.NewPDClient(endpoints, 10*time.Second, nil)

		// make sure TiKV are all up
		for _, kv := range kvs {
			if err := checkStoreStatus(pdClient, kv.StoreAddr()); err != nil {
				lastErr = err
				break
			}
		}

		if lastErr == nil {
			for _, flash := range flashs {
				if err := flash.Start(ctx, v0manifest.Version(options.version), pathMap["tiflash"]); err != nil {
					lastErr = err
					break
				}
			}
		}

		if lastErr == nil {
			// check if all TiFlash is up
			for _, flash := range flashs {
				if err := checkStoreStatus(pdClient, flash.StoreAddr()); err != nil {
					lastErr = err
					break
				}
			}
		}

		if lastErr != nil {
			fmt.Println(color.RedString("TiFlash failed to start. %s", lastErr))
		} else {
			fmt.Println(color.GreenString("CLUSTER START SUCCESSFULLY, Enjoy it ^-^"))
			for _, dbAddr := range succ {
				ss := strings.Split(dbAddr, ":")
				fmt.Println(color.GreenString("To connect TiDB: mysql --host %s --port %s -u root", ss[0], ss[1]))
			}
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
				fmt.Print(color.GreenString("To view the monitor: http://%s:%d\n", monitorInfo.IP, monitorInfo.Port))
			}
		}
	}

	dumpDSN(dbs)

	failpoint.Inject("terminateEarly", func() error {
		time.Sleep(20 * time.Second)

		fmt.Println("Early terminated via failpoint")
		for _, inst := range all {
			_ = syscall.Kill(inst.Pid(), syscall.SIGKILL)
		}
		if monitorCmd != nil {
			_ = syscall.Kill(monitorCmd.Process.Pid, syscall.SIGKILL)
		}

		fmt.Println("Wait all processes terminated")
		for _, inst := range all {
			_ = inst.Wait()
		}
		if monitorCmd != nil {
			_ = monitorCmd.Wait()
		}
		return nil
	})

	go func() {
		sc := make(chan os.Signal, 1)
		signal.Notify(sc,
			syscall.SIGHUP,
			syscall.SIGINT,
			syscall.SIGTERM,
			syscall.SIGQUIT)
		sig := (<-sc).(syscall.Signal)
		if sig != syscall.SIGINT {
			for _, inst := range all {
				_ = syscall.Kill(inst.Pid(), sig)
			}
			if monitorCmd != nil {
				_ = syscall.Kill(monitorCmd.Process.Pid, sig)
			}
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
	var dsn []string
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
