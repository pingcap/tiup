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
	"encoding/json"
	"fmt"
	"net"
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
	"github.com/pingcap/tiup/pkg/tui/colorstr"
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
	Mode           string              `yaml:"mode"`
	PDMode         string              `yaml:"pd_mode"`
	Version        string              `yaml:"version"`
	PD             instance.Config     `yaml:"pd"`         // will change to api when pd_mode == ms
	TSO            instance.Config     `yaml:"tso"`        // Only available when pd_mode == ms
	Scheduling     instance.Config     `yaml:"scheduling"` // Only available when pd_mode == ms
	TiProxy        instance.Config     `yaml:"tiproxy"`
	TiDB           instance.Config     `yaml:"tidb"`
	TiKV           instance.Config     `yaml:"tikv"`
	TiFlash        instance.Config     `yaml:"tiflash"`         // ignored when mode == tidb-cse or tiflash-disagg
	TiFlashWrite   instance.Config     `yaml:"tiflash_write"`   // Only available when mode == tidb-cse or tiflash-disagg
	TiFlashCompute instance.Config     `yaml:"tiflash_compute"` // Only available when mode == tidb-cse or tiflash-disagg
	TiCDC          instance.Config     `yaml:"ticdc"`
	TiKVCDC        instance.Config     `yaml:"tikv_cdc"`
	Pump           instance.Config     `yaml:"pump"`
	Drainer        instance.Config     `yaml:"drainer"`
	Host           string              `yaml:"host"`
	Monitor        bool                `yaml:"monitor"`
	CSEOpts        instance.CSEOptions `yaml:"cse"` // Only available when mode == tidb-cse or tiflash-disagg
	GrafanaPort    int                 `yaml:"grafana_port"`
	PortOffset     int                 `yaml:"port_offset"`
	DMMaster       instance.Config     `yaml:"dm_master"`
	DMWorker       instance.Config     `yaml:"dm_worker"`
}

