// Copyright 2026 PingCAP, Inc.
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

	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"
)

type renderedPrometheusConfig struct {
	Global struct {
		// Only decode global.external_labels because that is the output surface under test.
		ExternalLabels map[string]string `yaml:"external_labels"`
	} `yaml:"global"`
	RemoteWrite []struct {
		URL string `yaml:"url"`
	} `yaml:"remote_write"`
}

func decodePrometheusConfig(t *testing.T, content []byte) renderedPrometheusConfig {
	t.Helper()

	var cfg renderedPrometheusConfig
	// Decode the rendered YAML instead of string matching so the test validates a real config.
	require.NoErrorf(t, yaml.Unmarshal(content, &cfg), "failed to decode rendered prometheus config:\n%s", string(content))
	return cfg
}

func decodeExternalLabels(t *testing.T, content []byte) map[string]string {
	return decodePrometheusConfig(t, content).Global.ExternalLabels
}

func TestPrometheusConfigExternalLabelsDefaults(t *testing.T) {
	// Keep backward compatibility when no custom external_labels are provided.
	cfg := NewPrometheusConfig("test-cluster", "v6.1.0", false)

	content, err := cfg.Config()
	if err != nil {
		t.Fatalf("failed to render prometheus config: %v", err)
	}

	labels := decodeExternalLabels(t, content)
	expected := map[string]string{
		"cluster": "test-cluster",
		"monitor": "prometheus",
	}
	if len(labels) != len(expected) {
		t.Fatalf("expected %d labels, got %d: %#v", len(expected), len(labels), labels)
	}
	for key, value := range expected {
		if labels[key] != value {
			t.Fatalf("expected %s=%q, got %#v", key, value, labels)
		}
	}
}

func TestPrometheusConfigExternalLabels(t *testing.T) {
	// Include both quote styles to cover YAML escaping in custom label values.
	cfg := NewPrometheusConfig("test-cluster", "v6.1.0", false)
	cfg.SetExternalLabels(map[string]string{
		"environment": "prod'uction",
		"owner":       `sre:"primary"`,
		"region":      "us-east-1",
	})

	content, err := cfg.Config()
	if err != nil {
		t.Fatalf("failed to render prometheus config: %v", err)
	}

	labels := decodeExternalLabels(t, content)
	expected := map[string]string{
		"cluster":     "test-cluster",
		"environment": "prod'uction",
		"monitor":     "prometheus",
		"owner":       `sre:"primary"`,
		"region":      "us-east-1",
	}
	if len(labels) != len(expected) {
		t.Fatalf("expected %d labels, got %d: %#v", len(expected), len(labels), labels)
	}
	for key, value := range expected {
		if labels[key] != value {
			t.Fatalf("expected %s=%q, got %#v", key, value, labels)
		}
	}
}

func TestPrometheusConfigExternalLabelsWithRemoteWrite(t *testing.T) {
	cfg := NewPrometheusConfig("test-cluster", "v6.1.0", false)
	cfg.SetExternalLabels(map[string]string{
		"environment": "production",
		"region":      "us-east-1",
	})
	cfg.SetRemoteConfig(`remote_write:
  - url: "http://vm.example.com/api/v1/write"
`)

	content, err := cfg.Config()
	require.NoError(t, err)

	rendered := decodePrometheusConfig(t, content)
	require.Equal(t, map[string]string{
		"cluster":     "test-cluster",
		"environment": "production",
		"monitor":     "prometheus",
		"region":      "us-east-1",
	}, rendered.Global.ExternalLabels)
	require.Len(t, rendered.RemoteWrite, 1)
	require.Equal(t, "http://vm.example.com/api/v1/write", rendered.RemoteWrite[0].URL)
}

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
