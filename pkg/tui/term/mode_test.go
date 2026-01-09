package term

import (
	"bytes"
	"testing"
)

func TestResolve_DefaultNonTTY(t *testing.T) {
	t.Setenv(EnvNoColor, "")
	t.Setenv(EnvForceColor, "")
	t.Setenv(EnvForceTTY, "")

	got := Resolve(&bytes.Buffer{})
	if got.Color || got.Control {
		t.Fatalf("expected default non-tty to disable color/control, got %+v", got)
	}
}

func TestResolve_NO_COLOR(t *testing.T) {
	t.Setenv(EnvNoColor, "1")
	t.Setenv(EnvForceColor, "1")
	t.Setenv(EnvForceTTY, "1")

	got := Resolve(&bytes.Buffer{})
	if got.Color || got.Control {
		t.Fatalf("expected NO_COLOR to disable everything, got %+v", got)
	}
}

func TestResolve_FORCE_COLOR_NonTTY(t *testing.T) {
	t.Setenv(EnvNoColor, "")
	t.Setenv(EnvForceColor, "1")
	t.Setenv(EnvForceTTY, "")

	got := Resolve(&bytes.Buffer{})
	if !got.Color || got.Control {
		t.Fatalf("expected FORCE_COLOR(non-tty) => color on, control off, got %+v", got)
	}
}

func TestResolve_FORCE_TTY_NonTTY(t *testing.T) {
	t.Setenv(EnvNoColor, "")
	t.Setenv(EnvForceColor, "")
	t.Setenv(EnvForceTTY, "1")

	got := Resolve(&bytes.Buffer{})
	if !got.Color || !got.Control {
		t.Fatalf("expected FORCE_TTY(non-tty) => color+control on, got %+v", got)
	}
}
