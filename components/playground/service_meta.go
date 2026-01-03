package main

import (
	"slices"
	"strings"

	"github.com/pingcap/tiup/components/playground/proc"
	pgservice "github.com/pingcap/tiup/components/playground/service"
)

func scaleOutServiceIDs() []proc.ServiceID {
	var out []proc.ServiceID
	for _, spec := range pgservice.AllSpecs() {
		if spec.ServiceID == "" || !spec.Catalog.AllowScaleOut {
			continue
		}
		out = append(out, spec.ServiceID)
	}
	slices.SortFunc(out, func(a, b proc.ServiceID) int {
		return strings.Compare(a.String(), b.String())
	})
	return out
}
