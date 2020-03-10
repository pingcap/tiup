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

	_ "github.com/go-sql-driver/mysql"
	"github.com/pingcap-incubator/tiup/components/playground/instance"
	"github.com/pingcap-incubator/tiup/pkg/localdata"
	"github.com/pingcap-incubator/tiup/pkg/meta"
	"github.com/pingcap-incubator/tiup/pkg/utils"
	"github.com/spf13/cobra"
)

func installIfMissing(profile *localdata.Profile, component, version string) error {
	versions, err := profile.InstalledVersions(component)
	if err != nil {
		return err
	}
	if len(versions) > 0 {
		if meta.Version(version).IsEmpty() {
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
	if !meta.Version(version).IsEmpty() {
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
		Use:          "playground",
		Short:        "Bootstrap a TiDB cluster in your local host",
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
		} else {
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
		if err := inst.Start(ctx, meta.Version(version)); err != nil {
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
		fmt.Println("\x1b[032mCLUSTER START SUCCESSFULLY, Enjoy it ^-^\x1b[0m")
		for _, dbAddr := range succ {
			ss := strings.Split(dbAddr, ":")
			fmt.Printf("\x1b[032mTo connect TiDB: mysql --host %s --port %s -u root\x1b[0m\n", ss[0], ss[1])
		}
	}

	if pdAddr := pds[0].Addr(); hasDashboard(pdAddr) {
		fmt.Printf("To view the dashboard: http://%s/dashboard\n", pdAddr)
	}

	if monitorAddr != "" {
		addr := fmt.Sprintf("http://%s/pd/api/v1/config", pds[0].Addr())
		cfg := fmt.Sprintf(`{"metric-storage":"http://%s"}`, monitorAddr)
		resp, err := http.Post(addr, "", strings.NewReader(cfg))
		if err != nil || resp.StatusCode != http.StatusOK {
			fmt.Println("Set the PD metrics storage failed")
		}
		fmt.Printf("To view the monitor: http://%s\n", monitorAddr)
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
		"tiup", "prometheus", "--",
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

func main() {
	if err := execute(); err != nil {
		fmt.Println("Playground bootstrapping failed:", err)
		os.Exit(1)
	}
}
