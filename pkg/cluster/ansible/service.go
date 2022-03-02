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

package ansible

import (
	"context"
	"fmt"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/BurntSushi/toml"
	"github.com/pingcap/errors"
	"github.com/pingcap/tiup/pkg/cluster/ctxt"
	"github.com/pingcap/tiup/pkg/cluster/executor"
	"github.com/pingcap/tiup/pkg/cluster/spec"
	logprinter "github.com/pingcap/tiup/pkg/logger/printer"
)

var (
	systemdUnitPath = "/etc/systemd/system"
)

// parseDirs sets values of directories of component
func parseDirs(ctx context.Context, user string, ins spec.InstanceSpec, sshTimeout uint64, sshType executor.SSHType) (spec.InstanceSpec, error) {
	logger := ctx.Value(logprinter.ContextKeyLogger).(*logprinter.Logger)
	hostName, sshPort := ins.SSH()

	e, err := executor.New(sshType, false, executor.SSHConfig{
		Host:    hostName,
		Port:    sshPort,
		User:    user,
		KeyFile: SSHKeyPath(), // ansible generated keyfile
		Timeout: time.Second * time.Duration(sshTimeout),
	})
	if err != nil {
		return nil, err
	}
	logger.Debugf("Detecting deploy paths on %s...", hostName)

	stdout, err := readStartScript(ctx, e, ins.Role(), hostName, ins.GetMainPort())
	if len(stdout) <= 1 || err != nil {
		return ins, err
	}

	switch ins.Role() {
	case spec.ComponentTiDB:
		// parse dirs
		newIns := ins.(*spec.TiDBSpec)
		for _, line := range strings.Split(stdout, "\n") {
			if strings.HasPrefix(line, "DEPLOY_DIR=") {
				newIns.DeployDir = strings.TrimPrefix(line, "DEPLOY_DIR=")
				continue
			}
			if strings.Contains(line, "--log-file=") {
				fullLog := strings.Split(line, " ")[4] // 4 whitespaces ahead
				logDir := strings.TrimSuffix(strings.TrimPrefix(fullLog,
					"--log-file=\""), "/tidb.log\"")
				newIns.LogDir = logDir
				continue
			}
		}
		return newIns, nil
	case spec.ComponentTiKV:
		// parse dirs
		newIns := ins.(*spec.TiKVSpec)
		for _, line := range strings.Split(stdout, "\n") {
			if strings.HasPrefix(line, "cd \"") {
				newIns.DeployDir = strings.Trim(strings.Split(line, " ")[1], "\"")
				continue
			}
			if strings.Contains(line, "--data-dir") {
				dataDir := strings.Split(line, " ")[5] // 4 whitespaces ahead
				newIns.DataDir = strings.Trim(dataDir, "\"")
				continue
			}
			if strings.Contains(line, "--log-file") {
				fullLog := strings.Split(line, " ")[5] // 4 whitespaces ahead
				logDir := strings.TrimSuffix(strings.TrimPrefix(fullLog,
					"\""), "/tikv.log\"")
				newIns.LogDir = logDir
				continue
			}
		}
		return newIns, nil
	case spec.ComponentPD:
		// parse dirs
		newIns := ins.(*spec.PDSpec)
		for _, line := range strings.Split(stdout, "\n") {
			if strings.HasPrefix(line, "DEPLOY_DIR=") {
				newIns.DeployDir = strings.TrimPrefix(line, "DEPLOY_DIR=")
				continue
			}
			if strings.Contains(line, "--name") {
				nameArg := strings.Split(line, " ")[4] // 4 whitespaces ahead
				name := strings.TrimPrefix(nameArg, "--name=")
				newIns.Name = strings.Trim(name, "\"")
				continue
			}
			if strings.Contains(line, "--data-dir") {
				dataArg := strings.Split(line, " ")[4] // 4 whitespaces ahead
				dataDir := strings.TrimPrefix(dataArg, "--data-dir=")
				newIns.DataDir = strings.Trim(dataDir, "\"")
				continue
			}
			if strings.Contains(line, "--log-file=") {
				fullLog := strings.Split(line, " ")[4] // 4 whitespaces ahead
				logDir := strings.TrimSuffix(strings.TrimPrefix(fullLog,
					"--log-file=\""), "/pd.log\"")
				newIns.LogDir = logDir
				continue
			}
		}
		return newIns, nil
	case spec.ComponentTiFlash:
		// parse dirs
		newIns := ins.(*spec.TiFlashSpec)
		for _, line := range strings.Split(stdout, "\n") {
			if strings.HasPrefix(line, "cd \"") {
				newIns.DeployDir = strings.Trim(strings.Split(line, " ")[1], "\"")
				continue
			}

			// exec bin/tiflash/tiflash server --config-file conf/tiflash.toml
			if strings.Contains(line, "-config-file") {
				// parser the config file for `path` and `tmp_path`
				part := strings.Split(line, " ")
				fname := part[len(part)-1]
				fname = strings.TrimSpace(fname)
				if !filepath.IsAbs(fname) {
					fname = filepath.Join(newIns.DeployDir, fname)
				}
				err := parseTiflashConfig(ctx, e, newIns, fname)
				if err != nil {
					return nil, err
				}
			}
		}
		return newIns, nil
	case spec.ComponentPump:
		// parse dirs
		newIns := ins.(*spec.PumpSpec)
		for _, line := range strings.Split(stdout, "\n") {
			if strings.HasPrefix(line, "DEPLOY_DIR=") {
				newIns.DeployDir = strings.TrimPrefix(line, "DEPLOY_DIR=")
				continue
			}
			if strings.Contains(line, "--data-dir") {
				dataArg := strings.Split(line, " ")[4] // 4 whitespaces ahead
				dataDir := strings.TrimPrefix(dataArg, "--data-dir=")
				newIns.DataDir = strings.Trim(dataDir, "\"")
				continue
			}
			if strings.Contains(line, "--log-file=") {
				fullLog := strings.Split(line, " ")[4] // 4 whitespaces ahead
				logDir := strings.TrimSuffix(strings.TrimPrefix(fullLog,
					"--log-file=\""), "/pump.log\"")
				newIns.LogDir = logDir
				continue
			}
		}
		return newIns, nil
	case spec.ComponentDrainer:
		// parse dirs
		newIns := ins.(*spec.DrainerSpec)
		for _, line := range strings.Split(stdout, "\n") {
			if strings.HasPrefix(line, "DEPLOY_DIR=") {
				newIns.DeployDir = strings.TrimPrefix(line, "DEPLOY_DIR=")
				continue
			}
			if strings.Contains(line, "--data-dir") {
				dataArg := strings.Split(line, " ")[4] // 4 whitespaces ahead
				dataDir := strings.TrimPrefix(dataArg, "--data-dir=")
				newIns.DataDir = strings.Trim(dataDir, "\"")
				continue
			}
			if strings.Contains(line, "--log-file=") {
				fullLog := strings.Split(line, " ")[4] // 4 whitespaces ahead
				logDir := strings.TrimSuffix(strings.TrimPrefix(fullLog,
					"--log-file=\""), "/drainer.log\"")
				newIns.LogDir = logDir
				continue
			}
			if strings.Contains(line, "--initial-commit-ts=") {
				tsArg := strings.Split(line, " ")[4] // 4 whitespaces ahead
				tmpTs, _ := strconv.Atoi(strings.TrimPrefix(tsArg, "--initial-commit-ts="))
				newIns.Config = make(map[string]interface{})
				newIns.Config["initial-commit-ts"] = int64(tmpTs)
			}
		}
		return newIns, nil
	case spec.ComponentPrometheus:
		// parse dirs
		newIns := ins.(*spec.PrometheusSpec)
		for _, line := range strings.Split(stdout, "\n") {
			if strings.HasPrefix(line, "DEPLOY_DIR=") {
				newIns.DeployDir = strings.TrimPrefix(line, "DEPLOY_DIR=")
				continue
			}
			if strings.Contains(line, "exec > >(tee -i -a") {
				fullLog := strings.Split(line, " ")[5]
				logDir := strings.TrimSuffix(strings.TrimPrefix(fullLog, "\""),
					"/prometheus.log\")")
				newIns.LogDir = logDir
				continue
			}
			if strings.Contains(line, "--storage.tsdb.path=") {
				dataArg := strings.Split(line, " ")[4] // 4 whitespaces ahead
				dataDir := strings.TrimPrefix(dataArg, "--storage.tsdb.path=")
				newIns.DataDir = strings.Trim(dataDir, "\"")
				continue
			}
		}
		return newIns, nil
	case spec.ComponentAlertmanager:
		// parse dirs
		newIns := ins.(*spec.AlertmanagerSpec)
		for _, line := range strings.Split(stdout, "\n") {
			if strings.HasPrefix(line, "DEPLOY_DIR=") {
				newIns.DeployDir = strings.TrimPrefix(line, "DEPLOY_DIR=")
				continue
			}
			if strings.Contains(line, "exec > >(tee -i -a") {
				fullLog := strings.Split(line, " ")[5]
				logDir := strings.TrimSuffix(strings.TrimPrefix(fullLog, "\""),
					"/alertmanager.log\")")
				newIns.LogDir = logDir
				continue
			}
			if strings.Contains(line, "--storage.path=") {
				dataArg := strings.Split(line, " ")[4] // 4 whitespaces ahead
				dataDir := strings.TrimPrefix(dataArg, "--storage.path=")
				newIns.DataDir = strings.Trim(dataDir, "\"")
				continue
			}
		}
		return newIns, nil
	case spec.ComponentGrafana:
		// parse dirs
		newIns := ins.(*spec.GrafanaSpec)
		for _, line := range strings.Split(stdout, "\n") {
			if strings.HasPrefix(line, "DEPLOY_DIR=") {
				newIns.DeployDir = strings.TrimPrefix(line, "DEPLOY_DIR=")
				continue
			}
		}
		return newIns, nil
	}
	return ins, nil
}

