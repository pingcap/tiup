// Copyright 2017 TiKV Project Authors.
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

	"github.com/stretchr/testify/require"
)

func TestStringSliceJSON(t *testing.T) {
	b := StringSlice([]string{"zone", "rack"})
	o, err := json.Marshal(b)
	require.NoError(t, err)
	require.Equal(t, "\"zone,rack\"", string(o))

	var nb StringSlice
	err = json.Unmarshal(o, &nb)
	require.NoError(t, err)
	require.Equal(t, b, nb)
}

func TestStringSliceEmpty(t *testing.T) {
	ss := StringSlice([]string{})
	b, err := json.Marshal(ss)
	require.NoError(t, err)
	require.Equal(t, "\"\"", string(b))

	var ss2 StringSlice
	require.NoError(t, ss2.UnmarshalJSON(b))
	require.Equal(t, ss, ss2)
}