var (
	reportEnabled    bool // is telemetry report enabled
	teleReport       *telemetry.Report
	playgroundReport *telemetry.PlaygroundReport
	options          = &BootOptions{}
	tag              string
	deleteWhenExit   bool
	tiupDataDir      string
	dataDir          string
	log              = logprinter.NewLogger("")
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
  $ tiup playground --tag xx                           # Start a local cluster with data dir named 'xx' and uncleaned after exit
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
			switch {
			case tag != "":
				dataDir = filepath.Join(tiupHome, localdata.DataParentDir, tag)
			case tiupDataDir != "":
				dataDir = tiupDataDir
				tag = dataDir[strings.LastIndex(dataDir, "/")+1:]
			default:
				tag = utils.Base62Tag()
				dataDir = filepath.Join(tiupHome, localdata.DataParentDir, tag)
				deleteWhenExit = true
			}
			err := utils.MkdirAll(dataDir, os.ModePerm)
			if err != nil {
				return err
			}
			fmt.Printf("\033]0;TiUP Playground: %s\a", tag)
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

			if err := populateDefaultOpt(cmd.Flags()); err != nil {
				return err
			}

			port := utils.MustGetFreePort("0.0.0.0", 9527, options.PortOffset)
			err := dumpPort(filepath.Join(dataDir, "port"), port)
			p := NewPlayground(dataDir, port)
			if err != nil {
				return err
			}

			env, err := environment.InitEnv(repository.Options{}, repository.MirrorOptions{})
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
				colorstr.Printf("\n[red][bold]Playground receive signal: %s[reset]\n", sig)

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
					p.terminate(syscall.SIGKILL)
				}
			}()

			// expand version string
			if !semver.IsValid(options.Version) {
				version, err := env.V1Repository().ResolveComponentVersion(spec.ComponentTiDB, options.Version)
				if err != nil {
					return errors.Annotate(err, fmt.Sprintf("Cannot resolve version %s to a valid semver string", options.Version))
				}
				// for nightly, may not use the same version for cluster
				if options.Version == "nightly" {
					version = "nightly"
				}

				if options.Version != version.String() {
					colorstr.Fprintf(os.Stderr, `
Note: Version constraint [bold]%s[reset] is resolved to [green][bold]%s[reset]. If you'd like to use other versions:

    Use exact version:      [tiup_command]tiup playground v7.1.0[reset]
    Use version range:      [tiup_command]tiup playground ^5[reset]
    Use nightly:            [tiup_command]tiup playground nightly[reset]

`, options.Version, version.String())
				}

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

	rootCmd.Flags().StringVar(&options.Mode, "mode", "tidb", "TiUP playground mode: 'tidb', 'tidb-cse', 'tiflash-disagg', 'tikv-slim'")
	rootCmd.Flags().StringVar(&options.PDMode, "pd.mode", "pd", "PD mode: 'pd', 'ms'")
	rootCmd.PersistentFlags().StringVarP(&tag, "tag", "T", "", "Specify a tag for playground, data dir of this tag will not be removed after exit")
	rootCmd.Flags().Bool("without-monitor", false, "Don't start prometheus and grafana component")
	rootCmd.Flags().BoolVar(&options.Monitor, "monitor", true, "Start prometheus and grafana component")
	_ = rootCmd.Flags().MarkDeprecated("monitor", "Please use --without-monitor to control whether to disable monitor.")
	rootCmd.Flags().IntVar(&options.GrafanaPort, "grafana.port", 3000, "grafana port. If not provided, grafana will use 3000 as its port.")
	rootCmd.Flags().IntVar(&options.PortOffset, "port-offset", 0, "If specified, all components will use default_port+port_offset as the port. This argument is useful when you want to start multiple playgrounds on the same host. Recommend to set to 10000, 20000, etc.")

	// NOTE: Do not set default values if they may be changed in different modes.

	rootCmd.Flags().IntVar(&options.TiDB.Num, "db", 0, "TiDB instance number")
	rootCmd.Flags().IntVar(&options.TiKV.Num, "kv", 0, "TiKV instance number")
	rootCmd.Flags().IntVar(&options.PD.Num, "pd", 0, "PD instance number")
	rootCmd.Flags().IntVar(&options.TSO.Num, "tso", 0, "TSO instance number")
	rootCmd.Flags().IntVar(&options.Scheduling.Num, "scheduling", 0, "Scheduling instance number")
	rootCmd.Flags().IntVar(&options.TiProxy.Num, "tiproxy", 0, "TiProxy instance number")
	rootCmd.Flags().IntVar(&options.TiFlash.Num, "tiflash", 0, "TiFlash instance number, when --mode=tidb-cse or --mode=tiflash-disagg this will set instance number for both Write Node and Compute Node")
	rootCmd.Flags().IntVar(&options.TiFlashWrite.Num, "tiflash.write", 0, "TiFlash Write instance number, available when --mode=tidb-cse or --mode=tiflash-disagg, take precedence over --tiflash")
	rootCmd.Flags().IntVar(&options.TiFlashCompute.Num, "tiflash.compute", 0, "TiFlash Compute instance number, available when --mode=tidb-cse or --mode=tiflash-disagg, take precedence over --tiflash")
	rootCmd.Flags().IntVar(&options.TiCDC.Num, "ticdc", 0, "TiCDC instance number")
	rootCmd.Flags().IntVar(&options.TiKVCDC.Num, "kvcdc", 0, "TiKV-CDC instance number")
	rootCmd.Flags().IntVar(&options.Pump.Num, "pump", 0, "Pump instance number")
	rootCmd.Flags().IntVar(&options.Drainer.Num, "drainer", 0, "Drainer instance number")
	rootCmd.Flags().IntVar(&options.DMMaster.Num, "dm-master", 0, "DM-master instance number")
	rootCmd.Flags().IntVar(&options.DMWorker.Num, "dm-worker", 0, "DM-worker instance number")

	rootCmd.Flags().IntVar(&options.TiDB.UpTimeout, "db.timeout", 60, "TiDB max wait time in seconds for starting, 0 means no limit")
	rootCmd.Flags().IntVar(&options.TiFlash.UpTimeout, "tiflash.timeout", 120, "TiFlash max wait time in seconds for starting, 0 means no limit")
	rootCmd.Flags().IntVar(&options.TiProxy.UpTimeout, "tiproxy.timeout", 60, "TiProxy max wait time in seconds for starting, 0 means no limit")

	rootCmd.Flags().StringVar(&options.Host, "host", "127.0.0.1", "Playground cluster host")
	rootCmd.Flags().StringVar(&options.TiDB.Host, "db.host", "", "Playground TiDB host. If not provided, TiDB will still use `host` flag as its host")
	rootCmd.Flags().IntVar(&options.TiDB.Port, "db.port", 0, "Playground TiDB port. If not provided, TiDB will use 4000 as its port. Or 6000 if TiProxy is enabled.")
	rootCmd.Flags().StringVar(&options.PD.Host, "pd.host", "", "Playground PD host. If not provided, PD will still use `host` flag as its host")
	rootCmd.Flags().IntVar(&options.PD.Port, "pd.port", 0, "Playground PD port. If not provided, PD will use 2379 as its port")
	rootCmd.Flags().StringVar(&options.TiKV.Host, "kv.host", "", "Playground TiKV host. If not provided, TiKV will still use `host` flag as its host")
	rootCmd.Flags().IntVar(&options.TiKV.Port, "kv.port", 0, "Playground TiKV port. If not provided, TiKV will use 20160 as its port")
	rootCmd.Flags().StringVar(&options.TiCDC.Host, "ticdc.host", "", "Playground TiCDC host. If not provided, TiDB will still use `host` flag as its host")
	rootCmd.Flags().IntVar(&options.TiCDC.Port, "ticdc.port", 0, "Playground TiCDC port. If not provided, TiCDC will use 8300 as its port")
	rootCmd.Flags().StringVar(&options.TiProxy.Host, "tiproxy.host", "", "Playground TiProxy host. If not provided, TiProxy will still use `host` flag as its host")
	rootCmd.Flags().IntVar(&options.TiProxy.Port, "tiproxy.port", 0, "Playground TiProxy port. If not provided, TiProxy will use 6000 as its port")
	rootCmd.Flags().StringVar(&options.DMMaster.Host, "dm-master.host", "", "DM-master instance host")
	rootCmd.Flags().IntVar(&options.DMMaster.Port, "dm-master.port", 8261, "DM-master instance port")
	rootCmd.Flags().StringVar(&options.DMWorker.Host, "dm-worker.host", "", "DM-worker instance host")
	rootCmd.Flags().IntVar(&options.DMWorker.Port, "dm-worker.port", 8262, "DM-worker instance port")

	rootCmd.Flags().StringVar(&options.TiDB.ConfigPath, "db.config", "", "TiDB instance configuration file")
	rootCmd.Flags().StringVar(&options.TiKV.ConfigPath, "kv.config", "", "TiKV instance configuration file")
	rootCmd.Flags().StringVar(&options.PD.ConfigPath, "pd.config", "", "PD instance configuration file")
	rootCmd.Flags().StringVar(&options.TSO.ConfigPath, "tso.config", "", "TSO instance configuration file")
	rootCmd.Flags().StringVar(&options.Scheduling.ConfigPath, "scheduling.config", "", "Scheduling instance configuration file")
	rootCmd.Flags().StringVar(&options.TiProxy.ConfigPath, "tiproxy.config", "", "TiProxy instance configuration file")
	rootCmd.Flags().StringVar(&options.TiFlash.ConfigPath, "tiflash.config", "", "TiFlash instance configuration file, when --mode=tidb-cse or --mode=tiflash-disagg this will set config file for both Write Node and Compute Node")
	rootCmd.Flags().StringVar(&options.TiFlashWrite.ConfigPath, "tiflash.write.config", "", "TiFlash Write instance configuration file, available when --mode=tidb-cse or --mode=tiflash-disagg, take precedence over --tiflash.config")
	rootCmd.Flags().StringVar(&options.TiFlashCompute.ConfigPath, "tiflash.compute.config", "", "TiFlash Compute instance configuration file, available when --mode=tidb-cse or --mode=tiflash-disagg, take precedence over --tiflash.config")
	rootCmd.Flags().StringVar(&options.Pump.ConfigPath, "pump.config", "", "Pump instance configuration file")
	rootCmd.Flags().StringVar(&options.Drainer.ConfigPath, "drainer.config", "", "Drainer instance configuration file")
	rootCmd.Flags().StringVar(&options.TiCDC.ConfigPath, "ticdc.config", "", "TiCDC instance configuration file")
	rootCmd.Flags().StringVar(&options.TiKVCDC.ConfigPath, "kvcdc.config", "", "TiKV-CDC instance configuration file")
	rootCmd.Flags().StringVar(&options.DMMaster.ConfigPath, "dm-master.config", "", "DM-master instance configuration file")
	rootCmd.Flags().StringVar(&options.DMWorker.ConfigPath, "dm-worker.config", "", "DM-worker instance configuration file")

	rootCmd.Flags().StringVar(&options.TiDB.BinPath, "db.binpath", "", "TiDB instance binary path")
	rootCmd.Flags().StringVar(&options.TiKV.BinPath, "kv.binpath", "", "TiKV instance binary path")
	rootCmd.Flags().StringVar(&options.PD.BinPath, "pd.binpath", "", "PD instance binary path")
	rootCmd.Flags().StringVar(&options.TSO.BinPath, "tso.binpath", "", "TSO instance binary path")
	rootCmd.Flags().StringVar(&options.Scheduling.BinPath, "scheduling.binpath", "", "Scheduling instance binary path")
	rootCmd.Flags().StringVar(&options.TiProxy.BinPath, "tiproxy.binpath", "", "TiProxy instance binary path")
	rootCmd.Flags().StringVar(&options.TiProxy.Version, "tiproxy.version", "", "TiProxy instance version")
	rootCmd.Flags().StringVar(&options.TiFlash.BinPath, "tiflash.binpath", "", "TiFlash instance binary path, when --mode=tidb-cse or --mode=tiflash-disagg this will set binary path for both Write Node and Compute Node")
	rootCmd.Flags().StringVar(&options.TiFlashWrite.BinPath, "tiflash.write.binpath", "", "TiFlash Write instance binary path, available when --mode=tidb-cse or --mode=tiflash-disagg, take precedence over --tiflash.binpath")
	rootCmd.Flags().StringVar(&options.TiFlashCompute.BinPath, "tiflash.compute.binpath", "", "TiFlash Compute instance binary path, available when --mode=tidb-cse or --mode=tiflash-disagg, take precedence over --tiflash.binpath")
	rootCmd.Flags().StringVar(&options.TiCDC.BinPath, "ticdc.binpath", "", "TiCDC instance binary path")
	rootCmd.Flags().StringVar(&options.TiKVCDC.BinPath, "kvcdc.binpath", "", "TiKV-CDC instance binary path")
	rootCmd.Flags().StringVar(&options.Pump.BinPath, "pump.binpath", "", "Pump instance binary path")
	rootCmd.Flags().StringVar(&options.Drainer.BinPath, "drainer.binpath", "", "Drainer instance binary path")
	rootCmd.Flags().StringVar(&options.DMMaster.BinPath, "dm-master.binpath", "", "DM-master instance binary path")
	rootCmd.Flags().StringVar(&options.DMWorker.BinPath, "dm-worker.binpath", "", "DM-worker instance binary path")

	rootCmd.Flags().StringVar(&options.TiKVCDC.Version, "kvcdc.version", "", "TiKV-CDC instance version")

	rootCmd.Flags().StringVar(&options.CSEOpts.S3Endpoint, "cse.s3_endpoint", "http://127.0.0.1:9000", "Object store URL for --mode=tidb-cse or --mode=tiflash-disagg")
	rootCmd.Flags().StringVar(&options.CSEOpts.Bucket, "cse.bucket", "tiflash", "Object store bucket for --mode=tidb-cse or --mode=tiflash-disagg")
	rootCmd.Flags().StringVar(&options.CSEOpts.AccessKey, "cse.access_key", "minioadmin", "Object store access key for --mode=tidb-cse or --mode=tiflash-disagg")
	rootCmd.Flags().StringVar(&options.CSEOpts.SecretKey, "cse.secret_key", "minioadmin", "Object store secret key for --mode=tidb-cse or --mode=tiflash-disagg")

	rootCmd.AddCommand(newDisplay())
	rootCmd.AddCommand(newScaleOut())
	rootCmd.AddCommand(newScaleIn())

	return rootCmd.Execute()
}

