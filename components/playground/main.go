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
	"os/signal"
	"os/user"
	"path/filepath"
	"strconv"
	"strings"
	"sync/atomic"
	"syscall"
	"time"

	"github.com/fatih/color"
	_ "github.com/go-sql-driver/mysql"
	"github.com/pingcap/errors"
	"github.com/pingcap/tiup/components/playground/instance"
	"github.com/pingcap/tiup/pkg/cluster/api"
	"github.com/pingcap/tiup/pkg/environment"
	"github.com/pingcap/tiup/pkg/localdata"
	"github.com/pingcap/tiup/pkg/repository"
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
	ticdc   instance.Config
	pump    instance.Config
	drainer instance.Config
	host    string
	monitor bool
}

func installIfMissing(profile *localdata.Profile, component, version string) error {
	env := environment.GlobalEnv()

	installed, err := env.V1Repository().Local().ComponentInstalled(component, version)
	if err != nil {
		return err
	}
	if installed {
		return nil
	}

	spec := repository.ComponentSpec{
		ID:      component,
		Version: version,
	}
	return env.V1Repository().UpdateComponents([]repository.ComponentSpec{spec})
}

func execute() error {
	opt := &bootOptions{
		tidb: instance.Config{
			Num:       1,
			UpTimeout: 60,
		},
		tikv: instance.Config{
			Num: 1,
		},
		pd: instance.Config{
			Num: 1,
		},
		tiflash: instance.Config{
			Num:       1,
			UpTimeout: 120,
		},
		host:    "127.0.0.1",
		monitor: true,
		version: "",
	}

	rootCmd := &cobra.Command{
		Use: "tiup playground [version]",
		Long: `Bootstrap a TiDB cluster in your local host, the latest release version will be chosen
if you don't specified a version.

Examples:
  $ tiup playground nightly                         # Start a TiDB nightly version local cluster
  $ tiup playground v3.0.10 --db 3 --pd 3 --kv 3    # Start a local cluster with 10 nodes
  $ tiup playground nightly --monitor=false         # Start a local cluster and disable monitor system
  $ tiup playground --pd.config ~/config/pd.toml    # Start a local cluster with specified configuration file,
  $ tiup playground --db.binpath /xx/tidb-server    # Start a local cluster with component binary path`,
		SilenceUsage:  true,
		SilenceErrors: true,
		Args: func(cmd *cobra.Command, args []string) error {
			return nil
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) > 0 {
				opt.version = args[0]
			}

			dataDir := os.Getenv(localdata.EnvNameInstanceDataDir)
			if dataDir == "" {
				return errors.Errorf("cannot read environment variable %s", localdata.EnvNameInstanceDataDir)
			}

			port, err := utils.GetFreePort("0.0.0.0", 9527)
			if err != nil {
				return err
			}
			err = dumpPort(filepath.Join(dataDir, "port"), port)
			p := NewPlayground(dataDir, port)
			if err != nil {
				return err
			}

			env, err := environment.InitEnv(repository.Options{})
			if err != nil {
				return err
			}
			environment.SetGlobalEnv(env)

			var booted uint32
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()

			go func() {
				sc := make(chan os.Signal, 1)
				signal.Notify(sc,
					syscall.SIGHUP,
					syscall.SIGINT,
					syscall.SIGTERM,
					syscall.SIGQUIT,
				)

				sig := (<-sc).(syscall.Signal)
				atomic.StoreInt32(&p.curSig, int32(sig))
				fmt.Println("Playground receive signal: ", sig)

				// if bootCluster is not done we just cancel context to make it
				// clean up and return ASAP and exit directly after timeout.
				// Note now bootCluster can not learn the context is done and return quickly now
				// like while it's downloading component.
				if atomic.LoadUint32(&booted) == 0 {
					cancel()
					time.AfterFunc(time.Second, func() {
						os.Exit(0)
					})
					return
				}

				go p.terminate(sig)
				// If user try double ctrl+c, force quit
				sig = (<-sc).(syscall.Signal)
				atomic.StoreInt32(&p.curSig, int32(syscall.SIGKILL))
				if sig == syscall.SIGINT {
					p.terminate(syscall.SIGKILL)
				}
			}()

			bootErr := p.bootCluster(ctx, env, opt)
			if bootErr != nil {
				// always kill all process started and wait before quit.
				atomic.StoreInt32(&p.curSig, int32(syscall.SIGKILL))
				p.terminate(syscall.SIGKILL)
				_ = p.wait()
				return errors.Annotate(bootErr, "Playground bootstrapping failed")
			}

			atomic.StoreUint32(&booted, 1)

			waitErr := p.wait()
			if waitErr != nil {
				return waitErr
			}

			return nil
		},
	}

	rootCmd.Flags().IntVarP(&opt.tidb.Num, "db", "", opt.tidb.Num, "TiDB instance number")
	rootCmd.Flags().IntVarP(&opt.tikv.Num, "kv", "", opt.tikv.Num, "TiKV instance number")
	rootCmd.Flags().IntVarP(&opt.pd.Num, "pd", "", opt.pd.Num, "PD instance number")
	rootCmd.Flags().IntVarP(&opt.tiflash.Num, "tiflash", "", opt.tiflash.Num, "TiFlash instance number")
	rootCmd.Flags().IntVarP(&opt.ticdc.Num, "ticdc", "", opt.ticdc.Num, "TiCDC instance number")
	rootCmd.Flags().IntVarP(&opt.pump.Num, "pump", "", opt.pump.Num, "Pump instance number")
	rootCmd.Flags().IntVarP(&opt.drainer.Num, "drainer", "", opt.drainer.Num, "Drainer instance number")

	rootCmd.Flags().IntVarP(&opt.tidb.UpTimeout, "db.timeout", "", opt.tidb.UpTimeout, "TiDB max wait time in seconds for starting, 0 means no limit")
	rootCmd.Flags().IntVarP(&opt.tiflash.UpTimeout, "tiflash.timeout", "", opt.tiflash.UpTimeout, "TiFlash max wait time in seconds for starting, 0 means no limit")

	rootCmd.Flags().StringVarP(&opt.host, "host", "", opt.host, "Playground cluster host")
	rootCmd.Flags().StringVarP(&opt.tidb.Host, "db.host", "", opt.tidb.Host, "Playground TiDB host. If not provided, TiDB will still use `host` flag as its host")
	rootCmd.Flags().StringVarP(&opt.pd.Host, "pd.host", "", opt.pd.Host, "Playground PD host. If not provided, PD will still use `host` flag as its host")
	rootCmd.Flags().BoolVar(&opt.monitor, "monitor", opt.monitor, "Start prometheus and grafana component")

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
	rootCmd.Flags().StringVarP(&opt.ticdc.BinPath, "ticdc.binpath", "", opt.ticdc.BinPath, "TiCDC instance binary path")
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

