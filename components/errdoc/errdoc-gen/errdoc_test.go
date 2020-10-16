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

package main

import (
	"io/ioutil"
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/pingcap/tiup/components/errdoc/spec"
)

func TestErrdoc(t *testing.T) {
	var files = []struct {
		path    string
		comment string
		content string
		errors  []*spec.ErrorSpec
	}{
		{
			path:    "a/b/c/test1.go",
			comment: "",
			content: `
			package c
			import (
				"github.com/pingcap/errors"
			)

			var ErrTest1 = errors.Normalize()
			var (
				ErrTest2 = errors.Normalize()
				ErrTest3 = errors.Normalize()
			)
			`,
			errors: []*spec.ErrorSpec{
				{
					Code: "123546",
				},
			},
		},
	}

	tmpDir := t.TempDir()
	for _, file := range files {
		path := filepath.Join(tmpDir, file.path)
		parent := filepath.Dir(path)

		err := os.MkdirAll(parent, 0755)
		if err != nil {
			t.Fatalf("Create directory %s failed: %v", parent, err)
		}
		err = ioutil.WriteFile(path, []byte(file.content), os.ModePerm)
		if err != nil {
			t.Fatalf("Write file %s failed: %v", path, err)
		}

		errSpecs, err := errdoc(parent)
		if err != nil {
			t.Fatalf("Export error documentation %s(%s) failed: %v", file.path, file.comment, err)
		}
		if !reflect.DeepEqual(file.errors, errSpecs) {
			t.Fatalf("Test case %s(%s) failed:\n==> expected:\n%#v\n==> got:\n%+v",
				file.path, file.comment, file.errors, errSpecs)
		}
	}
}
