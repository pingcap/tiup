package main

import (
	"context"
	"fmt"
	"net/http"
	_ "net/http/pprof"
	"runtime"
	"time"

	"github.com/pingcap/go-tpc/pkg/measurement"
	"github.com/pingcap/go-tpc/pkg/workload"
	"github.com/pingcap/go-tpc/tpcc"
	"github.com/spf13/cobra"
)

var tpccConfig tpcc.Config

func executeTpcc(action string) error {
	if pprofAddr != "" {
		go func() {
			_ = http.ListenAndServe(pprofAddr, http.DefaultServeMux)
		}()
	}
	runtime.GOMAXPROCS(maxProcs)

	if err := openDB(); err != nil {
		fmt.Println("Cannot open database, pleae check it (ip/port/username/password)")
		closeDB()
		return err
	}
	defer closeDB()

	tpccConfig.DBName = dbName
	tpccConfig.Threads = threads
	tpccConfig.Isolation = isolationLevel
	var (
		w   workload.Workloader
		err error
	)
	switch tpccConfig.OutputType {
	case "csv", "CSV":
		if tpccConfig.OutputDir == "" {
			return fmt.Errorf("Output Directory cannot be empty when generating files")
		}
		w, err = tpcc.NewCSVWorkloader(globalDB, &tpccConfig)
	default:
		w, err = tpcc.NewWorkloader(globalDB, &tpccConfig)
	}
	if err != nil {
		fmt.Println("Failed to init work loader")
		return err
	}

	timeoutCtx, cancel := context.WithTimeout(globalCtx, totalTime)
	defer cancel()

	executeWorkload(timeoutCtx, w, threads, action)

	fmt.Println("Finished")
	w.OutputStats(true)

	return nil
}

func registerTpcc(root *cobra.Command) {
	cmd := &cobra.Command{
		Use: "tpcc",
	}

	cmd.PersistentFlags().IntVar(&tpccConfig.Parts, "parts", 1, "Number to partition warehouses")
	cmd.PersistentFlags().IntVar(&tpccConfig.PartitionType, "partition-type", 1, "Partition type (1 - HASH, 2 - RANGE, 3 - LIST (like HASH), 4 - LIST (like RANGE)")
	cmd.PersistentFlags().IntVar(&tpccConfig.Warehouses, "warehouses", 10, "Number of warehouses")
	cmd.PersistentFlags().BoolVar(&tpccConfig.CheckAll, "check-all", false, "Run all consistency checks")
	var cmdPrepare = &cobra.Command{
		Use:   "prepare",
		Short: "Prepare data for TPCC",
		RunE: func(cmd *cobra.Command, _ []string) error {
			return executeTpcc("prepare")
		},
	}
	cmdPrepare.PersistentFlags().BoolVar(&tpccConfig.NoCheck, "no-check", false, "TPCC prepare check, default false")
	cmdPrepare.PersistentFlags().StringVar(&tpccConfig.OutputType, "output-type", "", "Output file type."+
		" If empty, then load data to db. Current only support csv")
	cmdPrepare.PersistentFlags().StringVar(&tpccConfig.OutputDir, "output-dir", "", "Output directory for generating file if specified")
	cmdPrepare.PersistentFlags().StringVar(&tpccConfig.SpecifiedTables, "tables", "", "Specified tables for "+
		"generating file, separated by ','. Valid only if output is set. If this flag is not set, generate all tables by default")
	cmdPrepare.PersistentFlags().IntVar(&tpccConfig.PrepareRetryCount, "retry-count", 50, "Retry count when errors occur")
	cmdPrepare.PersistentFlags().DurationVar(&tpccConfig.PrepareRetryInterval, "retry-interval", 10*time.Second, "The interval for each retry")

	var cmdRun = &cobra.Command{
		Use:   "run",
		Short: "Run workload",
		RunE: func(cmd *cobra.Command, _ []string) error {
			return executeTpcc("run")
		},
	}
	cmdRun.PersistentFlags().BoolVar(&tpccConfig.Wait, "wait", false, "including keying & thinking time described on TPC-C Standard Specification")
	cmdRun.PersistentFlags().DurationVar(&tpccConfig.MaxMeasureLatency, "max-measure-latency", measurement.DefaultMaxLatency, "max measure latency in millisecond")
	cmdRun.PersistentFlags().IntSliceVar(&tpccConfig.Weight, "weight", []int{45, 43, 4, 4, 4}, "Weight for NewOrder, Payment, OrderStatus, Delivery, StockLevel")

	var cmdCleanup = &cobra.Command{
		Use:   "cleanup",
		Short: "Cleanup data for the workload",
		RunE: func(cmd *cobra.Command, _ []string) error {
			return executeTpcc("cleanup")
		},
	}

	var cmdCheck = &cobra.Command{
		Use:   "check",
		Short: "Check data consistency for the workload",
		RunE: func(cmd *cobra.Command, _ []string) error {
			return executeTpcc("check")
		},
	}

	cmd.AddCommand(cmdRun, cmdPrepare, cmdCleanup, cmdCheck)

	root.AddCommand(cmd)
}
