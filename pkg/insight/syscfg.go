// Copyright 2021 PingCAP, Inc.
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

package insight

import (
	"io/ioutil"
	"strconv"
	"strings"

	sysctl "github.com/lorenzosaino/go-sysctl"
)

// SysCfg are extra system configs we collected
type SysCfg struct {
	SecLimit []SecLimitField   `json:"sec_limit,omitempty"`
	SysCtl   map[string]string `json:"sysctl,omitempty"`
}

// SecLimitField is the config field in security limit file
type SecLimitField struct {
	Domain string `json:"domain"`
	Type   string `json:"type"`
	Item   string `json:"item"`
	Value  int    `json:"value"`
}

func (c *SysCfg) getSysCfg() {
	c.SysCtl = collectSysctl()
	c.SecLimit = collectSecLimit()
}

func collectSysctl() map[string]string {
	msg, err := sysctl.GetAll()
	if err != nil {
		return nil
	}
	return msg
}

const limitFilePath = "/etc/security/limits.conf"

func collectSecLimit() []SecLimitField {
	result := make([]SecLimitField, 0)

	data, err := ioutil.ReadFile(limitFilePath)
	if err != nil {
		return result
	}

	for _, line := range strings.Split(string(data), "\n") {
		line = strings.TrimSpace(line)
		if !strings.HasPrefix(line, "#") {
			fields := strings.Fields(line)
			if len(fields) < 4 {
				continue
			}
			var field SecLimitField
			field.Domain = fields[0]
			field.Type = fields[1]
			field.Item = fields[2]
			v, err := strconv.Atoi(fields[3])
			if err != nil {
				continue
			}
			field.Value = v
			result = append(result, field)
		}
	}
	return result
}
