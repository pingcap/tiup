// Copyright 2025 PingCAP, Inc.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE_2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// See the License for the specific language governing permissions and
// limitations under the License.

package instance

func (inst *TiCIInstance) getMetaConfig() map[string]any {
	config := make(map[string]any)
	tidbServers := make([]string, 0, len(inst.dbs))
	for _, db := range inst.dbs {
		tidbServers = append(tidbServers, db.DSN())
	}
	config["tidb_server.tidb_servers"] = tidbServers
	return config
}


// TiCIS3Config represents the S3 configuration for TiCI
type TiCIS3Config struct {
	Endpoint  string
	AccessKey string
	SecretKey string
	Bucket    string
	Prefix    string
}

func getDefaultTiCIMetaS3Config() *TiCIS3Config {
	return &TiCIS3Config{
		Endpoint:  "http://localhost:9000",
		AccessKey: "minioadmin",
		SecretKey: "minioadmin",
		Bucket:    "ticidefaultbucket",
		Prefix:    "tici_default_prefix",
	}
}

// GetTiCIS3ConfigFromFile reads the S3 configuration from the given config file path
func GetTiCIS3ConfigFromFile(configPath string) (*TiCIS3Config, error) {
	defaultConfig := getDefaultTiCIMetaS3Config()
	if configPath == "" {
		return defaultConfig, nil
	}
	// read the configuration file
	ticiMetaConfig, err := unmarshalConfig(configPath)
	if err != nil {
		return nil, err
	}
	s3, ok := ticiMetaConfig["s3"].(map[string]any)
	if ok {
		if v, ok := s3["endpoint"].(string); ok {
			defaultConfig.Endpoint = v
		}
		if v, ok := s3["access_key"].(string); ok {
			defaultConfig.AccessKey = v
		}
		if v, ok := s3["secret_key"].(string); ok {
			defaultConfig.SecretKey = v
		}
		if v, ok := s3["bucket"].(string); ok {
			defaultConfig.Bucket = v
		}
		if v, ok := s3["prefix"].(string); ok {
			defaultConfig.Prefix = v
		}
	}
	return defaultConfig, nil
}
