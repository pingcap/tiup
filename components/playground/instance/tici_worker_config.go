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

func (inst *TiCIInstance) getWorkerConfig() map[string]any {
	config := make(map[string]any)
	defaultS3Config := getDefaultTiCIMetaS3Config()
	config["s3.endpoint"] = defaultS3Config.Endpoint
	config["s3.access_key"] = defaultS3Config.AccessKey
	config["s3.secret_key"] = defaultS3Config.SecretKey
	config["s3.bucket"] = defaultS3Config.Bucket
	config["s3.prefix"] = defaultS3Config.Prefix
	return config
}

// GetCDCS3FlushIntervalFromFile reads the CDC S3 flush interval from the given config file path
func GetCDCS3FlushIntervalFromFile(configPath string) (string, error) {
	defaultInterval := "5s"
	if configPath == "" {
		return defaultInterval, nil
	}
	// read the configuration file
	ticiWorkerConfig, err := unmarshalConfig(configPath)
	if err != nil {
		return "", err
	}
	if _, ok := ticiWorkerConfig["cdc_s3_flush_interval"]; ok {
		if val, ok := ticiWorkerConfig["cdc_s3_flush_interval"].(string); ok {
			return val, nil
		}
	}
	return defaultInterval, nil
}
