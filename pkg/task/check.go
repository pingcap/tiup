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

package task

import (
	"fmt"
	"path/filepath"
	"strings"
	"time"

	"github.com/pingcap-incubator/tiup-cluster/pkg/clusterutil"
	"github.com/pingcap-incubator/tiup-cluster/pkg/meta"
	"github.com/pingcap-incubator/tiup-cluster/pkg/operation"
)

// the check types
var (
	CheckTypeSystemInfo   = "insight"
	CheckTypeSystemLimits = "limits"
	CheckTypeSystemConfig = "system"
	CheckTypeService      = "service"
	CheckTypePackage      = "package"
	CheckTypePartitions   = "partitions"
	CheckTypeFIO          = "fio"
)

// place the check utilities are stored
const (
	CheckToolsPathDir = "/tmp/tiup"
)

// CheckSys performs checks of system information
type CheckSys struct {
	host    string
	topo    *meta.TopologySpecification
	opt     *operator.CheckOptions
	check   string // check type name
	dataDir string
}

// Execute implements the Task interface
func (c *CheckSys) Execute(ctx *Context) error {
	stdout, stderr, _ := ctx.GetOutputs(c.host)
	if len(stderr) > 0 && len(stdout) == 0 {
		return fmt.Errorf("error getting output of %s: %s", c.host, stderr)
	}

	switch c.check {
	case CheckTypeSystemInfo:
		ctx.SetCheckResults(c.host, operator.CheckSystemInfo(c.opt, stdout))
	case CheckTypeSystemLimits:
		ctx.SetCheckResults(c.host, operator.CheckSysLimits(c.opt, c.topo.GlobalOptions.User, stdout))
	case CheckTypeSystemConfig:
		results := operator.CheckKernelParameters(c.opt, stdout)
		e, ok := ctx.GetExecutor(c.host)
		if !ok {
			return ErrNoExecutor
		}
		results = append(
			results,
			operator.CheckSELinux(e),
		)
		ctx.SetCheckResults(c.host, results)
	case CheckTypeService:
		e, ok := ctx.GetExecutor(c.host)
		if !ok {
			return ErrNoExecutor
		}
		var results []*operator.CheckResult

		// check services
		results = append(
			results,
			operator.CheckServices(e, c.host, "irqbalance", false),
			// FIXME: set firewalld rules in deploy, and not disabling it anymore
			operator.CheckServices(e, c.host, "firewalld", true),
		)
		ctx.SetCheckResults(c.host, results)
	case CheckTypePackage: // check if a command present, and if a package installed
		e, ok := ctx.GetExecutor(c.host)
		if !ok {
			return ErrNoExecutor
		}
		var results []*operator.CheckResult

		// check if numactl is installed
		stdout, stderr, err := e.Execute("numactl --show", false)
		if err != nil || len(stderr) > 0 {
			results = append(results, &operator.CheckResult{
				Name: operator.CheckNameCommand,
				Err:  fmt.Errorf("numactl not usable, %s", strings.Trim(string(stderr), "\n")),
				Msg:  "numactl is not installed properly",
			})
		} else {
			results = append(results, &operator.CheckResult{
				Name: operator.CheckNameCommand,
				Msg:  strings.Split(string(stdout), "\n")[0],
			})
		}
		ctx.SetCheckResults(c.host, results)
	case CheckTypePartitions:
		// check partition mount options for data_dir
		ctx.SetCheckResults(c.host, operator.CheckPartitions(c.opt, c.host, c.topo, stdout))
	case CheckTypeFIO:
		if !c.opt.EnableDisk || c.dataDir == "" {
			break
		}

		rr, rw, lat, err := c.runFIO(ctx)
		if err != nil {
			return err
		}

		ctx.SetCheckResults(c.host, operator.CheckFIOResult(rr, rw, lat))
	}

	return nil
}

// Rollback implements the Task interface
func (c *CheckSys) Rollback(ctx *Context) error {
	return ErrUnsupportedRollback
}

// String implements the fmt.Stringer interface
func (c *CheckSys) String() string {
	return fmt.Sprintf("CheckSys: host=%s type=%s", c.host, c.check)
}

