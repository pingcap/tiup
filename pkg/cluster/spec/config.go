package spec

import (
	"bytes"
	"reflect"

	"github.com/BurntSushi/toml"
	"github.com/pingcap/errors"
	"gopkg.in/yaml.v2"
)

// TomlConfig of an instance
type TomlConfig struct {
	mp map[string]interface{}
}

var _ yaml.Marshaler = &TomlConfig{}
var _ yaml.Unmarshaler = &TomlConfig{}

// UnmarshalYAML implements yaml.Unmarshal interface.
func (c *TomlConfig) UnmarshalYAML(unmarshal func(interface{}) error) error {
	var value interface{}
	err := unmarshal(&value)
	if err != nil {
		return errors.AddStack(err)
	}

	switch v := value.(type) {
	case string:
		err = toml.Unmarshal([]byte(v), &c.mp)
		if err != nil {
			return errors.AddStack(err)
		}
	case map[interface{}]interface{}:
		var value map[string]interface{}
		err := unmarshal(&value)
		if err != nil {
			return errors.AddStack(err)
		}

		c.mp, err = merge(value)
		if err != nil {
			return errors.AddStack(err)
		}
	default:
		return errors.Errorf("unknown type: %v", reflect.TypeOf(value))

	}

	return nil
}

// MarshalYAML implements yaml.Marshaler interface.
func (c *TomlConfig) MarshalYAML() (interface{}, error) {
	data, err := c.ToToml()
	if err != nil {
		return nil, errors.AddStack(err)
	}

	return string(data), nil
}

// ToToml return the toml format data.
func (c *TomlConfig) ToToml() (data []byte, err error) {
	buf := new(bytes.Buffer)
	enc := toml.NewEncoder(buf)
	enc.Indent = ""
	err = enc.Encode(&c.mp)
	if err != nil {
		return nil, errors.AddStack(err)
	}

	return buf.Bytes(), nil
}

// Inner return the inter config.
func (c *TomlConfig) Inner() map[string]interface{} {
	if c == nil {
		return nil
	}
	return c.mp
}

// Set the config item.
// k can be the format with "."
func (c *TomlConfig) Set(k string, v interface{}) {
	if c.mp == nil {
		c.mp = make(map[string]interface{})
	}
	c.mp[k] = v

	var err error
	c.mp, err = flattenMap(c.mp)
	if err != nil {
		panic(err)
	}
}
