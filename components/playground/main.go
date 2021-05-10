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
	"github.com/google/uuid"
	"github.com/pingcap/errors"
	"github.com/pingcap/tiup/components/playground/instance"
	"github.com/pingcap/tiup/pkg/cluster/api"
	"github.com/pingcap/tiup/pkg/environment"
	"github.com/pingcap/tiup/pkg/flags"
	"github.com/pingcap/tiup/pkg/localdata"
	"github.com/pingcap/tiup/pkg/logger/log"
	"github.com/pingcap/tiup/pkg/repository"
	"github.com/pingcap/tiup/pkg/telemetry"
	"github.com/pingcap/tiup/pkg/utils"
	"github.com/pingcap/tiup/pkg/version"
	"github.com/spf13/cobra"
	"go.etcd.io/etcd/clientv3"
	"go.uber.org/zap"
	"gopkg.in/yaml.v3"
)

// BootOptions is the topology and options used to start a playground cluster
type BootOptions struct {
	Version string          `yaml:"version"`
	PD      instance.Config `yaml:"pd"`
	TiDB    instance.Config `yaml:"tidb"`
	TiKV    instance.Config `yaml:"tikv"`
	TiFlash instance.Config `yaml:"tiflash"`
	TiCDC   instance.Config `yaml:"ticdc"`
	Pump    instance.Config `yaml:"pump"`
	Drainer instance.Config `yaml:"drainer"`
	Host    string          `yaml:"host"`
	Monitor bool            `yaml:"monitor"`
}

