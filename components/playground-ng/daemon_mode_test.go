package main

import (
	"os"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestBuildDaemonArgs_FiltersBackgroundAndTagFlags(t *testing.T) {
	oldArgs := os.Args
	t.Cleanup(func() { os.Args = oldArgs })

	os.Args = []string{
		"tiup-playground-ng",
		"--background",
		"--tag",
		"old",
		"--host",
		"127.0.0.1",
		"--run-as-daemon",
	}

	got := buildDaemonArgs("new")
	require.Equal(t, []string{
		"--host",
		"127.0.0.1",
		"--tag",
		"new",
		"--run-as-daemon",
	}, got)
}

func TestBuildDaemonArgs_FiltersShortTagForms(t *testing.T) {
	oldArgs := os.Args
	t.Cleanup(func() { os.Args = oldArgs })

	os.Args = []string{
		"tiup-playground-ng",
		"-d",
		"-Tfoo",
		"-T",
		"bar",
		"--tag=baz",
		"--host",
		"127.0.0.1",
	}

	got := buildDaemonArgs("final")
	require.Equal(t, []string{
		"--host",
		"127.0.0.1",
		"--tag",
		"final",
		"--run-as-daemon",
	}, got)
}

func TestBuildDaemonArgs_FiltersEqualsFormsAndKeepsPositionals(t *testing.T) {
	oldArgs := os.Args
	t.Cleanup(func() { os.Args = oldArgs })

	os.Args = []string{
		"tiup-playground-ng",
		"--background=true",
		"--run-as-daemon=true",
		"--tag=old",
		"v8.5.4",
		"--host",
		"127.0.0.1",
	}

	got := buildDaemonArgs("final")
	require.Equal(t, []string{
		"v8.5.4",
		"--host",
		"127.0.0.1",
		"--tag",
		"final",
		"--run-as-daemon",
	}, got)
}

func TestBackgroundStarterReadyMessage(t *testing.T) {
	require.Equal(t, "\n[dim]Cluster running in background (-d).[reset]\n[dim]To stop: [reset]tiup playground-ng stop --tag foo\n", backgroundStarterReadyMessage("foo"))
}
