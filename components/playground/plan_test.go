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

func TestPortPlanner_PortConflictNone_WildcardConflictsWithSpecificHost(t *testing.T) {
	p := newPortPlanner(PortConflictNone)

	a, err := p.alloc("127.0.0.1", 10080, 0)
	if err != nil {
		t.Fatalf("alloc: %v", err)
	}
	b, err := p.alloc("0.0.0.0", 10080, 0)
	if err != nil {
		t.Fatalf("alloc: %v", err)
	}
	if b == a {
		t.Fatalf("expected wildcard host port to avoid conflicts, got: %d", b)
	}

	c, err := p.alloc("127.0.0.1", 10080, 0)
	if err != nil {
		t.Fatalf("alloc: %v", err)
	}
	if c == a || c == b {
		t.Fatalf("expected per-host port to avoid wildcard conflicts, got: %d", c)
	}
}

func TestBuildBootPlan_Downloads_SkipsWhenUserProvidesBinPath(t *testing.T) {
	opts := &BootOptions{
		ShOpt: proc.SharedOptions{
			Mode:   proc.ModeNormal,
			PDMode: "pd",
		},
		Version: "nightly",
		Host:    "127.0.0.1",
		Monitor: false,
	}

	fs := newTestFlagSet()
	registerServiceFlags(fs, opts)
	if err := fs.Parse([]string{
		"--db.binpath=/tmp/tidb-server",
		"--tiflash=0",
	}); err != nil {
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

	foundTiDB := false
	for _, svc := range plan.Services {
		if svc.ServiceID != proc.ServiceTiDB.String() {
			continue
		}
		foundTiDB = true
		if svc.BinPath != "/tmp/tidb-server" {
			t.Fatalf("unexpected tidb binpath: %q", svc.BinPath)
		}
		break
	}
	if !foundTiDB {
		t.Fatalf("missing %s service in plan", proc.ServiceTiDB)
	}

	for _, dl := range plan.Downloads {
		if dl.ComponentID == proc.ComponentTiDB.String() {
			t.Fatalf("unexpected download plan for %s when user provides binpath: %+v", proc.ComponentTiDB, dl)
		}
	}
}
