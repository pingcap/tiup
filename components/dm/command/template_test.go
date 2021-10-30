package command

import (
	"bytes"
	"io"
	"strings"
	"testing"
)

func Test_DMTemplateLocalCommandSingle(t *testing.T) {
	tests := []struct {
		optKey   string
		optVal   string
		expected string
	}{
		{"user", "ubuntu", "user: \"ubuntu\""},
		{"group", "ubuntu", "group: \"ubuntu\""},
		{"ssh-port", "2222", "ssh_port: 2222"},
		{"deploy-dir", "/path/to/deploy", "deploy_dir: \"/path/to/deploy\""},
		{"data-dir", "/path/to/data", "data_dir: \"/path/to/data\""},
		{"arch", "arm64", "arch: \"arm64\""},
		{"master-servers", "a,b,c", "master_servers:\n  - host: a\n  - host: b\n  - host: c"},
		{"worker-servers", "a,b,c", "worker_servers:\n  - host: a\n  - host: b\n  - host: c"},
		{"monitoring-servers", "a,b,c", "monitoring_servers:\n  - host: a\n  - host: b\n  - host: c"},
		{"grafana-servers", "a,b,c", "grafana_servers:\n  - host: a\n  - host: b\n  - host: c"},
		{"alertmanager-servers", "a,b,c", "alertmanager_servers:\n  - host: a\n  - host: b\n  - host: c"},
	}

	for _, test := range tests {
		cmd := newTemplateCmd()
		b := bytes.NewBufferString("")
		cmd.SetOut(b)
		_ = cmd.Flags().Set("local", "true") // add --local
		_ = cmd.Flags().Set(test.optKey, test.optVal)

		if err := cmd.Execute(); err != nil {
			t.Fatal(err)
		}

		out, err := io.ReadAll(b)
		if err != nil {
			t.Fatal(err)
		}
		if !strings.Contains(string(out), test.expected) {
			t.Fatalf("expected \"%s\", got \"%s\"", test.expected, string(out))
		}
	}
}

func Test_DMTemplateLocalCommandMulti(t *testing.T) {
	cmd := newTemplateCmd()
	b := bytes.NewBufferString("")
	cmd.SetOut(b)
	_ = cmd.Flags().Set("local", "true")                    // add --local
	_ = cmd.Flags().Set("user", "ubuntu")                   // add --user=ubuntu
	_ = cmd.Flags().Set("group", "ubuntu")                  // add --group=ubuntu
	_ = cmd.Flags().Set("master-servers", "m1,m2,m3")       // add --master-servers=m1,m2,m3
	_ = cmd.Flags().Set("worker-servers", "w1,w2,w3")       // add --worker-servers=w1,w2,w3
	_ = cmd.Flags().Set("alertmanager-servers", "a1,a2,a3") // add --alertmanager-servers=a1,a2,a3

	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}

	out, err := io.ReadAll(b)
	if err != nil {
		t.Fatal(err)
	}

	for _, b := range []bool{
		strings.Contains(string(out), "user: \"ubuntu\""),
		strings.Contains(string(out), "group: \"ubuntu\""),
		strings.Contains(string(out), "master_servers:\n  - host: m1\n  - host: m2\n  - host: m3"),
		strings.Contains(string(out), "worker_servers:\n  - host: w1\n  - host: w2\n  - host: w3"),
		strings.Contains(string(out), "alertmanager_servers:\n  - host: a1\n  - host: a2\n  - host: a3"),
	} {
		if !b {
			t.Fatalf("unexpected output. got \"%s\"", string(out))
		}
	}
}

func Test_TemplateLocalCommandNoopt(t *testing.T) {
	cmd := newTemplateCmd()
	b := bytes.NewBufferString("")
	cmd.SetOut(b)
	_ = cmd.Flags().Set("local", "true") // add --local

	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}

	out, err := io.ReadAll(b)
	if err != nil {
		t.Fatal(err)
	}

	// check default output
	for _, b := range []bool{
		strings.Contains(string(out), "user: \"tidb\""),
		strings.Contains(string(out), "ssh_port: 22"),
		strings.Contains(string(out), "deploy_dir: \"/tidb-deploy\""),
		strings.Contains(string(out), "data_dir: \"/tidb-data\""),
		strings.Contains(string(out), "arch: \"amd64\""),
		strings.Contains(string(out), "master_servers:\n  - host: 172.19.0.101\n  - host: 172.19.0.102\n  - host: 172.19.0.103"),
		strings.Contains(string(out), "worker_servers:\n  - host: 172.19.0.101\n  - host: 172.19.0.102\n  - host: 172.19.0.103"),
		strings.Contains(string(out), "monitoring_servers:\n  - host: 172.19.0.101"),
		strings.Contains(string(out), "grafana_servers:\n  - host: 172.19.0.101"),
		strings.Contains(string(out), "alertmanager_servers:\n  - host: 172.19.0.101"),
	} {
		if !b {
			t.Fatalf("unexpected output. got \"%s\"", string(out))
		}
	}
}

func Test_TemplateLocalCommandValidate(t *testing.T) {
	tests := []struct {
		optKey string
		optVal string
	}{
		{"arch", "i386"},
		{"master-servers", "m1,m2"},
		{"worker-servers", "w1"},
	}

	for _, test := range tests {
		cmd := newTemplateCmd()
		b := bytes.NewBufferString("")
		cmd.SetOut(b)
		_ = cmd.Flags().Set("local", "true")          // add --local
		_ = cmd.Flags().Set(test.optKey, test.optVal) // add invalid option

		// should returns err
		if err := cmd.Execute(); err == nil {
			t.Fatal(err)
		}
	}
}
