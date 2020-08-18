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

package executor

import (
	"io/ioutil"
	"os"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestLocal(t *testing.T) {
	assert := require.New(t)
	local := new(Local)
	_, _, err := local.Execute("ls .", false)
	assert.Nil(err)

	// generate a src file and write some data
	src, err := ioutil.TempFile("", "")
	assert.Nil(err)
	defer os.Remove(src.Name())

	n, err := src.WriteString("src")
	assert.Nil(err)
	assert.Equal(3, n)
	err = src.Close()
	assert.Nil(err)

	// generate a dst file and just close it.
	dst, err := ioutil.TempFile("", "")
	assert.Nil(err)
	err = dst.Close()
	assert.Nil(err)
	defer os.Remove(dst.Name())

	// Transfer src to dst and check it.
	err = local.Transfer(src.Name(), dst.Name(), false)
	assert.Nil(err)

	data, err := ioutil.ReadFile(dst.Name())
	assert.Nil(err)
	assert.Equal("src", string(data))
}