var (
	reportEnabled    bool // is telemetry report enabled
	teleReport       *telemetry.Report
	playgroundReport *telemetry.PlaygroundReport
	opt              = &BootOptions{
		TiDB: instance.Config{
			Num:       1,
			UpTimeout: 60,
		},
		TiKV: instance.Config{
			Num: 1,
		},
		PD: instance.Config{
			Num: 1,
		},
		TiFlash: instance.Config{
			Num:       1,
			UpTimeout: 120,
		},
		Host:    "127.0.0.1",
		Monitor: true,
		Version: "",
	}
)

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
	rootCmd := &cobra.Command{
		Use: "tiup playground [version]",
		Long: `Bootstrap a TiDB cluster in your local host, the latest release version will be chosen
if you don't specified a version.

Examples:
  $ tiup playground nightly                         # Start a TiDB nightly version local cluster
  $ tiup playground v5.0.1 --db 3 --pd 3 --kv 3     # Start a local cluster with 10 nodes
  $ tiup playground nightly --monitor=false         # Start a local cluster and disable monitor system
  $ tiup playground --pd.config ~/config/pd.toml    # Start a local cluster with specified configuration file,
  $ tiup playground --db.binpath /xx/tidb-server    # Start a local cluster with component binary path`,
		SilenceUsage:  true,
		SilenceErrors: true,
		Version:       version.NewTiUPVersion().String(),
		Args: func(cmd *cobra.Command, args []string) error {
			return nil
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			teleReport = new(telemetry.Report)
			playgroundReport = new(telemetry.PlaygroundReport)
			teleReport.EventDetail = &telemetry.Report_Playground{Playground: playgroundReport}
			reportEnabled = telemetry.Enabled()
			if reportEnabled {
				eventUUID := os.Getenv(localdata.EnvNameTelemetryEventUUID)
				if eventUUID == "" {
					eventUUID = uuid.New().String()
				}
				teleReport.InstallationUUID = telemetry.GetUUID()
				teleReport.EventUUID = eventUUID
				teleReport.EventUnixTimestamp = time.Now().Unix()
				teleReport.Version = telemetry.TiUPMeta()
			}

			if len(args) > 0 {
				opt.Version = args[0]
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

	rootCmd.Flags().IntVarP(&opt.TiDB.Num, "db", "", opt.TiDB.Num, "TiDB instance number")
	rootCmd.Flags().IntVarP(&opt.TiKV.Num, "kv", "", opt.TiKV.Num, "TiKV instance number")
	rootCmd.Flags().IntVarP(&opt.PD.Num, "pd", "", opt.PD.Num, "PD instance number")
	rootCmd.Flags().IntVarP(&opt.TiFlash.Num, "tiflash", "", opt.TiFlash.Num, "TiFlash instance number")
	rootCmd.Flags().IntVarP(&opt.TiCDC.Num, "ticdc", "", opt.TiCDC.Num, "TiCDC instance number")
	rootCmd.Flags().IntVarP(&opt.Pump.Num, "pump", "", opt.Pump.Num, "Pump instance number")
	rootCmd.Flags().IntVarP(&opt.Drainer.Num, "drainer", "", opt.Drainer.Num, "Drainer instance number")

	rootCmd.Flags().IntVarP(&opt.TiDB.UpTimeout, "db.timeout", "", opt.TiDB.UpTimeout, "TiDB max wait time in seconds for starting, 0 means no limit")
	rootCmd.Flags().IntVarP(&opt.TiFlash.UpTimeout, "tiflash.timeout", "", opt.TiFlash.UpTimeout, "TiFlash max wait time in seconds for starting, 0 means no limit")

	rootCmd.Flags().StringVarP(&opt.Host, "host", "", opt.Host, "Playground cluster host")
	rootCmd.Flags().StringVarP(&opt.TiDB.Host, "db.host", "", opt.TiDB.Host, "Playground TiDB host. If not provided, TiDB will still use `host` flag as its host")
	rootCmd.Flags().StringVarP(&opt.PD.Host, "pd.host", "", opt.PD.Host, "Playground PD host. If not provided, PD will still use `host` flag as its host")
	rootCmd.Flags().BoolVar(&opt.Monitor, "monitor", opt.Monitor, "Start prometheus and grafana component")

	rootCmd.Flags().StringVarP(&opt.TiDB.ConfigPath, "db.config", "", opt.TiDB.ConfigPath, "TiDB instance configuration file")
	rootCmd.Flags().StringVarP(&opt.TiKV.ConfigPath, "kv.config", "", opt.TiKV.ConfigPath, "TiKV instance configuration file")
	rootCmd.Flags().StringVarP(&opt.PD.ConfigPath, "pd.config", "", opt.PD.ConfigPath, "PD instance configuration file")
	rootCmd.Flags().StringVarP(&opt.TiDB.ConfigPath, "tiflash.config", "", opt.TiDB.ConfigPath, "TiFlash instance configuration file")
	rootCmd.Flags().StringVarP(&opt.Pump.ConfigPath, "pump.config", "", opt.Pump.ConfigPath, "Pump instance configuration file")
	rootCmd.Flags().StringVarP(&opt.Drainer.ConfigPath, "drainer.config", "", opt.Drainer.ConfigPath, "Drainer instance configuration file")
	rootCmd.Flags().StringVarP(&opt.TiCDC.ConfigPath, "ticdc.config", "", opt.TiCDC.ConfigPath, "TiCDC instance configuration file")

	rootCmd.Flags().StringVarP(&opt.TiDB.BinPath, "db.binpath", "", opt.TiDB.BinPath, "TiDB instance binary path")
	rootCmd.Flags().StringVarP(&opt.TiKV.BinPath, "kv.binpath", "", opt.TiKV.BinPath, "TiKV instance binary path")
	rootCmd.Flags().StringVarP(&opt.PD.BinPath, "pd.binpath", "", opt.PD.BinPath, "PD instance binary path")
	rootCmd.Flags().StringVarP(&opt.TiFlash.BinPath, "tiflash.binpath", "", opt.TiFlash.BinPath, "TiFlash instance binary path")
	rootCmd.Flags().StringVarP(&opt.TiCDC.BinPath, "ticdc.binpath", "", opt.TiCDC.BinPath, "TiCDC instance binary path")
	rootCmd.Flags().StringVarP(&opt.Pump.BinPath, "pump.binpath", "", opt.Pump.BinPath, "Pump instance binary path")
	rootCmd.Flags().StringVarP(&opt.Drainer.BinPath, "drainer.binpath", "", opt.Drainer.BinPath, "Drainer instance binary path")

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
	return os.WriteFile(fname, []byte(strconv.Itoa(port)), 0644)
}

func loadPort(dir string) (port int, err error) {
	data, err := os.ReadFile(filepath.Join(dir, "port"))
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
	_ = os.WriteFile(fname, []byte(strings.Join(dsn, "\n")), 0644)
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
	start := time.Now()
	code := 0
	err := execute()
	if err != nil {
		fmt.Println(color.RedString("Error: %v", err))
		code = 1
	}

	if reportEnabled {
		f := func() {
			defer func() {
				if r := recover(); r != nil {
					if flags.DebugMode {
						log.Debugf("Recovered in telemetry report: %v", r)
					}
				}
			}()

			playgroundReport.ExitCode = int32(code)
			if optBytes, err := yaml.Marshal(opt); err == nil && len(optBytes) > 0 {
				if data, err := telemetry.ScrubYaml(
					optBytes,
					map[string]struct{}{
						"host":        {},
						"config_path": {},
						"bin_path":    {},
					}, // fields to hash
					map[string]struct{}{}, // fields to omit
					telemetry.GetSecret(),
				); err == nil {
					playgroundReport.Topology = (string(data))
				}
			}
			playgroundReport.TakeMilliseconds = uint64(time.Since(start).Milliseconds())
			tele := telemetry.NewTelemetry()
			ctx, cancel := context.WithTimeout(context.Background(), time.Second*2)
			err := tele.Report(ctx, teleReport)
			if flags.DebugMode {
				if err != nil {
					log.Infof("report failed: %v", err)
				}
				fmt.Printf("report: %s\n", teleReport.String())
				if data, err := json.Marshal(teleReport); err == nil {
					log.Debugf("report: %s\n", string(data))
				}
			}
			cancel()
		}

		f()
	}

	if code != 0 {
		os.Exit(code)
	}
}
