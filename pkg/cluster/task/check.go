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
	"context"
	"fmt"
	"path/filepath"
	"strings"
	"time"

	"github.com/pingcap/tiup/pkg/cluster/ctxt"
	operator "github.com/pingcap/tiup/pkg/cluster/operation"
	"github.com/pingcap/tiup/pkg/cluster/spec"
)

// the check types
var (
	CheckTypeSystemInfo   = "insight"
	CheckTypeSystemLimits = "limits"
	CheckTypeSystemConfig = "system"
	CheckTypePort         = "port"
	CheckTypeService      = "service"
	CheckTypePackage      = "package"
	CheckTypePartitions   = "partitions"
	CheckTypeFIO          = "fio"
	CheckTypePermission   = "permission"
	ChecktypeIsExist      = "exist"
	CheckTypeTimeZone     = "timezone"
)

// place the check utilities are stored
const (
	CheckToolsPathDir = "/tmp/tiup"
)

// CheckSys performs checks of system information
type CheckSys struct {
	host     string
	topo     *spec.Specification
	opt      *operator.CheckOptions
	check    string // check type name
	checkDir string
}

func storeResults(ctx context.Context, host string, results []*operator.CheckResult) {
	rr := []any{}
	for _, r := range results {
		rr = append(rr, r)
	}
	ctxt.GetInner(ctx).SetCheckResults(host, rr)
}

// Execute implements the Task interface
func (c *CheckSys) Execute(ctx context.Context) error {
	stdout, stderr, _ := ctxt.GetInner(ctx).GetOutputs(c.host)
	if len(stderr) > 0 && len(stdout) == 0 {
		return ErrNoOutput
	}
	sudo := true
	if c.topo.BaseTopo().GlobalOptions.SystemdMode == spec.UserMode {
		sudo = false
	}
	switch c.check {
	case CheckTypeSystemInfo:
		storeResults(ctx, c.host, operator.CheckSystemInfo(c.opt, stdout))
	case CheckTypeSystemLimits:
		storeResults(ctx, c.host, operator.CheckSysLimits(c.opt, c.topo.GlobalOptions.User, stdout))
	case CheckTypeSystemConfig:
		results := operator.CheckKernelParameters(c.opt, stdout)
		e, ok := ctxt.GetInner(ctx).GetExecutor(c.host)
		if !ok {
			return ErrNoExecutor
		}
		results = append(
			results,
			operator.CheckSELinux(ctx, e, sudo),
			operator.CheckTHP(ctx, e, sudo),
		)
		storeResults(ctx, c.host, results)
	case CheckTypePort:
		storeResults(ctx, c.host, operator.CheckListeningPort(c.opt, c.host, c.topo, stdout))
	case CheckTypeService:
		e, ok := ctxt.GetInner(ctx).GetExecutor(c.host)
		if !ok {
			return ErrNoExecutor
		}
		var results []*operator.CheckResult

		// check services
		results = append(
			results,
			operator.CheckServices(ctx, e, c.host, "irqbalance", false, spec.SystemdMode(string(c.topo.BaseTopo().GlobalOptions.SystemdMode))),
			// FIXME: set firewalld rules in deploy, and not disabling it anymore
			operator.CheckServices(ctx, e, c.host, "firewalld", true, spec.SystemdMode(string(c.topo.BaseTopo().GlobalOptions.SystemdMode))),
		)
		storeResults(ctx, c.host, results)
	case CheckTypePackage: // check if a command present, and if a package installed
		e, ok := ctxt.GetInner(ctx).GetExecutor(c.host)
		if !ok {
			return ErrNoExecutor
		}
		var results []*operator.CheckResult

		// check if numactl is installed
		stdout, stderr, err := e.Execute(ctx, "numactl --show", false)
		if err != nil || len(stderr) > 0 {
			results = append(results, &operator.CheckResult{
				Name: operator.CheckNameCommand,
				Err:  fmt.Errorf("numactl not usable, %s", strings.Trim(string(stderr), "\n")),
				Msg:  "numactl is not installed properly",
			})
		} else {
			results = append(results, &operator.CheckResult{
				Name: operator.CheckNameCommand,
				Msg:  "numactl: " + strings.Split(string(stdout), "\n")[0],
			})
		}

		// check if JRE is available for TiSpark
		results = append(results, operator.CheckJRE(ctx, e, c.host, c.topo)...)

		storeResults(ctx, c.host, results)
	case CheckTypePartitions:
		// check partition mount options for data_dir
		storeResults(ctx, c.host, operator.CheckPartitions(c.opt, c.host, c.topo, stdout))
	case CheckTypeFIO:
		if !c.opt.EnableDisk || c.checkDir == "" {
			break
		}

		rr, rw, lat, err := c.runFIO(ctx)
		if err != nil {
			return err
		}

		storeResults(ctx, c.host, operator.CheckFIOResult(rr, rw, lat))
	case CheckTypePermission:
		e, ok := ctxt.GetInner(ctx).GetExecutor(c.host)
		if !ok {
			return ErrNoExecutor
		}
		storeResults(ctx, c.host, operator.CheckDirPermission(ctx, e, c.topo.GlobalOptions.User, c.checkDir))
	case ChecktypeIsExist:
		e, ok := ctxt.GetInner(ctx).GetExecutor(c.host)
		if !ok {
			return ErrNoExecutor
		}
		// check partition mount options for data_dir
		storeResults(ctx, c.host, operator.CheckDirIsExist(ctx, e, c.checkDir))
	case CheckTypeTimeZone:
		storeResults(ctx, c.host, operator.CheckTimeZone(ctx, c.topo, c.host, stdout))
	}

	return nil
}

// Rollback implements the Task interface
func (c *CheckSys) Rollback(ctx context.Context) error {
	return ErrUnsupportedRollback
}

// String implements the fmt.Stringer interface
func (c *CheckSys) String() string {
	return fmt.Sprintf("CheckSys: host=%s type=%s", c.host, c.check)
}

// runFIO performs FIO checks
func (c *CheckSys) runFIO(ctx context.Context) (outRR []byte, outRW []byte, outLat []byte, err error) {
	e, ok := ctxt.GetInner(ctx).GetExecutor(c.host)
	if !ok {
		err = ErrNoExecutor
		return
	}

	checkDir := spec.Abs(c.topo.GlobalOptions.User, c.checkDir)
	testWd := filepath.Join(checkDir, "tiup-fio-test")
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

	outRR, stderr, err = e.Execute(ctx, cmdRR, false, time.Second*600)
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

	outRW, stderr, err = e.Execute(ctx, cmdRW, false, time.Second*600)
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

	outLat, stderr, err = e.Execute(ctx, cmdLat, false, time.Second*600)
	if err != nil {
		return
	}
	if len(stderr) > 0 {
		err = fmt.Errorf("%s", stderr)
		return
	}

	// cleanup
	_, stderr, err = e.Execute(
		ctx,
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
