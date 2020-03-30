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
	"fmt"
	"strings"

	"github.com/pingcap-incubator/tiops/pkg/executor"
	"github.com/pingcap-incubator/tiops/pkg/log"
	"github.com/pingcap-incubator/tiops/pkg/meta"
	"github.com/relex/aini"
)

var (
	systemdUnitPath = "/etc/systemd/system"
)

// parseDirs sets values of directories of component
func parseDirs(host *aini.Host, ins meta.InstanceSpec) (meta.InstanceSpec, error) {
	hostName, sshPort := ins.SSH()

	e, err := executor.NewSSHExecutor(executor.SSHConfig{
		Host:    hostName,
		Port:    sshPort,
		User:    host.Vars["ansible_user"],
		KeyFile: SSHKeyPath(), // ansible generated keyfile
	})
	if err != nil {
		return ins, err
	}
	log.Debugf("Detecting deploy paths on %s...", hostName)

	switch ins.Role() {
	case meta.ComponentTiDB:
		serviceFile := fmt.Sprintf("%s/%s-%d.service",
			systemdUnitPath,
			meta.ComponentTiDB,
			ins.GetMainPort())
		cmd := fmt.Sprintf("cat `grep 'ExecStart' %s | sed 's/ExecStart=//'`", serviceFile)
		stdout, _, err := e.Execute(cmd, false)
		if err != nil {
			return ins, nil
		}

		// parse dirs
		newIns := ins.(meta.TiDBSpec)
		for _, line := range strings.Split(string(stdout), "\n") {
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
	case meta.ComponentTiKV:
		serviceFile := fmt.Sprintf("%s/%s-%d.service",
			systemdUnitPath,
			meta.ComponentTiKV,
			ins.GetMainPort())
		cmd := fmt.Sprintf("cat `grep 'ExecStart' %s | sed 's/ExecStart=//'`", serviceFile)
		stdout, _, err := e.Execute(cmd, false)
		if err != nil {
			return ins, nil
		}

		// parse dirs
		newIns := ins.(meta.TiKVSpec)
		for _, line := range strings.Split(string(stdout), "\n") {
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
	case meta.ComponentPD:
		serviceFile := fmt.Sprintf("%s/%s-%d.service",
			systemdUnitPath,
			meta.ComponentPD,
			ins.GetMainPort())
		cmd := fmt.Sprintf("cat `grep 'ExecStart' %s | sed 's/ExecStart=//'`", serviceFile)
		stdout, _, err := e.Execute(cmd, false)
		if err != nil {
			return ins, nil
		}
		//fmt.Printf("%s\n", stdout)

		// parse dirs
		newIns := ins.(meta.PDSpec)
		for _, line := range strings.Split(string(stdout), "\n") {
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
	case meta.ComponentPump:
		serviceFile := fmt.Sprintf("%s/%s-%d.service",
			systemdUnitPath,
			meta.ComponentPump,
			ins.GetMainPort())
		cmd := fmt.Sprintf("cat `grep 'ExecStart' %s | sed 's/ExecStart=//'`", serviceFile)
		stdout, _, err := e.Execute(cmd, false)
		if err != nil {
			return ins, nil
		}

		// parse dirs
		newIns := ins.(meta.PumpSpec)
		for _, line := range strings.Split(string(stdout), "\n") {
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
	case meta.ComponentDrainer:
		serviceFile := fmt.Sprintf("%s/%s-%d.service",
			systemdUnitPath,
			meta.ComponentDrainer,
			ins.GetMainPort())
		cmd := fmt.Sprintf("cat `grep 'ExecStart' %s | sed 's/ExecStart=//'`", serviceFile)
		stdout, _, err := e.Execute(cmd, false)
		if err != nil {
			return ins, nil
		}

		// parse dirs
		newIns := ins.(meta.DrainerSpec)
		for _, line := range strings.Split(string(stdout), "\n") {
			if strings.HasPrefix(line, "DEPLOY_DIR=") {
				newIns.DeployDir = strings.TrimPrefix(line, "DEPLOY_DIR=")
				continue
			}
			if strings.Contains(line, "--log-file=") {
				fullLog := strings.Split(line, " ")[4] // 4 whitespaces ahead
				logDir := strings.TrimSuffix(strings.TrimPrefix(fullLog,
					"--log-file=\""), "/drainer.log\"")
				newIns.LogDir = logDir
				continue
			}
		}
		return newIns, nil
	case meta.ComponentPrometheus:
		serviceFile := fmt.Sprintf("%s/%s-%d.service",
			systemdUnitPath,
			meta.ComponentPrometheus,
			ins.GetMainPort())
		cmd := fmt.Sprintf("cat `grep 'ExecStart' %s | sed 's/ExecStart=//'`", serviceFile)
		stdout, _, err := e.Execute(cmd, false)
		if err != nil {
			return ins, nil
		}

		// parse dirs
		newIns := ins.(meta.PrometheusSpec)
		for _, line := range strings.Split(string(stdout), "\n") {
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
	case meta.ComponentAlertManager:
		serviceFile := fmt.Sprintf("%s/%s-%d.service",
			systemdUnitPath,
			meta.ComponentAlertManager,
			ins.GetMainPort())
		cmd := fmt.Sprintf("cat `grep 'ExecStart' %s | sed 's/ExecStart=//'`", serviceFile)
		stdout, _, err := e.Execute(cmd, false)
		if err != nil {
			return ins, nil
		}

		// parse dirs
		newIns := ins.(meta.AlertManagerSpec)
		for _, line := range strings.Split(string(stdout), "\n") {
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
	case meta.ComponentGrafana:
		serviceFile := fmt.Sprintf("%s/%s-%d.service",
			systemdUnitPath,
			meta.ComponentGrafana,
			ins.GetMainPort())
		cmd := fmt.Sprintf("cat `grep 'ExecStart' %s | sed 's/ExecStart=//'`", serviceFile)
		stdout, _, err := e.Execute(cmd, false)
		if err != nil {
			return ins, nil
		}

		// parse dirs
		newIns := ins.(meta.GrafanaSpec)
		for _, line := range strings.Split(string(stdout), "\n") {
			if strings.HasPrefix(line, "DEPLOY_DIR=") {
				newIns.DeployDir = strings.TrimPrefix(line, "DEPLOY_DIR=")
				continue
			}
		}
		return newIns, nil
	}
	return ins, nil
}
