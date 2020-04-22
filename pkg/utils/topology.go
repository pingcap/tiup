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

package utils

import (
	"bytes"
	"io/ioutil"

	"github.com/goccy/go-yaml"
	"github.com/pingcap-incubator/tiup-cluster/pkg/cliutil"
	"github.com/pingcap-incubator/tiup-cluster/pkg/errutil"
	"github.com/pingcap/errors"
	"go.uber.org/zap"
	"vimagination.zapto.org/dos2unix"
)

var (
	errNSTopolohy = errNS.NewSubNamespace("topology")
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

	// github.com/goccy/go-yaml will fail to marshal the topo file edit in windows with "CRCL" new line
	// one example is in testdata/topology_err.yaml
	// if we revert to use https://github.com/go-yaml/yaml we can avoid this.
	// but we must find a way to address https://github.com/go-yaml/yaml/issues/430
	// Note like 25 without decimal is not a valid float type for toml.
	yamlFile, err = ioutil.ReadAll(dos2unix.DOS2Unix(bytes.NewBuffer(yamlFile)))
	if err != nil {
		return errors.AddStack(err)
	}

	decoder := yaml.NewDecoder(bytes.NewBuffer(yamlFile), yaml.DisallowUnknownField())

	if err = decoder.Decode(out); err != nil {
		return ErrTopologyParseFailed.
			Wrap(err, "Failed to parse topology file %s", file).
			WithProperty(cliutil.SuggestionFromTemplate(`
Please check the syntax of your topology file {{ColorKeyword}}{{.File}}{{ColorReset}} and try again.
`, suggestionProps))
	}

	zap.L().Debug("Parse topology file succeeded", zap.Any("topology", out))

	return nil
}
