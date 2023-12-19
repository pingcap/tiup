// Copyright 2024 PingCAP, Inc.
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

package colorstr

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestDefaultTokens(t *testing.T) {
	require.Equal(t, "[blue", DefaultTokens.Sprintf("[blue"))
	require.Equal(t, "hello", DefaultTokens.Sprintf("hello"))
	require.Equal(t, "\x1B[34mhello\x1B[0m", DefaultTokens.Sprintf("[blue]hello"))
	require.Equal(t, "\x1B[34mhello\x1B[0m\x1B[0m", DefaultTokens.Sprintf("[blue]hello[reset]"))
	require.Equal(t, "\x1B[34mhello\x1B[0mfoo\x1B[0m", DefaultTokens.Sprintf("[blue]hello[reset]foo"))
	require.Equal(t, "\x1B[34mhello\x1B[31mfoo\x1B[0m", DefaultTokens.Sprintf("[blue]hello[red]foo"))
	require.Equal(t, "\x1B[34mhello [blue]\x1B[0m", DefaultTokens.Sprintf("[blue]hello %s", "[blue]"))
	require.Equal(t, "[ahh]hello", DefaultTokens.Sprintf("[ahh]hello"))
}
