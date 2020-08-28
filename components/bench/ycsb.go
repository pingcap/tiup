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
	RecordCount    uint32
	OperationCount uint32
	ReadAllFields  bool

	ReadProportion            float32
	UpdateProportion          float32
	ScanProportion            float32
	InsertProportion          float32
	ReadModifyWriteProportion float32

	RequestDistribution string
	Verbose             bool
	Pd                  string
	ConnCount           uint32
	BatchSize           uint32
}

func (config ycsbConfig) ToProperties() *properties.Properties {
	result := properties.NewProperties()
	_, _, _ = result.Set("operationcount", fmt.Sprint(config.OperationCount))
	_, _, _ = result.Set("recordcount", fmt.Sprint(config.RecordCount))
	_, _, _ = result.Set("verbose", fmt.Sprint(config.Verbose))
	_, _, _ = result.Set("readallfields", fmt.Sprint(config.ReadAllFields))
	_, _, _ = result.Set("readproportion", fmt.Sprint(config.ReadProportion))
	_, _, _ = result.Set("updateproportion", fmt.Sprint(config.UpdateProportion))
	_, _, _ = result.Set("scanproportion", fmt.Sprint(config.ScanProportion))
	_, _, _ = result.Set("insertproportion", fmt.Sprint(config.InsertProportion))
	_, _, _ = result.Set("readmodifywriteproportion", fmt.Sprint(config.ReadModifyWriteProportion))
	_, _, _ = result.Set("requestdistribution", config.RequestDistribution)

	// tikv specific settings
	_, _, _ = result.Set("tikv.pd", fmt.Sprint(config.Pd))
	_, _, _ = result.Set("tikv.conncount", fmt.Sprint(config.ConnCount))
	_, _, _ = result.Set("tikv.batchsize", fmt.Sprint(config.BatchSize))

	// merge "global" config into this
	_, _, _ = result.Set("threadcount", fmt.Sprint(threads))
	_, _, _ = result.Set("dropdata", fmt.Sprint(dropData))

	// default values
	_, _, _ = result.Set("workload", "core")
	_, _, _ = result.Set("tikv.type", "raw") // todo: try to support txn

	return result
}

var config ycsbConfig

func registerYcsb(root *cobra.Command) {
	cmd := &cobra.Command{
		Use: "ycsb",
	}

	cmd.PersistentFlags().Uint32VarP(&config.OperationCount, "operationcount", "ops", 100000, "The number of operations to use during the run phase")
	cmd.PersistentFlags().Uint32VarP(&config.RecordCount, "recordcount", "records", 100000, "The number of records to be read/write")

	cmd.PersistentFlags().Uint32Var(&config.BatchSize, "batchsize", 128, "")
	cmd.PersistentFlags().Uint32Var(&config.ConnCount, "conncount", 128, "")
	cmd.PersistentFlags().BoolVar(&config.ReadAllFields, "readallfields", true, "")

	cmd.PersistentFlags().Float32Var(&config.ReadProportion, "readproportion", 0.95, "What proportion of operations are reads")
	cmd.PersistentFlags().Float32Var(&config.UpdateProportion, "updateproportion", 0.05, "What proportion of operations are updates")
	cmd.PersistentFlags().Float32Var(&config.ScanProportion, "scanproportion", 0, "What proportion of operations are scans")
	cmd.PersistentFlags().Float32Var(&config.InsertProportion, "insertproportion", 0, "What proportion of operations are inserts")
	cmd.PersistentFlags().Float32Var(&config.ReadModifyWriteProportion, "readmodifywriteproportion", 0, "What proportion of operations are read-modify-write")

	cmd.PersistentFlags().StringVarP(&config.RequestDistribution, "requestdistribution", "dist", "zipfian", "The distribution of requests across the keyspace, [zipfian, uniform, latest]")

	cmd.PersistentFlags().BoolVar(&config.Verbose, "verbose", false, "Verbose mode")
	cmd.PersistentFlags().StringVarP(&config.Pd, "tikv.pd", "pd", "127.0.0.1:2379", "PD address")

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
	workloadCreator := ycsb.GetWorkloadCreator("core")
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
	c := client.NewClient(configProp, workload, client.DbWrapper{DB: db})
	ctx := context.Background()
	c.Run(ctx)
	measurement.Output()
}
