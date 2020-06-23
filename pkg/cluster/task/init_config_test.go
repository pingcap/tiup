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

package task

import (
	"testing"
	"time"

	"github.com/pingcap/check"
	"github.com/pingcap/errors"
	"github.com/pingcap/tiup/pkg/cluster/executor"
	"github.com/pingcap/tiup/pkg/cluster/meta"
	"github.com/pingcap/tiup/pkg/utils/mock"
)

type initConfigSuite struct {
}

type fakeExecutor struct {
}

func (e *fakeExecutor) Execute(cmd string, sudo bool, timeout ...time.Duration) (stdout []byte, stderr []byte, err error) {
	return []byte{}, []byte{}, nil
}

func (e *fakeExecutor) Transfer(src string, dst string, download bool) error {
	return nil
}

type fakeInstance struct {
	hasConfigError bool
	*meta.TiDBInstance
}

func (i *fakeInstance) InitConfig(e executor.TiOpsExecutor, clusterName string, clusterVersion string, deployUser string, paths meta.DirPaths) error {
	if i.hasConfigError {
		return errors.Annotate(meta.ErrorCheckConfig, "test error")
	}
	return nil
}

func (i *fakeInstance) GetHost() string {
	return "1.1.1.1"
}

func (i *fakeInstance) GetPort() int {
	return 4000
}

func Test(t *testing.T) { check.TestingT(t) }

var _ = check.Suite(&initConfigSuite{})

func (s *initConfigSuite) TestCheckConfig(c *check.C) {
	defer mock.With("FakeExecutor", &fakeExecutor{})()

	t := &InitConfig{
		clusterName:    "test-cluster-name",
		clusterVersion: "v4.0.0",
		paths: meta.DirPaths{
			Cache: "/tmp",
		},
	}

	tests := [][]bool{
		{false, false, false}, // hasConfigError, ignoreConfigError, expectError
		{true, false, true},
		{false, true, false},
		{true, true, false},
	}

	for _, test := range tests {
		t.instance = &fakeInstance{test[0], nil}
		t.ignoreCheck = test[1]
		if test[2] {
			c.Assert(t.Execute(nil), check.NotNil)
		} else {
			c.Assert(t.Execute(nil), check.IsNil)
		}
	}
}
