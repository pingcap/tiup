package main

import (
	"os"
	"path/filepath"
	"testing"
)

func TestGetAbsolutePath(t *testing.T) {
	t.Run("Empty", func(t *testing.T) {
		got, err := getAbsolutePath("")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if got != "" {
			t.Fatalf("unexpected path: %q", got)
		}
	})

	t.Run("Relative", func(t *testing.T) {
		wd, err := os.Getwd()
		if err != nil {
			t.Fatalf("Getwd: %v", err)
		}

		got, err := getAbsolutePath("a/b")
		if err != nil {
			t.Fatalf("getAbsolutePath: %v", err)
		}

		want, err := filepath.Abs(filepath.Join(wd, "a/b"))
		if err != nil {
			t.Fatalf("Abs: %v", err)
		}
		if got != want {
			t.Fatalf("unexpected path: got %q want %q", got, want)
		}
	})

	t.Run("HomeExpansion", func(t *testing.T) {
		home, err := os.UserHomeDir()
		if err != nil || home == "" {
			t.Skipf("os.UserHomeDir unavailable: %v", err)
		}

		got, err := getAbsolutePath("~/a/b")
		if err != nil {
			t.Fatalf("getAbsolutePath: %v", err)
		}

		want, err := filepath.Abs(filepath.Join(home, "a/b"))
		if err != nil {
			t.Fatalf("Abs: %v", err)
		}
		if got != want {
			t.Fatalf("unexpected path: got %q want %q", got, want)
		}
	})
}
