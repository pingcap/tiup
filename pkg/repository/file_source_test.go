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

package repository

import (
	"errors"
	"strings"
	"testing"

	"github.com/pingcap/tiup/pkg/utils"
	"github.com/stretchr/testify/assert"
)

func TestCheckSHA(t *testing.T) {
	// Correct.
	reader := strings.NewReader("Lorem ipsum dolor sit amet, consectetur adipiscing elit, sed do eiusmod tempor incididunt ut labore et dolore magna aliqua. Ut enim ad minim veniam, quis nostrud exercitation ullamco laboris nisi ut aliquip ex ea commodo consequat. Duis aute irure dolor in reprehenderit in voluptate velit esse cillum dolore eu fugiat nulla pariatur. Excepteur sint occaecat cupidatat non proident, sunt in culpa qui officia deserunt mollit anim id est laborum.")
	expected := "cd36b370758a259b34845084a6cc38473cb95e27"
	err := utils.CheckSHA(reader, expected)
	assert.Nil(t, err)

	// Incorrect.
	reader = strings.NewReader("Lorem ipsum dolor sit amet, consectetur adipiscing elit, sed do eiusmod tempor incididuntut labore et dolore magna aliqua. Ut enim ad minim veniam, quis nostrud exercitation ullamco laboris nisi ut aliquip ex ea commodo consequat. Duis aute irure dolor in reprehenderit in voluptate velit esse cillum dolore eu fugiat nulla pariatur. Excepteur sint occaecat cupidatat non proident, sunt in culpa qui officia deserunt mollit anim id est laborum.")
	err = utils.CheckSHA(reader, expected)
	assert.NotNil(t, err)

	// Edge cases.
	reader = strings.NewReader("")
	expected = "da39a3ee5e6b4b0d3255bfef95601890afd80709"
	err = utils.CheckSHA(reader, expected)
	assert.Nil(t, err)

	err = utils.CheckSHA(badReader{}, expected)
	assert.NotNil(t, err)
}

type badReader struct{}

func (reader badReader) Read(_ []byte) (int, error) {
	return 0, errors.New("bad")
}
