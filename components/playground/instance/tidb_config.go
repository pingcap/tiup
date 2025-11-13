// Copyright 2023 PingCAP, Inc.
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
	"os"
	"path/filepath"

	"github.com/pingcap/tiup/pkg/utils"
)

func (inst *TiDBInstance) getConfig(kvwrks []*TiKVWorkerInstance) map[string]any {
	config := make(map[string]any)
	config["security.auto-tls"] = true

	switch inst.shOpt.Mode {
	case ModeCSE:
		config["keyspace-name"] = "mykeyspace"
		config["enable-safe-point-v2"] = true
		config["force-enable-vector-type"] = true
		config["use-autoscaler"] = false
		config["disaggregated-tiflash"] = true
		config["ratelimit.full-speed"] = 1048576000
		config["ratelimit.full-speed-capacity"] = 1048576000
		config["ratelimit.low-speed-watermark"] = uint64(1048576000000)
		config["ratelimit.block-write-watermark"] = uint64(1048576000000)
		config["security.enable-sem"] = false
		config["tiflash-replicas.constraints"] = []any{
			map[string]any{
				"key": "engine",
				"op":  "in",
				"values": []string{
					"tiflash",
				},
			},
			map[string]any{
				"key": "engine_role",
				"op":  "in",
				"values": []string{
					"write",
				},
			},
		}
		config["tiflash-replicas.group-id"] = "enable_s3_wn_region"
		config["tiflash-replicas.extra-s3-rule"] = false
		config["tiflash-replicas.min-count"] = 1
	case ModeDisAgg:
		config["use-autoscaler"] = false
		config["disaggregated-tiflash"] = true
	case ModeNextGen:
		config["enable-safe-point-v2"] = true
		config["split-table"] = false
		config["use-autoscaler"] = false
		config["disaggregated-tiflash"] = false
		if inst.Role() == TiDBRoleSystem {
			config["instance.tidb_service_scope"] = "dxf_service"
			config["tikv-worker-url"] = fmt.Sprintf("http://%s", utils.JoinHostPort(AdvertiseHost(kvwrks[0].Host), kvwrks[0].Port))
			config["keyspace-name"] = "SYSTEM"
		} else {
			config["keyspace-name"] = "keyspace1"
		}
	}

	tiproxyCrtPath := filepath.Join(inst.tiproxyCertDir, "tiproxy.crt")
	tiproxyKeyPath := filepath.Join(inst.tiproxyCertDir, "tiproxy.key")
	_, err1 := os.Stat(tiproxyCrtPath)
	_, err2 := os.Stat(tiproxyKeyPath)
	if err1 == nil && err2 == nil {
		config["security.session-token-signing-cert"] = tiproxyCrtPath
		config["security.session-token-signing-key"] = tiproxyKeyPath
	}

	return config
}
