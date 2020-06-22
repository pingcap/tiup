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

package meta

// ResourceControl is used to control the system resource
// See: https://www.freedesktop.org/software/systemd/man/systemd.resource-control.html
type ResourceControl struct {
	MemoryLimit         string `yaml:"memory_limit,omitempty"`
	CPUQuota            string `yaml:"cpu_quota,omitempty"`
	IOReadBandwidthMax  string `yaml:"io_read_bandwidth_max,omitempty"`
	IOWriteBandwidthMax string `yaml:"io_write_bandwidth_max,omitempty"`
}
