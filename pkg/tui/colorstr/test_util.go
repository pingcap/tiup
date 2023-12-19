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

// RequireEqualColorToken compares whether the actual string is equal to the expected string after color processing.
func RequireEqualColorToken(t *testing.T, expectColorTokens string, actualString string) {
	require.Equal(t, DefaultTokens.Color(expectColorTokens), actualString)
}

// RequireNotEqualColorToken compares whether the actual string is not equal to the expected string after color processing.
func RequireNotEqualColorToken(t *testing.T, expectColorTokens string, actualString string) {
	require.NotEqual(t, DefaultTokens.Color(expectColorTokens), actualString)
}
