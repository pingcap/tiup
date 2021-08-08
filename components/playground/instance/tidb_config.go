package instance

import (
	"github.com/BurntSushi/toml"
	"github.com/pingcap/tidb/config"
)

// GetTiDBPort custom db port for playground using config file
func GetTiDBPort(configPath string) int {
	// default port
	port := 4000
	// read from config file if possible
	config := &config.Config{}
	_, err := toml.DecodeFile(configPath, config)
	if err == nil && config.Port > 0 {
		port = int(config.Port)
	}
	return port
}
