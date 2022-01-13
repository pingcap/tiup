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
	"github.com/pingcap/tiup/pkg/cluster/spec"
	"github.com/pingcap/tiup/pkg/environment"
	"github.com/pingcap/tiup/pkg/localdata"
	logprinter "github.com/pingcap/tiup/pkg/logger/printer"
	"github.com/pingcap/tiup/pkg/repository"
	"github.com/pingcap/tiup/pkg/telemetry"
	"github.com/pingcap/tiup/pkg/utils"
	"github.com/pingcap/tiup/pkg/version"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
	clientv3 "go.etcd.io/etcd/client/v3"
	"go.uber.org/zap"
	"golang.org/x/mod/semver"
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
	options          = &BootOptions{}
	tag              string
	tiupDataDir      string
	dataDir          string
	log              = logprinter.NewLogger("")
)

const (
	mode           = "mode"
	withMonitor    = "monitor"
	withoutMonitor = "without-monitor"

	// instance numbers
	db      = "db"
	kv      = "kv"
	pd      = "pd"
	tiflash = "tiflash"
	ticdc   = "ticdc"
	pump    = "pump"
	drainer = "drainer"

	// up timeouts
	dbTimeout      = "db.timeout"
	tiflashTimeout = "tiflash.timeout"

	// hosts
	clusterHost = "host"
	dbHost      = "db.host"
	dbPort      = "db.port"
	pdHost      = "pd.host"

	// config paths
	dbConfig      = "db.config"
	kvConfig      = "kv.config"
	pdConfig      = "pd.config"
	tiflashConfig = "tiflash.config"
	ticdcConfig   = "ticdc.config"
	pumpConfig    = "pump.config"
	drainerConfig = "drainer.config"

	// binary path
	dbBinpath      = "db.binpath"
	kvBinpath      = "kv.binpath"
	pdBinpath      = "pd.binpath"
	tiflashBinpath = "tiflash.binpath"
	ticdcBinpath   = "ticdc.binpath"
	pumpBinpath    = "pump.binpath"
	drainerBinpath = "drainer.binpath"
)

