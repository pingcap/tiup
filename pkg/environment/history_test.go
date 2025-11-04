// Copyright 2025 PingCAP, Inc.
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

package environment

import (
	"net/url"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestHideSensitiveInfo(t *testing.T) {
	xxx := "******"
	uxxx := url.QueryEscape(xxx)

	cases := []struct {
		args   []string
		output []string
	}{
		{
			[]string{"tiup", "tidb-lightning", "-tidb-password", "secret"},
			[]string{"tiup", "tidb-lightning", "-tidb-password", xxx},
		},
		{
			[]string{"tiup", "tidb-lightning", "--tidb-password", "secret"},
			[]string{"tiup", "tidb-lightning", "--tidb-password", xxx},
		},
		{
			[]string{"tiup", "dumpling", "--port", "10611", "--host", "127.0.0.1", "--user", "root", "--password", "msandbox"},
			[]string{"tiup", "dumpling", "--port", "10611", "--host", "127.0.0.1", "--user", "root", "--password", xxx},
		},
		{
			[]string{"tiup", "dmctl", "--master-addr", "127.0.0.1:8261", "encrypt", "very_secret"},
			[]string{"tiup", "dmctl", "--master-addr", "127.0.0.1:8261", "encrypt", xxx},
		},
		{
			[]string{"tiup", "br", "validate", "decode", "--field=end-version", "--storage", "s3://backup-101/snapshot-?access-key=key&secret-access-key=access-key"},
			[]string{"tiup", "br", "validate", "decode", "--field=end-version", "--storage", "s3://backup-101/snapshot-?access-key=key&secret-access-key=" + uxxx},
		},
		{
			[]string{"tiup", "cdc", "cli", "changefeed", "create", "--sink-uri=kafka://example.com:9092?protocol=canal-json&sasl-password=secret"},
			[]string{"tiup", "cdc", "cli", "changefeed", "create", "--sink-uri=kafka://example.com:9092?protocol=canal-json&sasl-password=" + uxxx},
		},
		{
			[]string{"tiup", "https://user:pass@example.com"},
			[]string{"tiup", "https://user:xxxxx@example.com"}, // (*URL).Redacted does this
		},
		{
			[]string{"tiup", "bench", "tpcc", "run", "-H", "example.com", "-P", "4000", "-U", "root", "-pr9876ABC4Z2dWAL", "--threads", "10", "--time", "10m"},
			[]string{"tiup", "bench", "tpcc", "run", "-H", "example.com", "-P", "4000", "-U", "root", "-p" + xxx, "--threads", "10", "--time", "10m"},
		},
	}

	for _, tc := range cases {
		r := HideSensitiveInfo(tc.args)
		require.Equal(t, tc.output, r)
	}
}
