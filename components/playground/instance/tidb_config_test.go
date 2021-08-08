package instance

import (
	"bytes"
	"os"
	"testing"

	"github.com/BurntSushi/toml"
	"github.com/pingcap/tidb/config"
	"github.com/stretchr/testify/assert"
)

func TestGetTiDBPort(t *testing.T) {
	defaultPort := 4000
	assert.Equal(t, GetTiDBPort(""), defaultPort)

	customPort := 30000
	inputs := config.Config{
		Port: uint(customPort),
	}
	var firstBuffer bytes.Buffer
	e := toml.NewEncoder(&firstBuffer)
	err := e.Encode(inputs)
	assert.Nil(t, err)
	// create temp config file
	tmp, err := os.CreateTemp(os.TempDir(), "tmp-")
	assert.Nil(t, err)
	defer tmp.Close()
	_, err = tmp.Write(firstBuffer.Bytes())
	assert.Nil(t, err)

	assert.Equal(t, GetTiDBPort(tmp.Name()), customPort)
}
