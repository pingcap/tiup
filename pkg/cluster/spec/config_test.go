package spec

import (
	"testing"

	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v2"
)

type Server struct {
	A      int
	Config *TomlConfig
}

func TestTomlConfigString(t *testing.T) {
	a := Server{
		A: 1,
		Config: &TomlConfig{
			mp: map[string]interface{}{
				"a": 1,
				"b": 2,
			},
		},
	}

	out, err := yaml.Marshal(&a)
	assert := require.New(t)
	assert.Nil(err)

	expectOut := `a: 1
config: |
  a = 1
  b = 2
`
	assert.Equal(expectOut, string(out))

	b := new(Server)
	err = yaml.Unmarshal(out, &b)
	assert.Nil(err)
	bout, err := yaml.Marshal(&a)
	assert.Nil(err)
	assert.Equal(expectOut, string(bout))
}

func TestTomlConfigMap(t *testing.T) {
	out := `a: 1
config:
  g: 1
  a.b: 1
  a.c: 1
  a.a.b: 1
  a.a.c: 1
`
	srv := new(Server)
	err := yaml.Unmarshal([]byte(out), srv)
	assert := require.New(t)
	assert.Nil(err)
	assert.Equal(1, srv.A)
	assert.Equal(map[string]interface{}{
		"g": 1,
		"a": map[string]interface{}{
			"b": 1,
			"c": 1,
			"a": map[string]interface{}{
				"b": 1,
				"c": 1,
			},
		},
	}, srv.Config.mp)

	tomlData, err := srv.Config.ToToml()
	assert.Nil(err)
	expectToml := `g = 1

[a]
b = 1
c = 1
[a.a]
b = 1
c = 1
`
	assert.Equal(expectToml, string(tomlData))

	data, err := yaml.Marshal(srv)
	assert.Nil(err)
	expectYAML := `a: 1
config: |
  g = 1

  [a]
  b = 1
  c = 1
  [a.a]
  b = 1
  c = 1
`
	assert.Equal(expectYAML, string(data))
}

func TestTomlConfigSetValue(t *testing.T) {
	c := new(TomlConfig)

	c.Set("a", 1)
	c.Set("binlog.enable", true)

	_, err := c.ToToml()
	assert := require.New(t)
	assert.Nil(err)
	assert.Equal(map[string]interface{}{
		"a": 1,
		"binlog": map[string]interface{}{
			"enable": true,
		},
	}, c.mp)
}
