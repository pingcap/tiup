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

package instance

import (
	"fmt"
	"io"
	"strings"

	"github.com/pingcap/tiup/pkg/tidbver"
	"github.com/pingcap/tiup/pkg/utils"
)

const tiflashDaemonConfigOld = `

[application]
runAsDaemon = true

`

const tiflashMarkCacheSizeOld = `mark_cache_size = 5368709120`

const tiflashConfigOld = `
default_profile = "default"
display_name = "TiFlash"
http_port = %[2]d
listen_host = "0.0.0.0"
tcp_port = %[3]d
path = "%[5]s"
tmp_path = "%[6]s"
%[14]s
%[13]s
[flash]
service_addr = "%[10]s:%[8]d"
tidb_status_addr = "%[11]s"
[flash.flash_cluster]
cluster_manager_path = "%[12]s"
log = "%[7]s/tiflash_cluster_manager.log"
master_ttl = 60
refresh_interval = 20
update_rule_interval = 5
[flash.proxy]
config = "%[4]s/tiflash-learner.toml"

[logger]
count = 20
errorlog = "%[7]s/tiflash_error.log"
level = "debug"
log = "%[7]s/tiflash.log"
size = "1000M"

[profiles]
[profiles.default]
load_balancing = "random"
max_memory_usage = 0
use_uncompressed_cache = 0
[profiles.readonly]
readonly = 1

[quotas]
[quotas.default]
[quotas.default.interval]
duration = 3600
errors = 0
execution_time = 0
queries = 0
read_rows = 0
result_rows = 0

[raft]
pd_addr = "%[1]s"

[status]
metrics_port = %[9]d

[users]
[users.default]
password = ""
profile = "default"
quota = "default"
[users.default.networks]
ip = "::/0"
[users.readonly]
password = ""
profile = "readonly"
quota = "default"
[users.readonly.networks]
ip = "::/0"
`

// writeTiFlashConfigOld is for < 7.1.0. Not maintained any more. Do not introduce new features.
func writeTiFlashConfigOld(w io.Writer, version utils.Version, tcpPort, httpPort, servicePort, metricsPort int, host, deployDir, clusterManagerPath string, tidbStatusAddrs, endpoints []string) error {
	pdAddrs := strings.Join(endpoints, ",")
	dataDir := fmt.Sprintf("%s/data", deployDir)
	tmpDir := fmt.Sprintf("%s/tmp", deployDir)
	logDir := fmt.Sprintf("%s/log", deployDir)
	ip := AdvertiseHost(host)
	var conf string

	if tidbver.TiFlashNotNeedSomeConfig(version.String()) {
		conf = fmt.Sprintf(tiflashConfigOld, pdAddrs, httpPort, tcpPort,
			deployDir, dataDir, tmpDir, logDir, servicePort, metricsPort,
			ip, strings.Join(tidbStatusAddrs, ","), clusterManagerPath, "", "")
	} else {
		conf = fmt.Sprintf(tiflashConfigOld, pdAddrs, httpPort, tcpPort,
			deployDir, dataDir, tmpDir, logDir, servicePort, metricsPort,
			ip, strings.Join(tidbStatusAddrs, ","), clusterManagerPath, tiflashDaemonConfigOld, tiflashMarkCacheSizeOld)
	}
	_, err := w.Write([]byte(conf))
	return err
}

func getTiFlashProxyConfigPathOld(cfg map[string]any) string {
	defer func() {
		if r := recover(); r != nil {
			return
		}
	}()
	return cfg["flash"].(map[string]any)["proxy"].(map[string]any)["config"].(string)
}

func setTiFlashProxyConfigPathOld(cfg map[string]any, path string) {
	cfg["flash"].(map[string]any)["proxy"].(map[string]any)["config"] = path
}
