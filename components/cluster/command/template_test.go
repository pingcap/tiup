package command

import (
	"bytes"
	"io"
	"strings"
	"testing"
)

func Test_TemplateLocalCommandSingle(t *testing.T) {
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
		{"pd-servers", "a,b,c", "pd_servers:\n  - host: a\n  - host: b\n  - host: c"},
		{"tidb-servers", "a,b,c", "tidb_servers:\n  - host: a\n  - host: b\n  - host: c"},
		{"tikv-servers", "a,b,c", "tikv_servers:\n  - host: a\n  - host: b\n  - host: c"},
		{"tiflash-servers", "a,b,c", "tiflash_servers:\n  - host: a\n  - host: b\n  - host: c"},
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

func Test_TemplateLocalCommandMulti(t *testing.T) {
	cmd := newTemplateCmd()
	b := bytes.NewBufferString("")
	cmd.SetOut(b)
	_ = cmd.Flags().Set("local", "true")                 // add --local
	_ = cmd.Flags().Set("user", "ubuntu")                // add --user=ubuntu
	_ = cmd.Flags().Set("group", "ubuntu")               // add --group=ubuntu
	_ = cmd.Flags().Set("tidb-servers", "a,b,c")         // add --tidb-servers=a,b,c
	_ = cmd.Flags().Set("alertmanager-servers", "a,b,c") // add --alertmanager-servers=a,b,c

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
		strings.Contains(string(out), "tidb_servers:\n  - host: a\n  - host: b\n  - host: c"),
		strings.Contains(string(out), "alertmanager_servers:\n  - host: a\n  - host: b\n  - host: c"),
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
		strings.Contains(string(out), "pd_servers:\n  - host: 127.0.0.1"),
		strings.Contains(string(out), "tidb_servers:\n  - host: 127.0.0.1"),
		strings.Contains(string(out), "tikv_servers:\n  - host: 127.0.0.1"),
		strings.Contains(string(out), "monitoring_servers:\n  - host: 127.0.0.1"),
		strings.Contains(string(out), "grafana_servers:\n  - host: 127.0.0.1"),
	} {
		if !b {
			t.Fatalf("unexpected output. got \"%s\"", string(out))
		}
	}
}

func Test_TemplateLocalCommandValidate(t *testing.T) {
	cmd := newTemplateCmd()
	b := bytes.NewBufferString("")
	cmd.SetOut(b)
	_ = cmd.Flags().Set("local", "true") // add --local
	_ = cmd.Flags().Set("arch", "i386")  // add --arch=i386 (invalid)

	// should returns err
	if err := cmd.Execute(); err == nil {
		t.Fatal(err)
	}
}
