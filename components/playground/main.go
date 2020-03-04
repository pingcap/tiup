package main

import (
	"context"
	"database/sql"
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/c4pt0r/tiup/components/playground/instance"
	"github.com/c4pt0r/tiup/pkg/localdata"
	_ "github.com/go-sql-driver/mysql"
	"github.com/spf13/cobra"
)

func installIfMissing(profile *localdata.Profile, component string) error {
	versions, err := profile.InstalledVersions(component)
	if err != nil {
		return err
	}
	if len(versions) > 0 {
		return nil
	}
	c := exec.Command("tiup", "install", component)
	c.Stdout = os.Stdout
	c.Stderr = os.Stderr
	return c.Run()
}

func execute() error {
	tidbNum := 1
	tikvNum := 1
	pdNum := 1

	rootCmd := &cobra.Command{
		Use:   "playground",
		Short: "Bootstrap a TiDB cluster in your local host",
		RunE: func(cmd *cobra.Command, args []string) error {
			return bootCluster(tidbNum, tikvNum, pdNum)
		},
	}

	rootCmd.Flags().IntVarP(&tidbNum, "db", "", 1, "TiDB instance number")
	rootCmd.Flags().IntVarP(&tikvNum, "kv", "", 1, "TiKV instance number")
	rootCmd.Flags().IntVarP(&pdNum, "pd", "", 1, "PD instance number")

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

func bootstrap(dbAddr, pdAddr string) {
	dsn := fmt.Sprintf("root:@tcp(%s)/", dbAddr)
	for i := 0; i < 60; i++ {
		if err := tryConnect(dsn); err != nil {
			time.Sleep(time.Second)
		} else {
			fmt.Println("TiDB cluster has run successfully")
			ss := strings.Split(dbAddr, ":")
			fmt.Printf("To connect TiDB: mysql --host %s --port %s -u root\n", ss[0], ss[1])
			if hasDashboard(pdAddr) {
				fmt.Printf("To view the dashboard: http://%s/dashboard\n", pdAddr)
			}
			break
		}
	}
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

func bootCluster(pdNum, tidbNum, tikvNum int) error {
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
		if err := installIfMissing(profile, comp); err != nil {
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
		inst := instance.NewPDInstance(dir, i)
		pds = append(pds, inst)
		all = append(all, inst)
	}
	for _, pd := range pds {
		pd.Join(pds)
	}

	for i := 0; i < tikvNum; i++ {
		dir := filepath.Join(dataDir, fmt.Sprintf("tikv-%d", i))
		inst := instance.NewTiKVInstance(dir, i, pds)
		kvs = append(kvs, inst)
		all = append(all, inst)
	}

	for i := 0; i < tidbNum; i++ {
		dir := filepath.Join(dataDir, fmt.Sprintf("tidb-%d", i))
		inst := instance.NewTiDBInstance(dir, i, pds)
		dbs = append(dbs, inst)
		all = append(all, inst)
	}

	fmt.Println("Playground Bootstrapping...")

	for _, inst := range all {
		if err := inst.Start(ctx); err != nil {
			return err
		}
	}

	bootstrap(dbs[0].Addr(), pds[0].Addr())

	for _, inst := range all {
		if err := inst.Wait(); err != nil {
			return err
		}
	}
	return nil
}

func main() {
	if err := execute(); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}
