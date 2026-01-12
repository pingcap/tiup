package main

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/pingcap/tiup/components/playground/proc"
	"github.com/pingcap/tiup/pkg/repository"
	"github.com/pingcap/tiup/pkg/utils"
)

type fakeComponentBinaryInstaller struct {
	binPath string

	updateSpecs [][]repository.ComponentSpec
	onUpdate    func(specs []repository.ComponentSpec) error
}

func (f *fakeComponentBinaryInstaller) BinaryPath(component string, v utils.Version) (string, error) {
	return f.binPath, nil
}

func (f *fakeComponentBinaryInstaller) UpdateComponents(specs []repository.ComponentSpec) error {
	f.updateSpecs = append(f.updateSpecs, specs)
	if f.onUpdate != nil {
		return f.onUpdate(specs)
	}
	return nil
}

func TestPrepareComponentBinaryWithInstaller_BinaryExistsSkipsUpdate(t *testing.T) {
	dir := t.TempDir()
	binPath := filepath.Join(dir, "bin")
	if err := os.WriteFile(binPath, []byte("ok"), 0o755); err != nil {
		t.Fatalf("write bin: %v", err)
	}

	inst := &fakeComponentBinaryInstaller{binPath: binPath}
	out, err := prepareComponentBinaryWithInstaller(inst, proc.ServicePrometheus, "prometheus", utils.Version("v1.0.0"), false)
	if err != nil {
		t.Fatalf("prepareComponentBinaryWithInstaller: %v", err)
	}
	if out != binPath {
		t.Fatalf("unexpected binary path: %q", out)
	}
	if len(inst.updateSpecs) != 0 {
		t.Fatalf("unexpected UpdateComponents calls: %d", len(inst.updateSpecs))
	}
}

func TestPrepareComponentBinaryWithInstaller_BinaryMissingForcesUpdate(t *testing.T) {
	dir := t.TempDir()
	binPath := filepath.Join(dir, "bin")

	inst := &fakeComponentBinaryInstaller{
		binPath: binPath,
		onUpdate: func(specs []repository.ComponentSpec) error {
			if len(specs) != 1 {
				t.Fatalf("unexpected specs: %+v", specs)
			}
			if !specs[0].Force {
				t.Fatalf("expected Force=true, got %+v", specs[0])
			}
			if err := os.WriteFile(binPath, []byte("ok"), 0o755); err != nil {
				t.Fatalf("write bin: %v", err)
			}
			return nil
		},
	}

	out, err := prepareComponentBinaryWithInstaller(inst, proc.ServicePrometheus, "prometheus", utils.Version("v1.0.0"), false)
	if err != nil {
		t.Fatalf("prepareComponentBinaryWithInstaller: %v", err)
	}
	if out != binPath {
		t.Fatalf("unexpected binary path: %q", out)
	}
	if len(inst.updateSpecs) != 1 {
		t.Fatalf("expected 1 UpdateComponents call, got %d", len(inst.updateSpecs))
	}
}

func TestPrepareComponentBinaryWithInstaller_AfterUpdateStillMissingReturnsError(t *testing.T) {
	dir := t.TempDir()
	binPath := filepath.Join(dir, "bin")

	inst := &fakeComponentBinaryInstaller{
		binPath: binPath,
		onUpdate: func(specs []repository.ComponentSpec) error {
			return nil
		},
	}

	_, err := prepareComponentBinaryWithInstaller(inst, proc.ServicePrometheus, "prometheus", utils.Version("v1.0.0"), false)
	if err == nil {
		t.Fatalf("expected error")
	}
	if len(inst.updateSpecs) != 1 {
		t.Fatalf("expected 1 UpdateComponents call, got %d", len(inst.updateSpecs))
	}
}

func TestPrepareComponentBinaryWithInstaller_MissingRequiredBinaryForService_ForcesUpdate(t *testing.T) {
	dir := t.TempDir()
	baseBinPath := filepath.Join(dir, "tikv-server")
	if err := os.WriteFile(baseBinPath, []byte("ok"), 0o755); err != nil {
		t.Fatalf("write tikv-server: %v", err)
	}

	requiredBinPath := filepath.Join(dir, "tikv-worker")

	inst := &fakeComponentBinaryInstaller{
		binPath: baseBinPath,
		onUpdate: func(specs []repository.ComponentSpec) error {
			if len(specs) != 1 {
				t.Fatalf("unexpected specs: %+v", specs)
			}
			if !specs[0].Force {
				t.Fatalf("expected Force=true, got %+v", specs[0])
			}
			if err := os.WriteFile(requiredBinPath, []byte("ok"), 0o755); err != nil {
				t.Fatalf("write tikv-worker: %v", err)
			}
			return nil
		},
	}

	out, err := prepareComponentBinaryWithInstaller(inst, proc.ServiceTiKVWorker, "tikv", utils.Version("v1.0.0"), false)
	if err != nil {
		t.Fatalf("prepareComponentBinaryWithInstaller: %v", err)
	}
	if out != requiredBinPath {
		t.Fatalf("unexpected binary path: %q", out)
	}
	if len(inst.updateSpecs) != 1 {
		t.Fatalf("expected 1 UpdateComponents call, got %d", len(inst.updateSpecs))
	}
}

func TestPrepareComponentBinaryWithInstaller_NGMonitoring_UsesSiblingBinary(t *testing.T) {
	dir := t.TempDir()
	baseBinPath := filepath.Join(dir, "prometheus")
	if err := os.WriteFile(baseBinPath, []byte("ok"), 0o755); err != nil {
		t.Fatalf("write prometheus: %v", err)
	}

	requiredBinPath := filepath.Join(dir, "ng-monitoring-server")

	inst := &fakeComponentBinaryInstaller{
		binPath: baseBinPath,
		onUpdate: func(specs []repository.ComponentSpec) error {
			if len(specs) != 1 {
				t.Fatalf("unexpected specs: %+v", specs)
			}
			if !specs[0].Force {
				t.Fatalf("expected Force=true, got %+v", specs[0])
			}
			if err := os.WriteFile(requiredBinPath, []byte("ok"), 0o755); err != nil {
				t.Fatalf("write ng-monitoring-server: %v", err)
			}
			return nil
		},
	}

	out, err := prepareComponentBinaryWithInstaller(inst, proc.ServiceNGMonitoring, "prometheus", utils.Version("v1.0.0"), false)
	if err != nil {
		t.Fatalf("prepareComponentBinaryWithInstaller: %v", err)
	}
	if out != requiredBinPath {
		t.Fatalf("unexpected binary path: %q", out)
	}
	if len(inst.updateSpecs) != 1 {
		t.Fatalf("expected 1 UpdateComponents call, got %d", len(inst.updateSpecs))
	}
}
