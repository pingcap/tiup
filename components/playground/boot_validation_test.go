package main

import (
	"strings"
	"testing"

	"github.com/pingcap/tiup/components/playground/proc"
)

func TestValidateBootOptionsPure_ModeNormal_AllowsEmptyCSEOptions(t *testing.T) {
	opts := &BootOptions{
		ShOpt: proc.SharedOptions{
			Mode:   proc.ModeNormal,
			PDMode: "pd",
		},
		Version: "nightly",
	}
	opts.Service(proc.ServicePD).Num = 1

	if err := ValidateBootOptionsPure(opts); err != nil {
		t.Fatalf("ValidateBootOptionsPure: %v", err)
	}
}

func TestValidateBootOptionsPure_ModeCSE_RequiresBucket(t *testing.T) {
	opts := &BootOptions{
		ShOpt: proc.SharedOptions{
			Mode:   proc.ModeCSE,
			PDMode: "pd",
			CSE: proc.CSEOptions{
				S3Endpoint: "http://127.0.0.1:9000",
				Bucket:     "",
			},
		},
		Version: "nightly",
	}
	opts.Service(proc.ServicePD).Num = 1

	err := ValidateBootOptionsPure(opts)
	if err == nil {
		t.Fatalf("expected error, got nil")
	}
	if !strings.Contains(err.Error(), "bucket") {
		t.Fatalf("expected bucket error, got: %v", err)
	}
}

func TestValidateBootOptionsPure_ModeCSE_RequiresEndpointHost(t *testing.T) {
	opts := &BootOptions{
		ShOpt: proc.SharedOptions{
			Mode:   proc.ModeCSE,
			PDMode: "pd",
			CSE: proc.CSEOptions{
				S3Endpoint: "http://",
				Bucket:     "tiflash",
			},
		},
		Version: "nightly",
	}
	opts.Service(proc.ServicePD).Num = 1

	err := ValidateBootOptionsPure(opts)
	if err == nil {
		t.Fatalf("expected error, got nil")
	}
	if !strings.Contains(err.Error(), "endpoint") {
		t.Fatalf("expected endpoint error, got: %v", err)
	}
}

func TestValidateBootOptionsPure_ModeCSE_AllowsTrailingSlashInEndpoint(t *testing.T) {
	opts := &BootOptions{
		ShOpt: proc.SharedOptions{
			Mode:   proc.ModeCSE,
			PDMode: "pd",
			CSE: proc.CSEOptions{
				S3Endpoint: "http://127.0.0.1:9000/",
				Bucket:     "tiflash",
			},
		},
		Version: "nightly",
	}
	opts.Service(proc.ServicePD).Num = 1

	if err := ValidateBootOptionsPure(opts); err != nil {
		t.Fatalf("ValidateBootOptionsPure: %v", err)
	}
}

func TestValidateBootOptionsPure_ModeCSE_RejectsEndpointWithPath(t *testing.T) {
	opts := &BootOptions{
		ShOpt: proc.SharedOptions{
			Mode:   proc.ModeCSE,
			PDMode: "pd",
			CSE: proc.CSEOptions{
				S3Endpoint: "http://127.0.0.1:9000/minio",
				Bucket:     "tiflash",
			},
		},
		Version: "nightly",
	}
	opts.Service(proc.ServicePD).Num = 1

	err := ValidateBootOptionsPure(opts)
	if err == nil {
		t.Fatalf("expected error, got nil")
	}
	if !strings.Contains(err.Error(), "path") {
		t.Fatalf("expected path error, got: %v", err)
	}
}
