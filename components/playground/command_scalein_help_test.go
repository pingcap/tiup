package main

import (
	"bytes"
	"strings"
	"testing"
)

func TestScaleInHelp_DoesNotUseBackquotedUsageAsPlaceholder(t *testing.T) {
	cmd := newScaleIn(newCLIState())

	var buf bytes.Buffer
	cmd.SetOut(&buf)
	cmd.SetErr(&buf)

	if err := cmd.Help(); err != nil {
		t.Fatalf("help: %v", err)
	}

	out := buf.String()
	if strings.Contains(out, "--name tiup playground display") {
		t.Fatalf("unexpected --name placeholder derived from usage backquotes:\n%s", out)
	}
	if strings.Contains(out, "--pid tiup playground display --verbose") {
		t.Fatalf("unexpected --pid placeholder derived from usage backquotes:\n%s", out)
	}
}
