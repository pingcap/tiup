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

package operator

import (
	"strings"

	"github.com/pingcap/errors"
	"github.com/pingcap/tiup/pkg/cluster/executor"
	"github.com/pingcap/tiup/pkg/cluster/module"
)

// GetServiceStatus return the Acitive line of status.
/*
[tidb@ip-172-16-5-70 deploy]$ sudo systemctl status drainer-8249.service
● drainer-8249.service - drainer-8249 service
   Loaded: loaded (/etc/systemd/system/drainer-8249.service; disabled; vendor preset: disabled)
   Active: active (running) since Mon 2020-03-09 13:56:19 CST; 1 weeks 3 days ago
 Main PID: 36718 (drainer)
   CGroup: /system.slice/drainer-8249.service
           └─36718 bin/drainer --addr=172.16.5.70:8249 --pd-urls=http://172.16.5.70:2379 --data-dir=/data1/deploy/data.drainer --log-file=/data1/deploy/log/drainer.log --config=conf/drainer.toml --initial-commit-ts=408375872006389761

Mar 09 13:56:19 ip-172-16-5-70 systemd[1]: Started drainer-8249 service.
*/
func GetServiceStatus(e executor.Executor, name string) (active string, err error) {
	c := module.SystemdModuleConfig{
		Unit:   name,
		Action: "status",
	}
	systemd := module.NewSystemdModule(c)
	stdout, _, err := systemd.Execute(e)

	lines := strings.Split(string(stdout), "\n")
	if len(lines) >= 3 {
		return lines[2], nil
	}

	if err != nil {
		return
	}

	return "", errors.Errorf("unexpected output: %s", string(stdout))
}
