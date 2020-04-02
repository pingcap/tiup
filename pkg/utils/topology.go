package utils

import (
	"io/ioutil"

	"github.com/pingcap-incubator/tiops/pkg/cliutil"
	"github.com/pingcap-incubator/tiops/pkg/errutil"
	"go.uber.org/zap"
	"gopkg.in/yaml.v2"
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

	if err = yaml.Unmarshal(yamlFile, out); err != nil {
		return ErrTopologyParseFailed.
			Wrap(err, "Failed to parse topology file %s", file).
			WithProperty(cliutil.SuggestionFromTemplate(`
Please check the syntax of your topology file {{ColorKeyword}}{{.File}}{{ColorReset}} and try again.
`, suggestionProps))
	}

	zap.L().Debug("Parse topology file succeeded", zap.Any("topology", out))

	return nil
}