func installIfMissing(component, version string) error {
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
  $ tiup playground nightly --without-monitor       # Start a local cluster and disable monitor system
  $ tiup playground --pd.config ~/config/pd.toml    # Start a local cluster with specified configuration file
  $ tiup playground --db.binpath /xx/tidb-server    # Start a local cluster with component binary path
  $ tiup playground --mode tikv-slim                # Start a local tikv only cluster (No TiDB or TiFlash Available)
  $ tiup playground --mode tikv-slim --kv 3 --pd 3  # Start a local tikv only cluster with 6 nodes`,
		SilenceUsage:  true,
		SilenceErrors: true,
		Version:       version.NewTiUPVersion().String(),
		Args: func(cmd *cobra.Command, args []string) error {
			return nil
		},
		PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
			tiupDataDir = os.Getenv(localdata.EnvNameInstanceDataDir)
			tiupHome := os.Getenv(localdata.EnvNameHome)
			if tiupHome == "" {
				tiupHome, _ = getAbsolutePath(filepath.Join("~", localdata.ProfileDirName))
			}
			if tiupDataDir == "" {
				if tag == "" {
					dataDir = filepath.Join(tiupHome, localdata.DataParentDir, utils.Base62Tag())
				} else {
					dataDir = filepath.Join(tiupHome, localdata.DataParentDir, tag)
				}
				if dataDir == "" {
					return errors.Errorf("cannot read environment variable %s nor %s", localdata.EnvNameInstanceDataDir, localdata.EnvNameHome)
				}
				err := os.MkdirAll(dataDir, os.ModePerm)
				if err != nil {
					return err
				}
			} else {
				dataDir = tiupDataDir
			}
			instanceName := dataDir[strings.LastIndex(dataDir, "/")+1:]
			fmt.Printf("\033]0;TiUP Playground: %s\a", instanceName)
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
				options.Version = args[0]
			}

			if err := populateOpt(cmd.Flags()); err != nil {
				return err
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
			ctx = context.WithValue(ctx, logprinter.ContextKeyLogger, log)
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
						removeData()
						os.Exit(0)
					})
					return
				}

				go p.terminate(sig)
				// If user try double ctrl+c, force quit
				sig = (<-sc).(syscall.Signal)
				atomic.StoreInt32(&p.curSig, int32(syscall.SIGKILL))
				if sig == syscall.SIGINT {
					removeData()
					p.terminate(syscall.SIGKILL)
				}
			}()

			// expand version string
			if !semver.IsValid(options.Version) {
				version, err := env.V1Repository().ResolveComponentVersion(spec.ComponentTiDB, options.Version)
				if err != nil {
					return errors.Annotate(err, fmt.Sprintf("can not expand version %s to a valid semver string", options.Version))
				}
				// for nightly, may not use the same version for cluster
				if options.Version == "nightly" {
					version = "nightly"
				}
				fmt.Println(color.YellowString(`Using the version %s for version constraint "%s".
		
If you'd like to use a TiDB version other than %s, cancel and retry with the following arguments:
	Specify version manually:   tiup playground <version>
	Specify version range:      tiup playground ^5
	The nightly version:        tiup playground nightly
`, version, options.Version, version))

				options.Version = version.String()
			}

			bootErr := p.bootCluster(ctx, env, options)
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

	defaultMode := "tidb"
	defaultOptions := &BootOptions{}

	rootCmd.Flags().String(mode, defaultMode, "TiUP playground mode: 'tidb', 'tikv-slim'")
	rootCmd.Flags().StringVarP(&tag, "tag", "T", "", "Specify a tag for playground")
	rootCmd.Flags().Bool(withoutMonitor, false, "Don't start prometheus and grafana component")
	rootCmd.Flags().Bool(withMonitor, true, "Start prometheus and grafana component")
	_ = rootCmd.Flags().MarkDeprecated(withMonitor, "Please use --without-monitor to control whether to disable monitor.")

	rootCmd.Flags().Int(db, defaultOptions.TiDB.Num, "TiDB instance number")
	rootCmd.Flags().Int(kv, defaultOptions.TiKV.Num, "TiKV instance number")
	rootCmd.Flags().Int(pd, defaultOptions.PD.Num, "PD instance number")
	rootCmd.Flags().Int(tiflash, defaultOptions.TiFlash.Num, "TiFlash instance number")
	rootCmd.Flags().Int(ticdc, defaultOptions.TiCDC.Num, "TiCDC instance number")
	rootCmd.Flags().Int(pump, defaultOptions.Pump.Num, "Pump instance number")
	rootCmd.Flags().Int(drainer, defaultOptions.Drainer.Num, "Drainer instance number")

	rootCmd.Flags().Int(dbTimeout, defaultOptions.TiDB.UpTimeout, "TiDB max wait time in seconds for starting, 0 means no limit")
	rootCmd.Flags().Int(tiflashTimeout, defaultOptions.TiFlash.UpTimeout, "TiFlash max wait time in seconds for starting, 0 means no limit")

	rootCmd.Flags().String(clusterHost, defaultOptions.Host, "Playground cluster host")
	rootCmd.Flags().String(dbHost, defaultOptions.TiDB.Host, "Playground TiDB host. If not provided, TiDB will still use `host` flag as its host")
	rootCmd.Flags().Int(dbPort, defaultOptions.TiDB.Port, "Playground TiDB port. If not provided, TiDB will use 4000 as its port")
	rootCmd.Flags().String(pdHost, defaultOptions.PD.Host, "Playground PD host. If not provided, PD will still use `host` flag as its host")

	rootCmd.Flags().String(dbConfig, defaultOptions.TiDB.ConfigPath, "TiDB instance configuration file")
	rootCmd.Flags().String(kvConfig, defaultOptions.TiKV.ConfigPath, "TiKV instance configuration file")
	rootCmd.Flags().String(pdConfig, defaultOptions.PD.ConfigPath, "PD instance configuration file")
	rootCmd.Flags().String(tiflashConfig, defaultOptions.TiDB.ConfigPath, "TiFlash instance configuration file")
	rootCmd.Flags().String(pumpConfig, defaultOptions.Pump.ConfigPath, "Pump instance configuration file")
	rootCmd.Flags().String(drainerConfig, defaultOptions.Drainer.ConfigPath, "Drainer instance configuration file")
	rootCmd.Flags().String(ticdcConfig, defaultOptions.TiCDC.ConfigPath, "TiCDC instance configuration file")

	rootCmd.Flags().String(dbBinpath, defaultOptions.TiDB.BinPath, "TiDB instance binary path")
	rootCmd.Flags().String(kvBinpath, defaultOptions.TiKV.BinPath, "TiKV instance binary path")
	rootCmd.Flags().String(pdBinpath, defaultOptions.PD.BinPath, "PD instance binary path")
	rootCmd.Flags().String(tiflashBinpath, defaultOptions.TiFlash.BinPath, "TiFlash instance binary path")
	rootCmd.Flags().String(ticdcBinpath, defaultOptions.TiCDC.BinPath, "TiCDC instance binary path")
	rootCmd.Flags().String(pumpBinpath, defaultOptions.Pump.BinPath, "Pump instance binary path")
	rootCmd.Flags().String(drainerBinpath, defaultOptions.Drainer.BinPath, "Drainer instance binary path")

	rootCmd.AddCommand(newDisplay())
	rootCmd.AddCommand(newScaleOut())
	rootCmd.AddCommand(newScaleIn())

	return rootCmd.Execute()
}

