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
	"context"
	"strings"
	"time"

	"github.com/pingcap/errors"
	"github.com/pingcap/tiup/pkg/cluster/ctxt"
	"github.com/pingcap/tiup/pkg/cluster/module"
	"go.uber.org/zap"
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
func GetServiceStatus(ctx context.Context, e ctxt.Executor, name string, scope string, systemdMode string) (active, memory string, since time.Duration, err error) {
	c := module.SystemdModuleConfig{
		Unit:        name,
		Action:      "status",
		Scope:       scope,
		SystemdMode: systemdMode,
	}
	systemd := module.NewSystemdModule(c)
	// ignore error since stopped service returns exit code 3
	stdout, _, _ := systemd.Execute(ctx, e)

	lines := strings.Split(string(stdout), "\n")
	for _, line := range lines {
		words := strings.Split(strings.TrimSpace(line), " ")
		if len(words) >= 2 {
			switch words[0] {
			case "Active:":
				active = words[1]
				since = parseSystemctlSince(line)
			case "Memory:":
				memory = words[1]
			}
		}
	}
	if active == "" {
		err = errors.Errorf("unexpected output: %s", string(stdout))
	}
	return
}

// `systemctl status xxx.service` returns as below
// Active: active (running) since Sat 2021-03-27 10:51:11 CST; 41min ago
func parseSystemctlSince(str string) (dur time.Duration) {
	// if service is not found or other error, don't need to parse it
	if str == "" {
		return 0
	}
	defer func() {
		if dur == 0 {
			zap.L().Warn("failed to parse systemctl since", zap.String("value", str))
		}
	}()
	parts := strings.Split(str, ";")
	if len(parts) != 2 {
		return
	}
	parts = strings.Split(parts[0], " ")
	if len(parts) < 3 {
		return
	}

	dateStr := strings.Join(parts[len(parts)-3:], " ")

	tm, err := time.Parse("2006-01-02 15:04:05 MST", dateStr)
	if err != nil {
		return
	}

	return time.Since(tm)
}
