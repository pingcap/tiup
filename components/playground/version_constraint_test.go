package main

import (
	"testing"

	"github.com/pingcap/tiup/components/playground/proc"
	"github.com/pingcap/tiup/pkg/utils"
)

func TestResolveVersionConstraint_UsesLatestAliasByDefault(t *testing.T) {
	options := &BootOptions{}
	got, err := resolveVersionConstraint(proc.ServiceTiProxy, options)
	if err != nil {
		t.Fatalf("resolveVersionConstraint: %v", err)
	}
	if got != utils.LatestVersionAlias {
		t.Fatalf("unexpected version constraint: %q", got)
	}
}

func TestResolveVersionConstraint_ServiceOverrideAndBind(t *testing.T) {
	options := &BootOptions{Version: "latest-" + utils.NextgenVersionAlias}
	options.Service(proc.ServiceTiProxy).Version = "v9.9.9"
	options.Service(proc.ServicePrometheus).Version = "v0.0.0-should-be-ignored"

	got, err := resolveVersionConstraint(proc.ServiceTiProxy, options)
	if err != nil {
		t.Fatalf("resolveVersionConstraint(tiproxy): %v", err)
	}
	if got != "v9.9.9" {
		t.Fatalf("unexpected tiproxy version constraint: %q", got)
	}

	got, err = resolveVersionConstraint(proc.ServicePrometheus, options)
	if err != nil {
		t.Fatalf("resolveVersionConstraint(prometheus): %v", err)
	}
	if got != utils.LatestVersionAlias {
		t.Fatalf("unexpected prometheus version constraint: %q", got)
	}
}

func TestPlaygroundVersionConstraintForService_NoImplicitDefault(t *testing.T) {
	pg := &Playground{bootOptions: &BootOptions{}}
	got := pg.versionConstraintForService(proc.ServiceTiProxy, "")
	if got != "" {
		t.Fatalf("unexpected version constraint: %q", got)
	}
}