func populateOpt(flagSet *pflag.FlagSet) (err error) {
	var modeVal string
	if modeVal, err = flagSet.GetString(mode); err != nil {
		return
	}

	switch modeVal {
	case "tidb":
		options.TiDB.Num = 1
		options.TiDB.UpTimeout = 60
		options.TiKV.Num = 1
		options.PD.Num = 1
		options.TiFlash.Num = 1
		options.TiFlash.UpTimeout = 120
		options.Host = "127.0.0.1"
		options.Monitor = true
	case "tikv-slim":
		options.TiKV.Num = 1
		options.PD.Num = 1
		options.Host = "127.0.0.1"
		options.Monitor = true
	default:
		err = errors.Errorf("unknown playground mode: %s", modeVal)
		return
	}

	flagSet.Visit(func(flag *pflag.Flag) {
		switch flag.Name {
		case withMonitor:
			options.Monitor, err = strconv.ParseBool(flag.Value.String())
			if err != nil {
				return
			}
		case withoutMonitor:
			options.Monitor, err = strconv.ParseBool(flag.Value.String())
			if err != nil {
				return
			}
			options.Monitor = !options.Monitor
		case db:
			options.TiDB.Num, err = strconv.Atoi(flag.Value.String())
			if err != nil {
				return
			}
		case kv:
			options.TiKV.Num, err = strconv.Atoi(flag.Value.String())
			if err != nil {
				return
			}
		case pd:
			options.PD.Num, err = strconv.Atoi(flag.Value.String())
			if err != nil {
				return
			}
		case tiflash:
			options.TiFlash.Num, err = strconv.Atoi(flag.Value.String())
			if err != nil {
				return
			}
		case ticdc:
			options.TiCDC.Num, err = strconv.Atoi(flag.Value.String())
			if err != nil {
				return
			}
		case pump:
			options.Pump.Num, err = strconv.Atoi(flag.Value.String())
			if err != nil {
				return
			}
		case drainer:
			options.Drainer.Num, err = strconv.Atoi(flag.Value.String())
			if err != nil {
				return
			}

		case dbConfig:
			options.TiDB.ConfigPath = flag.Value.String()
		case kvConfig:
			options.TiKV.ConfigPath = flag.Value.String()
		case pdConfig:
			options.PD.ConfigPath = flag.Value.String()
		case tiflashConfig:
			options.TiFlash.ConfigPath = flag.Value.String()
		case ticdcConfig:
			options.TiCDC.ConfigPath = flag.Value.String()
		case pumpConfig:
			options.Pump.ConfigPath = flag.Value.String()
		case drainerConfig:
			options.Drainer.ConfigPath = flag.Value.String()

		case dbBinpath:
			options.TiDB.BinPath = flag.Value.String()
		case kvBinpath:
			options.TiKV.BinPath = flag.Value.String()
		case pdBinpath:
			options.PD.BinPath = flag.Value.String()
		case tiflashBinpath:
			options.TiFlash.BinPath = flag.Value.String()
		case ticdcBinpath:
			options.TiCDC.BinPath = flag.Value.String()
		case pumpBinpath:
			options.Pump.BinPath = flag.Value.String()
		case drainerBinpath:
			options.Drainer.BinPath = flag.Value.String()

		case dbTimeout:
			options.TiDB.UpTimeout, err = strconv.Atoi(flag.Value.String())
			if err != nil {
				return
			}
		case tiflashTimeout:
			options.TiFlash.UpTimeout, err = strconv.Atoi(flag.Value.String())
			if err != nil {
				return
			}

		case clusterHost:
			options.Host = flag.Value.String()
		case dbHost:
			options.TiDB.Host = flag.Value.String()
		case dbPort:
			options.TiDB.Port, err = strconv.Atoi(flag.Value.String())
			if err != nil {
				return
			}
		case pdHost:
			options.PD.Host = flag.Value.String()
		}
	})

	return
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
		wd, err := os.Getwd()
		if err != nil {
			return "", err
		}
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
					if environment.DebugMode {
						log.Debugf("Recovered in telemetry report: %v", r)
					}
				}
			}()

			playgroundReport.ExitCode = int32(code)
			if optBytes, err := yaml.Marshal(options); err == nil && len(optBytes) > 0 {
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
			ctx, cancel := context.WithTimeout(context.Background(), time.Second*2)
			tele := telemetry.NewTelemetry()
			err := tele.Report(ctx, teleReport)
			if environment.DebugMode {
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

func removeData() {
	// remove if not set tag when run at standalone mode
	if tiupDataDir == "" && tag == "" {
		os.RemoveAll(dataDir)
	}
}