// checkDB check if the addr is connectable by getting a connection from sql.DB. timeout <=0 means no timeout
func checkDB(dbAddr string, timeout int) bool {
	dsn := fmt.Sprintf("root:@tcp(%s)/", dbAddr)
	if timeout > 0 {
		for i := 0; i < timeout; i++ {
			if tryConnect(dsn) == nil {
				return true
			}
			time.Sleep(time.Second)
		}
		return false
	}
	for {
		if err := tryConnect(dsn); err == nil {
			return true
		}
		time.Sleep(time.Second)
	}
}

// checkStoreStatus uses pd client to check whether a store is up. timeout <= 0 means no timeout
func checkStoreStatus(pdClient *api.PDClient, storeAddr string, timeout int) bool {
	if timeout > 0 {
		for i := 0; i < timeout; i++ {
			if up, err := pdClient.IsUp(storeAddr); err == nil && up {
				return true
			}
			time.Sleep(time.Second)
		}
		return false
	}
	for {
		if up, err := pdClient.IsUp(storeAddr); err == nil && up {
			return true
		}
		time.Sleep(time.Second)
	}
}

func hasDashboard(pdAddr string) bool {
	resp, err := http.Get(fmt.Sprintf("http://%s/dashboard", pdAddr))
	if err != nil {
		return false
	}
	defer resp.Body.Close()

	return resp.StatusCode == 200
}

// getAbsolutePath returns the absolute path
func getAbsolutePath(path string) (string, error) {
	if path == "" {
		return "", nil
	}

	if !filepath.IsAbs(path) && !strings.HasPrefix(path, "~/") {
		wd := os.Getenv(localdata.EnvNameWorkDir)
		if wd == "" {
			return "", errors.New("playground running at non-tiup mode")
		}
		path = filepath.Join(wd, path)
	}

	if strings.HasPrefix(path, "~/") {
		usr, err := user.Current()
		if err != nil {
			return "", errors.Annotatef(err, "retrieve user home failed")
		}
		path = filepath.Join(usr.HomeDir, path[2:])
	}

	absPath, err := filepath.Abs(path)
	if err != nil {
		return "", errors.AddStack(err)
	}

	return absPath, nil
}

func dumpPort(fname string, port int) error {
	return ioutil.WriteFile(fname, []byte(strconv.Itoa(port)), 0644)
}

func loadPort(dir string) (port int, err error) {
	data, err := ioutil.ReadFile(filepath.Join(dir, "port"))
	if err != nil {
		return 0, err
	}

	port, err = strconv.Atoi(string(data))
	return
}

func dumpDSN(fname string, dbs []*instance.TiDBInstance) {
	var dsn []string
	for _, db := range dbs {
		dsn = append(dsn, fmt.Sprintf("mysql://root@%s", db.Addr()))
	}
	_ = ioutil.WriteFile(fname, []byte(strings.Join(dsn, "\n")), 0644)
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
		fmt.Println(color.RedString("Error: %v", err))
		os.Exit(1)
	}
}
