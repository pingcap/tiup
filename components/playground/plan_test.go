package main

import (
	"slices"
	"strings"
	"testing"

	"github.com/pingcap/tiup/components/playground/proc"
)

type recordingSource struct {
	resolvedVersion string
}

func (s *recordingSource) ResolveVersion(component, constraint string) (string, error) {
	_ = component
	_ = constraint
	if s.resolvedVersion != "" {
		return s.resolvedVersion, nil
	}
	return "v0.0.0", nil
}

func (s *recordingSource) PlanInstall(serviceID proc.ServiceID, component, resolved string, forcePull bool) (*DownloadPlan, error) {
	_ = serviceID
	_ = forcePull
	return &DownloadPlan{
		ComponentID:     component,
		ResolvedVersion: resolved,
	}, nil
}

func (s *recordingSource) EnsureInstalled(component, resolved string) error {
	_ = component
	_ = resolved
	return nil
}

func (s *recordingSource) BinaryPath(component, resolved string) (string, error) {
	_ = resolved
	return component, nil
}

func TestBuildBootPlan_Downloads_DedupAndSorted(t *testing.T) {
	opts := &BootOptions{
		ShOpt: proc.SharedOptions{
			Mode:   proc.ModeNormal,
			PDMode: "ms",
		},
		Version: "nightly",
		Host:    "127.0.0.1",
	}

	fs := newTestFlagSet()
	registerServiceFlags(fs, opts)
	if err := fs.Parse([]string{"--pd=2"}); err != nil {
		t.Fatalf("parse flags: %v", err)
	}
	if err := applyServiceDefaults(fs, opts); err != nil {
		t.Fatalf("applyServiceDefaults: %v", err)
	}

	src := &recordingSource{resolvedVersion: "v1.0.0"}
	plan, err := BuildBootPlan(opts, bootPlannerConfig{
		dataDir:            "/tmp/tiup-playground-test-plan",
		portConflictPolicy: PortConflictNone,
		advertiseHost:      func(listen string) string { return listen },
		componentSource:    src,
	})
	if err != nil {
		t.Fatalf("BuildBootPlan: %v", err)
	}

	if got := len(plan.Downloads); got != 4 {
		t.Fatalf("unexpected downloads count: %d", got)
	}
	if plan.Downloads[0].ComponentID != proc.ComponentPD.String() {
		t.Fatalf("unexpected downloads[0] component: %q", plan.Downloads[0].ComponentID)
	}
	if plan.Downloads[1].ComponentID != proc.ComponentTiDB.String() {
		t.Fatalf("unexpected downloads[1] component: %q", plan.Downloads[1].ComponentID)
	}
	if plan.Downloads[2].ComponentID != proc.ComponentTiFlash.String() {
		t.Fatalf("unexpected downloads[2] component: %q", plan.Downloads[2].ComponentID)
	}
	if plan.Downloads[3].ComponentID != proc.ComponentTiKV.String() {
		t.Fatalf("unexpected downloads[3] component: %q", plan.Downloads[3].ComponentID)
	}
}

func TestBuildBootPlan_PortConflictNone_UniquePDPorts(t *testing.T) {
	opts := &BootOptions{
		ShOpt: proc.SharedOptions{
			Mode:   proc.ModeNormal,
			PDMode: "pd",
		},
		Version: "nightly",
		Host:    "127.0.0.1",
	}

	fs := newTestFlagSet()
	registerServiceFlags(fs, opts)
	if err := applyServiceDefaults(fs, opts); err != nil {
		t.Fatalf("applyServiceDefaults: %v", err)
	}
	opts.Service(proc.ServicePD).Num = 2

	plan, err := BuildBootPlan(opts, bootPlannerConfig{
		portConflictPolicy: PortConflictNone,
		advertiseHost:      func(listen string) string { return listen },
		componentSource:    fakeComponentSource{},
	})
	if err != nil {
		t.Fatalf("BuildBootPlan: %v", err)
	}

	var pdPorts []int
	var pdStatusPorts []int
	for _, svc := range plan.Services {
		if svc.ServiceID != proc.ServicePD.String() {
			continue
		}
		pdPorts = append(pdPorts, svc.Shared.Port)
		pdStatusPorts = append(pdStatusPorts, svc.Shared.StatusPort)
	}
	if len(pdPorts) != 2 || len(pdStatusPorts) != 2 {
		t.Fatalf("unexpected pd instances in plan: ports=%v status_ports=%v", pdPorts, pdStatusPorts)
	}
	if pdPorts[0] == pdPorts[1] {
		t.Fatalf("pd peer ports should be unique, got: %v", pdPorts)
	}
	if pdStatusPorts[0] == pdStatusPorts[1] {
		t.Fatalf("pd status ports should be unique, got: %v", pdStatusPorts)
	}
}

func TestBuildBootPlan_StartAfterServices_SortedAndDeduped(t *testing.T) {
	opts := &BootOptions{
		ShOpt: proc.SharedOptions{
			Mode:   proc.ModeNormal,
			PDMode: "pd",
		},
		Version: "nightly",
		Host:    "127.0.0.1",
		Monitor: true,
	}

	fs := newTestFlagSet()
	registerServiceFlags(fs, opts)
	if err := applyServiceDefaults(fs, opts); err != nil {
		t.Fatalf("applyServiceDefaults: %v", err)
	}

	plan, err := BuildBootPlan(opts, bootPlannerConfig{
		portConflictPolicy: PortConflictNone,
		advertiseHost:      func(listen string) string { return listen },
		componentSource:    fakeComponentSource{},
	})
	if err != nil {
		t.Fatalf("BuildBootPlan: %v", err)
	}

	found := false
	for _, svc := range plan.Services {
		if svc.ServiceID != proc.ServiceTiDB.String() {
			continue
		}
		found = true
		if len(svc.StartAfterServices) == 0 {
			t.Fatalf("expected tidb to have start_after constraints")
		}
		if !slices.IsSorted(svc.StartAfterServices) {
			t.Fatalf("expected start_after to be sorted, got: %v", svc.StartAfterServices)
		}
		for i := 1; i < len(svc.StartAfterServices); i++ {
			if strings.TrimSpace(svc.StartAfterServices[i]) == strings.TrimSpace(svc.StartAfterServices[i-1]) {
				t.Fatalf("expected start_after to be deduped, got: %v", svc.StartAfterServices)
			}
		}
		break
	}
	if !found {
		t.Fatalf("missing tidb service in plan")
	}
}