func parseTiflashConfig(ctx context.Context, e ctxt.Executor, spec *spec.TiFlashSpec, fname string) error {
	data, err := readFile(ctx, e, fname)
	if err != nil {
		return err
	}

	err = parseTiflashConfigFromFileData(spec, data)
	if err != nil {
		return err
	}

	return nil
}

func parseTiflashConfigFromFileData(spec *spec.TiFlashSpec, data []byte) error {
	cfg := make(map[string]interface{})

	err := toml.Unmarshal(data, &cfg)
	if err != nil {
		return errors.AddStack(err)
	}

	if path, ok := cfg["path"]; ok {
		spec.DataDir = fmt.Sprintf("%v", path)
	}

	if tmpPath, ok := cfg["tmp_path"]; ok {
		spec.TmpDir = fmt.Sprintf("%v", tmpPath)
	}

	return nil
}

func readFile(ctx context.Context, e ctxt.Executor, fname string) (data []byte, err error) {
	cmd := fmt.Sprintf("cat %s", fname)
	stdout, stderr, err := e.Execute(ctx, cmd, false)
	if err != nil {
		return nil, errors.Annotatef(err, "stderr: %s", stderr)
	}

	return stdout, nil
}

func readStartScript(ctx context.Context, e ctxt.Executor, component, host string, port int) (string, error) {
	serviceFile := fmt.Sprintf("%s/%s-%d.service",
		systemdUnitPath,
		component,
		port)
	cmd := fmt.Sprintf("cat `grep 'ExecStart' %s | sed 's/ExecStart=//'`", serviceFile)
	stdout, stderr, err := e.Execute(ctx, cmd, false)
	if err != nil {
		return string(stdout), err
	}
	if len(stderr) > 0 {
		return string(stdout), errors.Errorf(
			"can not detect dir paths of %s %s:%d, %s",
			component,
			host,
			port,
			stderr,
		)
	}
	return string(stdout), nil
}
