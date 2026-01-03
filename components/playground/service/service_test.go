package service

import (
	"strings"
	"testing"
)

func TestAllSpecsSortedByServiceID(t *testing.T) {
	specs := AllSpecs()
	if len(specs) == 0 {
		t.Fatalf("AllSpecs returned empty list")
	}

	seen := make(map[string]struct{}, len(specs))
	last := ""
	for i, spec := range specs {
		id := spec.ServiceID.String()
		if id == "" {
			t.Fatalf("spec[%d] has empty serviceID", i)
		}
		if _, ok := seen[id]; ok {
			t.Fatalf("duplicate serviceID %q", id)
		}
		seen[id] = struct{}{}
		if i > 0 && strings.Compare(last, id) > 0 {
			t.Fatalf("service IDs not sorted: %q before %q", last, id)
		}
		last = id
	}
}

