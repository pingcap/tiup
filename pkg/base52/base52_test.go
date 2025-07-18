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

package base52

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestEncode(t *testing.T) {
	require.Equal(t, "2TPzw7", Encode(1000000000))
}

func TestDecode(t *testing.T) {
	decoded, err := Decode("2TPzw7")
	require.Equal(t, int64(1000000000), decoded)
	require.Nil(t, err)

	decoded, err = Decode("../../etc/passwd")
	require.Equal(t, int64(0), decoded)
	require.NotNil(t, err)
}
