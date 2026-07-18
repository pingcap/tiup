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

	"gopkg.in/yaml.v3"
)

type renderedPrometheusConfig struct {
	Global struct {
		// 这里直接只解析 global.external_labels，测试只关心本次改动真正影响的输出片段。
		ExternalLabels map[string]string `yaml:"external_labels"`
	} `yaml:"global"`
}

func decodeExternalLabels(t *testing.T, content []byte) map[string]string {
	t.Helper()

	var cfg renderedPrometheusConfig
	// 这里把渲染结果重新反序列化成 YAML 结构，而不是只做字符串 contains，这样能真正验证结果是合法 YAML。
	if err := yaml.Unmarshal(content, &cfg); err != nil {
		t.Fatalf("failed to decode rendered prometheus config: %v\n%s", err, string(content))
	}
	// 这里只返回 external_labels，方便下面的测试直接对 map 做键值断言。
	return cfg.Global.ExternalLabels
}

func TestPrometheusConfigExternalLabelsDefaults(t *testing.T) {
	// 这里构造一个完全不带自定义标签的默认 Prometheus 配置，用来验证 backward compatibility。
	cfg := NewPrometheusConfig("test-cluster", "v6.1.0", false)

	content, err := cfg.Config()
	if err != nil {
		t.Fatalf("failed to render prometheus config: %v", err)
	}

	// 这里解析渲染结果，确认在没有 external_labels 输入时仍然只保留 TiUP 默认的两个标签。
	labels := decodeExternalLabels(t, content)
	expected := map[string]string{
		"cluster": "test-cluster",
		"monitor": "prometheus",
	}
	// 这里先校验标签总数，确保默认情况下不会平白多渲染出其他标签。
	if len(labels) != len(expected) {
		t.Fatalf("expected %d labels, got %d: %#v", len(expected), len(labels), labels)
	}
	for key, value := range expected {
		// 这里逐项校验 cluster/monitor 的默认值，确保这次改动没有破坏已有行为。
		if labels[key] != value {
			t.Fatalf("expected %s=%q, got %#v", key, value, labels)
		}
	}
}

func TestPrometheusConfigExternalLabels(t *testing.T) {
	// 这里构造带自定义 external_labels 的配置，并故意放入单引号和双引号，验证 YAML 转义路径是否安全。
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

	// 这里再次把渲染后的 YAML 解回来，验证特殊字符值并不会破坏最终配置结构。
	labels := decodeExternalLabels(t, content)
	expected := map[string]string{
		"cluster":     "test-cluster",
		"environment": "prod'uction",
		"monitor":     "prometheus",
		"owner":       `sre:"primary"`,
		"region":      "us-east-1",
	}
	// 这里先确认最终 external_labels 的数量符合预期，确保自定义标签和默认标签都在。
	if len(labels) != len(expected) {
		t.Fatalf("expected %d labels, got %d: %#v", len(expected), len(labels), labels)
	}
	for key, value := range expected {
		// 这里逐项确认默认标签和自定义标签都被正确渲染并且值没有因为转义而发生变化。
		if labels[key] != value {
			t.Fatalf("expected %s=%q, got %#v", key, value, labels)
		}
	}
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
