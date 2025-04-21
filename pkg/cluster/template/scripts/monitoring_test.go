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

package scripts

import (
	"os"
	"strings"
	"testing"
)

func TestPrometheusScriptWithAgentMode(t *testing.T) {
	// Create a temporary file for testing
	tmpfile, err := os.CreateTemp("", "prometheus_script_test_*.sh")
	if err != nil {
		t.Fatalf("Failed to create temp file: %v", err)
	}
	defer os.Remove(tmpfile.Name())
	defer tmpfile.Close()

	// Initialize Prometheus script with agent mode enabled
	script := &PrometheusScript{
		Port:           9090,
		WebExternalURL: "http://localhost:9090",
		Retention:      "30d",
		EnableNG:       false,
		EnableAgent:    true,
		DeployDir:      "/deploy",
		LogDir:         "/log",
		DataDir:        "/data",
		NumaNode:       "",
		AdditionalArgs: []string{"--some-additional-arg=value"},
	}

	// Write to the temp file
	err = script.ConfigToFile(tmpfile.Name())
	if err != nil {
		t.Fatalf("Failed to write config to file: %v", err)
	}

	// Read the file content
	content, err := os.ReadFile(tmpfile.Name())
	if err != nil {
		t.Fatalf("Failed to read file: %v", err)
	}

	// Convert to string for easier assertions
	contentStr := string(content)

	// Verify agent mode flag is included
	if !strings.Contains(contentStr, "--enable-feature=agent") {
		t.Error("Agent mode script should contain the --enable-feature=agent flag")
	}

	// Verify storage flags are not present in agent mode
	if strings.Contains(contentStr, "--storage.tsdb.path") && strings.Contains(contentStr, "--storage.tsdb.retention") {
		t.Error("Agent mode script should not contain storage flags")
	}

	// Now test with agent mode disabled
	tmpfile2, err := os.CreateTemp("", "prometheus_script_test_normal_*.sh")
	if err != nil {
		t.Fatalf("Failed to create second temp file: %v", err)
	}
	defer os.Remove(tmpfile2.Name())
	defer tmpfile2.Close()

	// Initialize Prometheus script with agent mode disabled
	script.EnableAgent = false

	// Write to the second temp file
	err = script.ConfigToFile(tmpfile2.Name())
	if err != nil {
		t.Fatalf("Failed to write normal config to file: %v", err)
	}

	// Read the second file content
	content2, err := os.ReadFile(tmpfile2.Name())
	if err != nil {
		t.Fatalf("Failed to read second file: %v", err)
	}

	contentStr2 := string(content2)

	// Verify normal mode doesn't have agent flag
	if strings.Contains(contentStr2, "--enable-feature=agent") {
		t.Error("Normal mode script should not contain the --enable-feature=agent flag")
	}

	// Verify storage flags are present in normal mode
	if !strings.Contains(contentStr2, "--storage.tsdb.path") || !strings.Contains(contentStr2, "--storage.tsdb.retention") {
		t.Error("Normal mode script should contain storage flags")
	}
}