func populateDefaultOpt(flagSet *pflag.FlagSet) error {
	if flagSet.Lookup("without-monitor").Changed {
		v, _ := flagSet.GetBool("without-monitor")
		options.Monitor = !v
	}

	defaultInt := func(variable *int, flagName string, defaultValue int) {
		if !flagSet.Lookup(flagName).Changed {
			*variable = defaultValue
		}
	}

	defaultStr := func(variable *string, flagName string, defaultValue string) {
		if !flagSet.Lookup(flagName).Changed {
			*variable = defaultValue
		}
	}

	switch options.Mode {
	case "tidb":
		defaultInt(&options.TiDB.Num, "db", 1)
		defaultInt(&options.TiKV.Num, "kv", 1)
		defaultInt(&options.TiFlash.Num, "tiflash", 1)
	case "tikv-slim":
		defaultInt(&options.TiKV.Num, "kv", 1)
	case "tidb-cse", "tiflash-disagg":
		defaultInt(&options.TiDB.Num, "db", 1)
		defaultInt(&options.TiKV.Num, "kv", 1)
		defaultInt(&options.TiFlash.Num, "tiflash", 1)
		defaultInt(&options.TiFlashWrite.Num, "tiflash.write", options.TiFlash.Num)
		defaultStr(&options.TiFlashWrite.BinPath, "tiflash.write.binpath", options.TiFlash.BinPath)
		defaultStr(&options.TiFlashWrite.ConfigPath, "tiflash.write.config", options.TiFlash.ConfigPath)
		options.TiFlashWrite.UpTimeout = options.TiFlash.UpTimeout
		defaultInt(&options.TiFlashCompute.Num, "tiflash.compute", options.TiFlash.Num)
		defaultStr(&options.TiFlashCompute.BinPath, "tiflash.compute.binpath", options.TiFlash.BinPath)
		defaultStr(&options.TiFlashCompute.ConfigPath, "tiflash.compute.config", options.TiFlash.ConfigPath)
		options.TiFlashCompute.UpTimeout = options.TiFlash.UpTimeout
	default:
		return errors.Errorf("Unknown --mode %s", options.Mode)
	}

	switch options.PDMode {
	case "pd":
		defaultInt(&options.PD.Num, "pd", 1)
	case "ms":
		defaultInt(&options.PD.Num, "pd", 1)
		defaultStr(&options.PD.BinPath, "pd.binpath", options.PD.BinPath)
		defaultStr(&options.PD.ConfigPath, "pd.config", options.PD.ConfigPath)
		defaultInt(&options.TSO.Num, "tso", 1)
		defaultStr(&options.TSO.BinPath, "tso.binpath", options.PD.BinPath)
		defaultStr(&options.TSO.ConfigPath, "tso.config", options.PD.ConfigPath)
		defaultInt(&options.Scheduling.Num, "scheduling", 1)
		defaultStr(&options.Scheduling.BinPath, "scheduling.binpath", options.PD.BinPath)
		defaultStr(&options.Scheduling.ConfigPath, "scheduling.config", options.PD.ConfigPath)
	default:
		return errors.Errorf("Unknown --pd.mode %s", options.PDMode)
	}

	return nil
}

