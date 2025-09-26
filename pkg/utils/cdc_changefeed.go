// Copyright 2025 PingCAP, Inc.
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
	"fmt"
	"os/exec"
)

// CreateChangefeed creates a changefeed using tiup cdc cli
func CreateChangefeed(cdcAddr, bucket, prefix, endpoint, accessKey, secretKey, flushInterval string) (string, error) {
	// Prepare changefeed creation command
	sinkURI := fmt.Sprintf("s3://%s/%s/cdc?protocol=canal-json&access-key=%s&secret-access-key=%s&endpoint=%s&enable-tidb-extension=true&output-row-key=true&flush-interval=%s", bucket, prefix, accessKey, secretKey, endpoint, flushInterval)

	cmd := exec.Command("tiup", "cdc:nightly", "cli", "changefeed", "create",
		fmt.Sprintf("--server=%s", cdcAddr),
		fmt.Sprintf("--sink-uri=%s", sinkURI),
	)

	output, err := cmd.CombinedOutput()
	if err != nil {
		return string(output), err
	}

	return string(output), nil
}
