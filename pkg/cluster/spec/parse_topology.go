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

package spec

import (
	"io/ioutil"
	"path"
	"reflect"
	"strings"

	"github.com/joomcode/errorx"
	"github.com/pingcap/tiup/pkg/cliutil"
	"github.com/pingcap/tiup/pkg/errutil"
	"go.uber.org/zap"
	"gopkg.in/yaml.v2"
)

var (
	defaultDeployUser = "tidb"
	errNSTopolohy     = errorx.NewNamespace("topology")
	// ErrTopologyReadFailed is ErrTopologyReadFailed
	ErrTopologyReadFailed = errNSTopolohy.NewType("read_failed", errutil.ErrTraitPreCheck)
	// ErrTopologyParseFailed is ErrTopologyParseFailed
	ErrTopologyParseFailed = errNSTopolohy.NewType("parse_failed", errutil.ErrTraitPreCheck)
)

// ParseTopologyYaml read yaml content from `file` and unmarshal it to `out`
func ParseTopologyYaml(file string, out interface{}) error {
	suggestionProps := map[string]string{
		"File": file,
	}

	zap.L().Debug("Parse topology file", zap.String("file", file))

	yamlFile, err := ioutil.ReadFile(file)
	if err != nil {
		return ErrTopologyReadFailed.
			Wrap(err, "Failed to read topology file %s", file).
			WithProperty(cliutil.SuggestionFromTemplate(`
Please check whether your topology file {{ColorKeyword}}{{.File}}{{ColorReset}} exists and try again.

To generate a sample topology file:
  {{ColorCommand}}{{OsArgs0}} template topology > topo.yaml{{ColorReset}}
`, suggestionProps))
	}

	if err = yaml.UnmarshalStrict(yamlFile, out); err != nil {
		return ErrTopologyParseFailed.
			Wrap(err, "Failed to parse topology file %s", file).
			WithProperty(cliutil.SuggestionFromTemplate(`
Please check the syntax of your topology file {{ColorKeyword}}{{.File}}{{ColorReset}} and try again.
`, suggestionProps))
	}

	fixRelativePath(deployUser(out), out)

	zap.L().Debug("Parse topology file succeeded", zap.Any("topology", out))

	return nil
}

func fixRelativePath(user string, topo interface{}) {
	v := reflect.ValueOf(topo).Elem()

	switch v.Kind() {
	case reflect.Slice:
		for i := 0; i < v.Len(); i++ {
			ref := reflect.New(v.Index(i).Type())
			ref.Elem().Set(v.Index(i))
			fixRelativePath(user, ref.Interface())
			v.Index(i).Set(ref.Elem())
		}
	case reflect.Struct:
		dirs := []string{"DeployDir", "DataDir", "LogDir"}
		for _, dir := range dirs {
			f := v.FieldByName(dir)
			if !f.IsValid() || f.String() == "" {
				continue
			}
			if dir == "DeployDir" {
				f.SetString(Abs(user, f.String()))
			} else if !strings.HasPrefix(f.String(), "/") {
				f.SetString(path.Join(v.FieldByName("DeployDir").String(), f.String()))
			}
		}
		for i := 0; i < v.NumField(); i++ {
			ref := reflect.New(v.Field(i).Type())
			ref.Elem().Set(v.Field(i))
			fixRelativePath(user, ref.Interface())
			v.Field(i).Set(ref.Elem())
		}
	case reflect.Ptr:
		fixRelativePath(user, v.Interface())
	}
}

func deployUser(topo interface{}) string {
	base := topo.(Topology).BaseTopo()
	if base.GlobalOptions == nil || base.GlobalOptions.User == "" {
		return defaultDeployUser
	}
	return base.GlobalOptions.User
}
