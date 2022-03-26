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

package manager

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestGetGrafanaURLStr(t *testing.T) {
	var str string
	var exist bool

	str, exist = getGrafanaURLStr([]InstInfo{
		{
			Role: "grafana",
			Port: 3000,
			Host: "127.0.0.1",
		}, {
			Role: "others",
			Port: 3000,
			Host: "127.0.0.1",
		},
	})
	assert.Equal(t, exist, true)
	assert.Equal(t, "http://127.0.0.1:3000", str)

	str, exist = getGrafanaURLStr([]InstInfo{
		{
			Role: "grafana",
			Port: 3000,
			Host: "127.0.0.1",
		}, {
			Role: "grafana",
			Port: 3000,
			Host: "127.0.0.2",
		},
	})
	assert.Equal(t, exist, true)
	assert.Equal(t, "http://127.0.0.1:3000,http://127.0.0.2:3000", str)

	_, exist = getGrafanaURLStr([]InstInfo{
		{
			Role: "others",
			Port: 3000,
			Host: "127.0.0.1",
		}, {
			Role: "others",
			Port: 3000,
			Host: "127.0.0.2",
		},
	})
	assert.Equal(t, exist, false)
}
