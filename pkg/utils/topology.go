package utils

import (
	"io/ioutil"

	"github.com/pingcap/errors"
	"gopkg.in/yaml.v2"
)

// ParseYaml read yaml content from `file` and unmarshal it to `out`
func ParseYaml(file string, out interface{}) error {
	yamlFile, err := ioutil.ReadFile(file)
	if err != nil {
		return errors.Trace(err)
	}
	if err = yaml.Unmarshal(yamlFile, out); err != nil {
		return errors.Trace(err)
	}
	return nil
}
