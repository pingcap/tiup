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
	"fmt"
	"runtime"
	"strings"

	"github.com/magiconair/properties"
	_ "github.com/pingcap/go-ycsb/db/tikv"
	"github.com/pingcap/go-ycsb/pkg/client"
	"github.com/pingcap/go-ycsb/pkg/measurement"
	_ "github.com/pingcap/go-ycsb/pkg/workload"
	"github.com/pingcap/go-ycsb/pkg/ycsb"
	"github.com/spf13/cobra"
)

type ycsbConfig struct {
	PropertyFile string
	// Count is used as RecordCount in "prepare" stage and OperationCount in "run" stage
	// Both of them specify how much work should be done
	// Since they cannot exist together and stand for same reason, we can merged them together
	Count         uint32
	ReadAllFields bool

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

const commonWorkloadURL = "https://raw.githubusercontent.com/pingcap/go-ycsb/master/workloads/workload"

func (config ycsbConfig) toProperties() (*properties.Properties, error) {
	result := properties.NewProperties()
	var err error
	if config.PropertyFile != "" {
		// "a" to "f" are some workloads that used a lot
		// expand these shorthands to URL
		if len(config.PropertyFile) == 1 {
			for _, content := range []string{"a", "b", "c", "d", "e", "f"} {
				if config.PropertyFile == content {
					config.PropertyFile = commonWorkloadURL + content
					break
				}
			}
		}

		if strings.HasPrefix(config.PropertyFile, "http") {
			result, err = properties.LoadURL(config.PropertyFile)
			if err != nil {
				return nil, err
			}
		} else {
			result, err = properties.LoadFile(config.PropertyFile, properties.UTF8)
			if err != nil {
				return nil, err
			}
		}
	}
	// since the properties are not from the user input directly
	// we don't want expansion here
	result.DisableExpansion = true

	// these config are always included in the config file
	// should be overwritten if they are passed through the command line
	if config.ReadProportion != 0 {
		// we don't need to check errors since we disabled expansion
		_, _, _ = result.Set("readproportion", fmt.Sprint(config.ReadProportion))
	}
	if config.UpdateProportion != 0 {
		_, _, _ = result.Set("updateproportion", fmt.Sprint(config.UpdateProportion))
	}
	if config.ScanProportion != 0 {
		_, _, _ = result.Set("scanproportion", fmt.Sprint(config.ScanProportion))
	}
	if config.InsertProportion != 0 {
		_, _, _ = result.Set("insertproportion", fmt.Sprint(config.InsertProportion))
	}
	if config.ReadModifyWriteProportion != 0 {
		_, _, _ = result.Set("readmodifywriteproportion", fmt.Sprint(config.ReadModifyWriteProportion))
	}
	if config.RequestDistribution != "" {
		_, _, _ = result.Set("requestdistribution", config.RequestDistribution)
	}
	if config.Count != 0 {
		_, _, _ = result.Set("operationcount", fmt.Sprint(config.Count))
		_, _, _ = result.Set("recordcount", fmt.Sprint(config.Count))
	}
	_, _, _ = result.Set("verbose", fmt.Sprint(config.Verbose))
	_, _, _ = result.Set("readallfields", fmt.Sprint(config.ReadAllFields))

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

	return result, nil
}

var config ycsbConfig

func registerYcsb(root *cobra.Command) {
	cmd := &cobra.Command{
		Use: "ycsb",
	}

	cmd.PersistentFlags().StringVarP(&config.PropertyFile, "propertyfile", "f", "", "Specify a property file, can be a url, a path or one of [a, b, c, d, e, f]")
	cmd.PersistentFlags().Uint32VarP(&config.Count, "count", "c", 0, "The number of operations/records to use")
	cmd.PersistentFlags().Uint32Var(&config.BatchSize, "batchsize", 128, "Batch Size")
	cmd.PersistentFlags().Uint32Var(&config.ConnCount, "conncount", 128, "Connection Count")
	cmd.PersistentFlags().BoolVar(&config.ReadAllFields, "readallfields", true, "Whether Read All Fields")

	cmd.PersistentFlags().Float32Var(&config.ReadProportion, "readproportion", 0.95, "What proportion of operations are reads, default 0.95")
	cmd.PersistentFlags().Float32Var(&config.UpdateProportion, "updateproportion", 0.05, "What proportion of operations are updates, default 0.05")
	cmd.PersistentFlags().Float32Var(&config.ScanProportion, "scanproportion", 0, "What proportion of operations are scans, default 0")
	cmd.PersistentFlags().Float32Var(&config.InsertProportion, "insertproportion", 0, "What proportion of operations are inserts, default 0")
	cmd.PersistentFlags().Float32Var(&config.ReadModifyWriteProportion, "readmodifywriteproportion", 0, "What proportion of operations are read-modify-write, default 0")

	cmd.PersistentFlags().StringVar(&config.RequestDistribution, "requestdistribution", "uniform", "The distribution of requests across the keyspace, [zipfian, uniform, latest]")

	cmd.PersistentFlags().BoolVar(&config.Verbose, "verbose", false, "Verbose mode")
	cmd.PersistentFlags().StringVar(&config.Pd, "pd", "127.0.0.1:2379", "PD address")

	var cmdPrepare = &cobra.Command{
		Use:   "prepare",
		Short: "Prepare data for ycsb",
		RunE: func(cmd *cobra.Command, _ []string) error {
			return executeYcsb("prepare")
		},
	}

	var cmdRun = &cobra.Command{
		Use:   "run",
		Short: "Run ycsb",
		RunE: func(cmd *cobra.Command, _ []string) error {
			return executeYcsb("run")
		},
	}

	cmd.AddCommand(cmdPrepare, cmdRun)
	root.AddCommand(cmd)
}

func executeYcsb(action string) error {
	runtime.GOMAXPROCS(maxProcs)
	configProp, err := config.toProperties()
	if err != nil {
		return err
	}
	switch action {
	case "prepare":
		// note the expansion is already disabled in toProperties
		// so no need to check the error
		_, _, _ = configProp.Set("dotransactions", "false")
	case "run":
		_, _, _ = configProp.Set("dotransactions", "true")
	}
	workloadCreator := ycsb.GetWorkloadCreator("core")
	measurement.InitMeasure(configProp)
	workload, err := workloadCreator.Create(configProp)
	if err != nil {
		return err
	}
	dbCreator := ycsb.GetDBCreator("tikv")
	db, err := dbCreator.Create(configProp)
	if err != nil {
		return err
	}
	c := client.NewClient(configProp, workload, client.DbWrapper{DB: db})
	ctx := context.Background()
	c.Run(ctx)
	measurement.Output()
	return nil
}
