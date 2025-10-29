package utils

import (
	"os"
	"testing"
)

func TestUserHome(t *testing.T) {
	os.Setenv("HOME", "/tmp/testhome")

	home := UserHome()
	if home != "/tmp/testhome" {
		t.Fatalf("expected /tmp/testhome, got %s", home)
	}
}
