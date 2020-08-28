package main

import (
	"context"
	"fmt"
	"github.com/magiconair/properties"
	_ "github.com/pingcap/go-ycsb/db/tikv"
	"github.com/pingcap/go-ycsb/pkg/client"
	"github.com/pingcap/go-ycsb/pkg/measurement"
	_ "github.com/pingcap/go-ycsb/pkg/workload"
	"github.com/pingcap/go-ycsb/pkg/ycsb"
	"github.com/spf13/cobra"
	"runtime"
)

type ycsbConfig struct {
	OperationCount uint32
	RecordCount    uint32
	Workload       string
	ReadAllFields  bool
	// proportion for each kind of workload
	ReadProportion            float32
	UpdateProportion          float32
	ScanProportion            float32
	InsertProportion          float32
	ReadModifyWriteProportion float32

	RequestDistribution string
	Verbose             bool
	Pd                  string
}

func (config ycsbConfig) ToProperties() *properties.Properties {
	result := properties.NewProperties()
	_, _, _ = result.Set("operationcount", fmt.Sprint(config.OperationCount))
	_, _, _ = result.Set("recordcount", fmt.Sprint(config.RecordCount))
	_, _, _ = result.Set("workload", config.Workload)
	_, _, _ = result.Set("readallfields", fmt.Sprint(config.ReadAllFields))
	_, _, _ = result.Set("readproportion", fmt.Sprint(config.ReadProportion))
	_, _, _ = result.Set("updateproportion", fmt.Sprint(config.UpdateProportion))
	_, _, _ = result.Set("scanproportion", fmt.Sprint(config.ScanProportion))
	_, _, _ = result.Set("insertproportion", fmt.Sprint(config.InsertProportion))
	_, _, _ = result.Set("readmodifywriteproportion", fmt.Sprint(config.ReadModifyWriteProportion))
	_, _, _ = result.Set("requestdistribution", config.RequestDistribution)
	_, _, _ = result.Set("verbose", fmt.Sprint(config.Verbose))
	_, _, _ = result.Set("tikv.pd", fmt.Sprint(config.Pd))
	_, _, _ = result.Set("tikv.type", "raw")
	_, _, _ = result.Set("tikv.conncount", "128")
	_, _, _ = result.Set("tikv.batchsize", "128")
	_, _, _ = result.Set("threadcount", fmt.Sprint(threads))
	_, _, _ = result.Set("dropdata", fmt.Sprint(dropData))
	return result
}

var config ycsbConfig

func registerYcsb(root *cobra.Command) {
	cmd := &cobra.Command{
		Use: "ycsb",
	}

	cmd.PersistentFlags().Uint32Var(&config.OperationCount, "operationcount", 100000, "")
	cmd.PersistentFlags().Uint32Var(&config.RecordCount, "recordcount", 100000, "")
	cmd.PersistentFlags().StringVar(&config.Workload, "workload", "core", "")
	cmd.PersistentFlags().BoolVar(&config.ReadAllFields, "readallfields", true, "")

	cmd.PersistentFlags().Float32Var(&config.ReadProportion, "readproportion", 1, "")
	cmd.PersistentFlags().Float32Var(&config.UpdateProportion, "updateproportion", 0, "")
	cmd.PersistentFlags().Float32Var(&config.ScanProportion, "scanproportion", 0, "")
	cmd.PersistentFlags().Float32Var(&config.InsertProportion, "insertproportion", 0, "")
	cmd.PersistentFlags().Float32Var(&config.ReadModifyWriteProportion, "readmodifywriteproportion", 0, "")
	cmd.PersistentFlags().StringVar(&config.Pd, "debug.pprof", pprofAddr, "")

	cmd.PersistentFlags().StringVar(&config.RequestDistribution, "requestdistribution", "uniform", "")
	cmd.PersistentFlags().BoolVar(&config.Verbose, "verbose", false, "")
	cmd.PersistentFlags().StringVar(&config.Pd, "tikv.pd", "127.0.0.1:2379", "")

	var cmdPrepare = &cobra.Command{
		Use:   "prepare",
		Short: "Prepare data for ycsb",
		Run: func(cmd *cobra.Command, _ []string) {
			executeYcsb("prepare")
		},
	}

	var cmdRun = &cobra.Command{
		Use:   "run",
		Short: "Run ycsb",
		Run: func(cmd *cobra.Command, _ []string) {
			executeYcsb("run")
		},
	}

	cmd.AddCommand(cmdPrepare, cmdRun)
	root.AddCommand(cmd)
}

func executeYcsb(action string) {
	runtime.GOMAXPROCS(maxProcs)
	configProp := config.ToProperties()
	switch action {
	case "prepare":
		_, _, _ = configProp.Set("dotransactions", "false")
	case "run":
		_, _, _ = configProp.Set("dotransactions", "true")
	}
	workloadCreator := ycsb.GetWorkloadCreator(config.Workload)
	measurement.InitMeasure(configProp)
	workload, err := workloadCreator.Create(configProp)
	if err != nil {
		panic(err)
	}
	dbCreator := ycsb.GetDBCreator("tikv")
	db, err := dbCreator.Create(configProp)
	if err != nil {
		panic(err)
	}
	c := client.NewClient(configProp, workload, client.DbWrapper{db})
	ctx := context.Background()
	c.Run(ctx)
	measurement.Output()
}
