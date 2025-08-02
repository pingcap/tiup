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

package queue

import (
	"reflect"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestAnyQueue(t *testing.T) {
	q := NewAnyQueue(reflect.DeepEqual)

	q.Put(true)
	q.Put(9527)

	require.Equal(t, true, q.slice[0])
	require.Equal(t, 9527, q.slice[1])

	q.Put(true)
	q.Put(9527)

	require.Equal(t, true, q.slice[2])
	require.Equal(t, 9527, q.slice[3])

	require.Equal(t, true, q.Get(true))
	require.Equal(t, true, q.Get(true))
	require.Nil(t, q.Get(true))

	require.Equal(t, 9527, q.Get(9527))
	require.Equal(t, 9527, q.Get(9527))
	require.Nil(t, q.Get(9527))
}