func tryConnect(addr string, timeoutSec int) error {
	conn, err := net.DialTimeout("tcp", addr, time.Duration(timeoutSec)*time.Second)
	if err != nil {
		return err
	}
	defer conn.Close()
	return nil
}

// checkDB check if the addr is connectable by getting a connection from sql.DB. timeout <=0 means no timeout
func checkDB(dbAddr string, timeout int) bool {
	if timeout > 0 {
		for i := 0; i < timeout; i++ {
			if tryConnect(dbAddr, timeout) == nil {
				return true
			}
			time.Sleep(time.Second)
		}
		return false
	}
	for {
		if err := tryConnect(dbAddr, timeout); err == nil {
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

func checkDMMasterStatus(dmMasterClient *api.DMMasterClient, dmMasterAddr string, timeout int) bool {
	if timeout > 0 {
		for i := 0; i < timeout; i++ {
			if _, isActive, _, err := dmMasterClient.GetMaster(dmMasterAddr); err == nil && isActive {
				return true
			}
			time.Sleep(time.Second)
		}
		return false
	}
	for {
		if _, isActive, _, err := dmMasterClient.GetMaster(dmMasterAddr); err == nil && isActive {
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
	return utils.WriteFile(fname, []byte(strconv.Itoa(port)), 0644)
}

func loadPort(dir string) (port int, err error) {
	data, err := os.ReadFile(filepath.Join(dir, "port"))
	if err != nil {
		return 0, err
	}

	port, err = strconv.Atoi(string(data))
	return
}

func dumpDSN(fname string, dbs []*instance.TiDBInstance, tdbs []*instance.TiProxy) {
	var dsn []string
	for _, db := range dbs {
		dsn = append(dsn, fmt.Sprintf("mysql://root@%s", db.Addr()))
	}
	for _, tdb := range tdbs {
		dsn = append(dsn, fmt.Sprintf("mysql://root@%s", tdb.Addr()))
	}
	_ = utils.WriteFile(fname, []byte(strings.Join(dsn, "\n")), 0644)
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
	removeData()

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
	if deleteWhenExit {
		os.RemoveAll(dataDir)
	}
}
