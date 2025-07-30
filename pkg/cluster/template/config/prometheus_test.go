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

package config

import (
	"strings"
	"testing"
)

func TestPrometheusConfigWithAgentMode(t *testing.T) {
	cfg := NewPrometheusConfig("test-cluster", "v6.1.0", false)
	cfg.AddPD("127.0.0.1", 2379)
	cfg.AddTiDB("127.0.0.1", 10080)
	cfg.AddTiKV("127.0.0.1", 20180)

	// Test normal mode config
	normalConfig, err := cfg.Config()
	if err != nil {
		t.Fatalf("Failed to generate normal config: %v", err)
	}

	// Verify that normal config contains rule_files
	if !strings.Contains(string(normalConfig), "rule_files:") {
		t.Error("Normal config should contain rule_files section")
	}

	// Test agent mode config
	agentConfig, err := cfg.ConfigWithAgentMode(true)
	if err != nil {
		t.Fatalf("Failed to generate agent config: %v", err)
	}

	// Verify that agent config doesn't contain rule_files
	if strings.Contains(string(agentConfig), "rule_files:") {
		t.Error("Agent mode config should not contain rule_files section")
	}

	// Verify that agent config contains scrape_configs
	if !strings.Contains(string(agentConfig), "scrape_configs:") {
		t.Error("Agent mode config should contain scrape_configs section")
	}
}

func TestConfigToFileWithAgentMode(t *testing.T) {
	// This is just a basic test to ensure the function doesn't panic
	// For real file operations, we'd need to use a test directory
	cfg := NewPrometheusConfig("test-cluster", "v6.1.0", false)

	// Generate a config string directly instead of writing to file
	agentConfig, err := cfg.ConfigWithAgentMode(true)
	if err != nil {
		t.Fatalf("Failed to generate agent config: %v", err)
	}

	// Verify basic structure of the output
	if !strings.Contains(string(agentConfig), "cluster: 'test-cluster'") {
		t.Error("Agent config should contain cluster name")
	}

	// Verify that rule_files section is removed
	if strings.Contains(string(agentConfig), "rule_files:") {
		t.Error("Agent mode config should not contain rule_files section")
	}
}
