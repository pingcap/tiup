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
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
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
	host := "127.0.0.1"
	monitor := false

	rootCmd := &cobra.Command{
		Use: "tiup playground [version]",
		Long: `Bootstrap a TiDB cluster in your local host, the latest release version will be chosen
if you don't specified a version.

Examples:
  $ tiup playground nightly                         # Start a TiDB nightly version local cluster
  $ tiup playground v3.0.10 --db 3 --pd 3 --kv 3    # Start a local cluster with 10 nodes
  $ tiup playground nightly --monitor               # Start a local cluster with monitor system`,
		SilenceUsage: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			version := ""
			if len(args) > 0 {
				version = args[0]
			}
			return bootCluster(version, pdNum, tidbNum, tikvNum, host, monitor)
		},
	}

	rootCmd.Flags().IntVarP(&tidbNum, "db", "", 1, "TiDB instance number")
	rootCmd.Flags().IntVarP(&tikvNum, "kv", "", 1, "TiKV instance number")
	rootCmd.Flags().IntVarP(&pdNum, "pd", "", 1, "PD instance number")
	rootCmd.Flags().StringVarP(&host, "host", "", host, "Playground cluster host")
	rootCmd.Flags().BoolVar(&monitor, "monitor", false, "Start prometheus component")

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

func bootCluster(version string, pdNum, tidbNum, tikvNum int, host string, monitor bool) error {
	if pdNum < 1 || tidbNum < 1 || tikvNum < 1 {
		return fmt.Errorf("all components count must be great than 0 (tidb=%v, tikv=%v, pd=%v)",
			tidbNum, tikvNum, pdNum)
	}

	// Initialize the profile
	profileRoot := os.Getenv(localdata.EnvNameHome)
	if profileRoot == "" {
		return fmt.Errorf("cannot read environment variable %s", localdata.EnvNameHome)
	}
	profile := localdata.NewProfile(profileRoot)
	for _, comp := range []string{"pd", "tikv", "tidb"} {
		if err := installIfMissing(profile, comp, version); err != nil {
			return err
		}
	}
	dataDir := os.Getenv(localdata.EnvNameInstanceDataDir)
	if dataDir == "" {
		return fmt.Errorf("cannot read environment variable %s", localdata.EnvNameInstanceDataDir)
	}

	all := make([]instance.Instance, 0, pdNum+tikvNum+tidbNum)
	pds := make([]*instance.PDInstance, 0, pdNum)
	kvs := make([]*instance.TiKVInstance, 0, tikvNum)
	dbs := make([]*instance.TiDBInstance, 0, tidbNum)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	for i := 0; i < pdNum; i++ {
		dir := filepath.Join(dataDir, fmt.Sprintf("pd-%d", i))
		inst := instance.NewPDInstance(dir, host, i)
		pds = append(pds, inst)
		all = append(all, inst)
	}
	for _, pd := range pds {
		pd.Join(pds)
	}

	for i := 0; i < tikvNum; i++ {
		dir := filepath.Join(dataDir, fmt.Sprintf("tikv-%d", i))
		inst := instance.NewTiKVInstance(dir, host, i, pds)
		kvs = append(kvs, inst)
		all = append(all, inst)
	}

	for i := 0; i < tidbNum; i++ {
		dir := filepath.Join(dataDir, fmt.Sprintf("tidb-%d", i))
		inst := instance.NewTiDBInstance(dir, host, i, pds)
		dbs = append(dbs, inst)
		all = append(all, inst)
	}

	fmt.Println("Playground Bootstrapping...")

	var monitorAddr string
	if monitor {
		if err := installIfMissing(profile, "prometheus", ""); err != nil {
			return err
		}
		var pdAddrs, tidbAddrs, tikvAddrs []string
		for _, pd := range pds {
			pdAddrs = append(pdAddrs, fmt.Sprintf("%s:%d", pd.Host, pd.StatusPort))
		}
		for _, db := range dbs {
			tidbAddrs = append(tidbAddrs, fmt.Sprintf("%s:%d", db.Host, db.StatusPort))
		}
		for _, kv := range kvs {
			tikvAddrs = append(tikvAddrs, fmt.Sprintf("%s:%d", kv.Host, kv.StatusPort))
		}

		promDir := filepath.Join(dataDir, "prometheus")
		addr, cmd, err := startMonitor(ctx, host, promDir, tidbAddrs, tikvAddrs, pdAddrs)
		if err != nil {
			return err
		}

		monitorAddr = addr
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
			if err := cmd.Wait(); err != nil {
				fmt.Println("Monitor system wait failed", err)
			}
		}()
	}

	for _, inst := range all {
		if err := inst.Start(ctx, repository.Version(version)); err != nil {
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

	if monitorAddr != "" && len(pds) != 0 {
		client, err := newEtcdClient(pds[0].Addr())
		if err == nil {
			_, err = client.Put(context.TODO(), "/topology/prometheus", monitorAddr)
			if err != nil {
				fmt.Println("Set the PD metrics storage failed")
			}
			fmt.Printf(color.GreenString("To view the monitor: http://%s\n", monitorAddr))
		}
	}

	dumpDSN(dbs)

	for _, inst := range all {
		if err := inst.Wait(); err != nil {
			return err
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

func startMonitor(ctx context.Context, host, dir string, dbs, kvs, pds []string) (string, *exec.Cmd, error) {
	if err := os.MkdirAll(dir, 0755); err != nil {
		return "", nil, err
	}

	port, err := utils.GetFreePort(host, 9090)
	if err != nil {
		return "", nil, err
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
`, addr, strings.Join(dbs, "','"), strings.Join(kvs, "','"), strings.Join(pds, "','"))

	if err := ioutil.WriteFile(filepath.Join(dir, "prometheus.yml"), []byte(tmpl), os.ModePerm); err != nil {
		return "", nil, err
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
	return addr, cmd, nil
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
