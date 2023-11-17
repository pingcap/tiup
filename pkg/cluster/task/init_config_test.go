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
	"context"
	"testing"
	"time"

	"github.com/pingcap/tiup/pkg/cluster/ctxt"
	"github.com/pingcap/tiup/pkg/cluster/spec"
	logprinter "github.com/pingcap/tiup/pkg/logger/printer"
	"github.com/pingcap/tiup/pkg/meta"

	"github.com/pingcap/check"
	"github.com/pingcap/errors"
	"github.com/pingcap/tiup/pkg/utils/mock"
)

type initConfigSuite struct {
}

type fakeExecutor struct {
}

func (e *fakeExecutor) Execute(ctx context.Context, cmd string, sudo bool, timeout ...time.Duration) (stdout []byte, stderr []byte, err error) {
	return []byte{}, []byte{}, nil
}

func (e *fakeExecutor) Transfer(ctx context.Context, src, dst string, download bool, limit int, compress bool) error {
	return nil
}

type fakeInstance struct {
	hasConfigError bool
	*spec.TiDBInstance
}

func (i *fakeInstance) InitConfig(
	ctx context.Context,
	e ctxt.Executor,
	clusterName string,
	clusterVersion string,
	deployUser string,
	paths meta.DirPaths,
) error {
	if i.hasConfigError {
		return errors.Annotate(spec.ErrorCheckConfig, "test error")
	}
	return nil
}

func (i *fakeInstance) GetHost() string {
	return "1.1.1.1"
}

func (i *fakeInstance) GetPort() int {
	return 4000
}

func (i *fakeInstance) GetManageHost() string {
	return "1.1.1.1"
}

func Test(t *testing.T) { check.TestingT(t) }

var _ = check.Suite(&initConfigSuite{})

func (s *initConfigSuite) TestCheckConfig(c *check.C) {
	ctx := ctxt.New(context.Background(), 0, logprinter.NewLogger(""))
	mf := mock.With("FakeExecutor", &fakeExecutor{})
	defer mf()

	t := &InitConfig{
		clusterName:    "test-cluster-name",
		clusterVersion: "v6.0.0",
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
			c.Assert(t.Execute(ctx), check.NotNil)
		} else {
			c.Assert(t.Execute(ctx), check.IsNil)
		}
	}
}
