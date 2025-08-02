// Copyright 2016 TiKV Project Authors.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package typeutil

import (
	"encoding/json"
	"testing"

	"github.com/BurntSushi/toml"
	"github.com/stretchr/testify/require"
)

type example struct {
	Interval Duration `json:"interval" toml:"interval"`
}

func TestDurationJSON(t *testing.T) {
	ex := &example{}

	text := []byte(`{"interval":"1h1m1s"}`)
	require.NoError(t, json.Unmarshal(text, ex))
	require.Equal(t, float64(60*60+60+1), ex.Interval.Seconds())

	b, err := json.Marshal(ex)
	require.NoError(t, err)
	require.Equal(t, string(text), string(b))
}

func TestDurationTOML(t *testing.T) {
	ex := &example{}

	text := []byte(`interval = "1h1m1s"`)
	require.NoError(t, toml.Unmarshal(text, ex))
	require.Equal(t, float64(60*60+60+1), ex.Interval.Seconds())
}
