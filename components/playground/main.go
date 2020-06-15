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
	_ "net/http/pprof"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"time"

	"github.com/fatih/color"
	_ "github.com/go-sql-driver/mysql"
	"github.com/pingcap/errors"
	"github.com/pingcap/tiup/components/playground/instance"
	"github.com/pingcap/tiup/pkg/cluster/api"
	"github.com/pingcap/tiup/pkg/localdata"
	"github.com/pingcap/tiup/pkg/repository/v0manifest"
	"github.com/pingcap/tiup/pkg/utils"
	"github.com/spf13/cobra"
	"go.etcd.io/etcd/clientv3"
	"go.uber.org/zap"
)

type bootOptions struct {
	version string
	pd      instance.Config
	tidb    instance.Config
	tikv    instance.Config
	tiflash instance.Config
	pump    instance.Config
	drainer instance.Config
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
		Args: func(cmd *cobra.Command, args []string) error {
			return nil
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) > 0 {
				opt.version = args[0]
			}

			port, err := utils.GetFreePort("0.0.0.0", 9527)
			if err != nil {
				return errors.AddStack(err)
			}
			err = dumpPort(port)
			p := NewPlayground(port)
			if err != nil {
				return errors.AddStack(err)
			}
			return p.bootCluster(opt)
		},
	}

	rootCmd.Flags().IntVarP(&opt.tidb.Num, "db", "", opt.tidb.Num, "TiDB instance number")
	rootCmd.Flags().IntVarP(&opt.tikv.Num, "kv", "", opt.tikv.Num, "TiKV instance number")
	rootCmd.Flags().IntVarP(&opt.pd.Num, "pd", "", opt.pd.Num, "PD instance number")
	rootCmd.Flags().IntVarP(&opt.tiflash.Num, "tiflash", "", opt.tiflash.Num, "TiFlash instance number")
	rootCmd.Flags().IntVarP(&opt.pump.Num, "pump", "", opt.pump.Num, "Pump instance number")
	rootCmd.Flags().IntVarP(&opt.drainer.Num, "drainer", "", opt.drainer.Num, "Drainer instance number")

	rootCmd.Flags().StringVarP(&opt.host, "host", "", opt.host, "Playground cluster host")
	rootCmd.Flags().StringVarP(&opt.tidb.Host, "db.host", "", opt.tidb.Host, "Playground TiDB host. If not provided, TiDB will still use `host` flag as its host")
	rootCmd.Flags().StringVarP(&opt.pd.Host, "pd.host", "", opt.pd.Host, "Playground PD host. If not provided, PD will still use `host` flag as its host")
	rootCmd.Flags().BoolVar(&opt.monitor, "monitor", false, "Start prometheus component")

	rootCmd.Flags().StringVarP(&opt.tidb.ConfigPath, "db.config", "", opt.tidb.ConfigPath, "TiDB instance configuration file")
	rootCmd.Flags().StringVarP(&opt.tikv.ConfigPath, "kv.config", "", opt.tikv.ConfigPath, "TiKV instance configuration file")
	rootCmd.Flags().StringVarP(&opt.pd.ConfigPath, "pd.config", "", opt.pd.ConfigPath, "PD instance configuration file")
	rootCmd.Flags().StringVarP(&opt.tidb.ConfigPath, "tiflash.config", "", opt.tidb.ConfigPath, "TiFlash instance configuration file")
	rootCmd.Flags().StringVarP(&opt.pump.ConfigPath, "pump.config", "", opt.pump.ConfigPath, "Pump instance configuration file")
	rootCmd.Flags().StringVarP(&opt.drainer.ConfigPath, "drainer.config", "", opt.drainer.ConfigPath, "Drainer instance configuration file")

	rootCmd.Flags().StringVarP(&opt.tidb.BinPath, "db.binpath", "", opt.tidb.BinPath, "TiDB instance binary path")
	rootCmd.Flags().StringVarP(&opt.tikv.BinPath, "kv.binpath", "", opt.tikv.BinPath, "TiKV instance binary path")
	rootCmd.Flags().StringVarP(&opt.pd.BinPath, "pd.binpath", "", opt.pd.BinPath, "PD instance binary path")
	rootCmd.Flags().StringVarP(&opt.tiflash.BinPath, "tiflash.binpath", "", opt.tiflash.BinPath, "TiFlash instance binary path")
	rootCmd.Flags().StringVarP(&opt.pump.BinPath, "pump.binpath", "", opt.pump.BinPath, "Pump instance binary path")
	rootCmd.Flags().StringVarP(&opt.drainer.BinPath, "drainer.binpath", "", opt.drainer.BinPath, "Drainer instance binary path")

	rootCmd.AddCommand(newDisplay())
	rootCmd.AddCommand(newScaleOut())
	rootCmd.AddCommand(newScaleIn())

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

func checkStoreStatus(pdClient *api.PDClient, typ, storeAddr string) error {
	fmt.Print(color.YellowString("Waiting for %s %s ready ", typ, storeAddr))
	for i := 0; i < 180; i++ {
		up, err := pdClient.IsUp(storeAddr)
		if err != nil || !up {
			time.Sleep(time.Second)
			fmt.Print(color.YellowString("."))
		} else {
			fmt.Println()
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

func dumpPort(port int) error {
	return ioutil.WriteFile("port", []byte(strconv.Itoa(port)), 0644)
}

func loadPort(dir string) (port int, err error) {
	data, err := ioutil.ReadFile(filepath.Join(dir, "port"))
	if err != nil {
		return 0, err
	}

	port, err = strconv.Atoi(string(data))
	return
}

func dumpDSN(dbs []*instance.TiDBInstance) {
	var dsn []string
	for _, db := range dbs {
		dsn = append(dsn, fmt.Sprintf("mysql://root@%s", db.Addr()))
	}
	_ = ioutil.WriteFile("dsn", []byte(strings.Join(dsn, "\n")), 0644)
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
		fmt.Printf("Playground bootstrapping failed: %v\n", err)
		os.Exit(1)
	}
}