// runFIO performs FIO checks
func (c *CheckSys) runFIO(ctx *Context) (outRR []byte, outRW []byte, outLat []byte, err error) {
	e, ok := ctx.GetExecutor(c.host)
	if !ok {
		err = ErrNoExecutor
		return
	}

	dataDir := clusterutil.Abs(c.topo.GlobalOptions.User, c.dataDir)
	testWd := filepath.Join(dataDir, "tiup-fio-test")
	fioBin := filepath.Join(CheckToolsPathDir, "bin", "fio")

	var stderr []byte

	// rand read
	var (
		fileRR = "fio_randread_test.txt"
		resRR  = "fio_randread_result.json"
	)
	cmdRR := strings.Join([]string{
		fmt.Sprintf("mkdir -p %s && cd %s", testWd, testWd),
		fmt.Sprintf("rm -f %s %s", fileRR, resRR), // cleanup any legancy files
		strings.Join([]string{
			fioBin,
			"-ioengine=psync",
			"-bs=32k",
			"-fdatasync=1",
			"-thread",
			"-rw=randread",
			"-name='fio randread test'",
			"-iodepth=4",
			"-runtime=60",
			"-numjobs=4",
			fmt.Sprintf("-filename=%s", fileRR),
			"-size=1G",
			"-group_reporting",
			"--output-format=json",
			fmt.Sprintf("--output=%s", resRR),
			"> /dev/null", // ignore output
		}, " "),
		fmt.Sprintf("cat %s", resRR),
	}, " && ")

	outRR, stderr, err = e.Execute(cmdRR, false, time.Second*600)
	if err != nil {
		return
	}
	if len(stderr) > 0 {
		err = fmt.Errorf("%s", stderr)
		return
	}

	// rand read write
	var (
		fileRW = "fio_randread_write_test.txt"
		resRW  = "fio_randread_write_test.json"
	)
	cmdRW := strings.Join([]string{
		fmt.Sprintf("mkdir -p %s && cd %s", testWd, testWd),
		fmt.Sprintf("rm -f %s %s", fileRW, resRW), // cleanup any legancy files
		strings.Join([]string{
			fioBin,
			"-ioengine=psync",
			"-bs=32k",
			"-fdatasync=1",
			"-thread",
			"-rw=randrw",
			"-percentage_random=100,0",
			"-name='fio mixed randread and sequential write test'",
			"-iodepth=4",
			"-runtime=60",
			"-numjobs=4",
			fmt.Sprintf("-filename=%s", fileRW),
			"-size=1G",
			"-group_reporting",
			"--output-format=json",
			fmt.Sprintf("--output=%s", resRW),
			"> /dev/null", // ignore output
		}, " "),
		fmt.Sprintf("cat %s", resRW),
	}, " && ")

	outRW, stderr, err = e.Execute(cmdRW, false, time.Second*600)
	if err != nil {
		return
	}
	if len(stderr) > 0 {
		err = fmt.Errorf("%s", stderr)
		return
	}

	// rand read write
	var (
		fileLat = "fio_randread_write_latency_test.txt"
		resLat  = "fio_randread_write_latency_test.json"
	)
	cmdLat := strings.Join([]string{
		fmt.Sprintf("mkdir -p %s && cd %s", testWd, testWd),
		fmt.Sprintf("rm -f %s %s", fileLat, resLat), // cleanup any legancy files
		strings.Join([]string{
			fioBin,
			"-ioengine=psync",
			"-bs=32k",
			"-fdatasync=1",
			"-thread",
			"-rw=randrw",
			"-percentage_random=100,0",
			"-name='fio mixed randread and sequential write test'",
			"-iodepth=1",
			"-runtime=60",
			"-numjobs=1",
			fmt.Sprintf("-filename=%s", fileLat),
			"-size=1G",
			"-group_reporting",
			"--output-format=json",
			fmt.Sprintf("--output=%s", resLat),
			"> /dev/null", // ignore output
		}, " "),
		fmt.Sprintf("cat %s", resLat),
	}, " && ")

	outLat, stderr, err = e.Execute(cmdLat, false, time.Second*600)
	if err != nil {
		return
	}
	if len(stderr) > 0 {
		err = fmt.Errorf("%s", stderr)
		return
	}

	// cleanup
	_, stderr, err = e.Execute(
		fmt.Sprintf("rm -rf %s", testWd),
		false,
	)
	if err != nil {
		return
	}
	if len(stderr) > 0 {
		err = fmt.Errorf("%s", stderr)
	}

	return
}
