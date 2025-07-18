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

package set

import (
	"reflect"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestAnySet(t *testing.T) {
	set := NewAnySet(reflect.DeepEqual)
	set.Insert(true)
	set.Insert(9527)

	require.Equal(t, true, set.slice[0])
	require.Equal(t, true, set.Slice()[0])

	require.Equal(t, 9527, set.slice[1])
	require.Equal(t, 9527, set.Slice()[1])
}
