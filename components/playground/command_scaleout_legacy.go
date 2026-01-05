package main

import (
	"fmt"
	"slices"
	"strings"

	"github.com/pingcap/tiup/components/playground/proc"
	"github.com/spf13/cobra"
)

// --- Legacy scale-out flag compatibility ------------------------------------
//
// This block implements the legacy usage:
//   tiup playground scale-out --db 1 --kv 2 ...
//
// It intentionally lives in one place so the modern --service/--count codepath
// stays readable.

type legacyScaleOutFlagSpec struct {
	flagPrefix string
	serviceID  proc.ServiceID

	hasHost    bool
	hasConfig  bool
	hasBinPath bool
}

type legacyScaleOutServiceFlags struct {
	count   int
	host    string
	config  string
	binpath string
}

type legacyScaleOutFlags struct {
	services map[proc.ServiceID]*legacyScaleOutServiceFlags
}

func registerLegacyScaleOutFlags(cmd *cobra.Command) *legacyScaleOutFlags {
	out := &legacyScaleOutFlags{services: make(map[proc.ServiceID]*legacyScaleOutServiceFlags)}
	if cmd == nil {
		return out
	}

	specs := []legacyScaleOutFlagSpec{
		{flagPrefix: "db", serviceID: proc.ServiceTiDB, hasHost: true, hasConfig: true, hasBinPath: true},
		{flagPrefix: "dm-master", serviceID: proc.ServiceDMMaster, hasConfig: true, hasBinPath: true},
		{flagPrefix: "dm-worker", serviceID: proc.ServiceDMWorker, hasConfig: true, hasBinPath: true},
		{flagPrefix: "drainer", serviceID: proc.ServiceDrainer, hasConfig: true, hasBinPath: true},
		{flagPrefix: "kv", serviceID: proc.ServiceTiKV, hasConfig: true, hasBinPath: true},
		{flagPrefix: "kvcdc", serviceID: proc.ServiceTiKVCDC, hasBinPath: true},
		{flagPrefix: "pd", serviceID: proc.ServicePD, hasHost: true, hasConfig: true, hasBinPath: true},
		{flagPrefix: "pump", serviceID: proc.ServicePump, hasConfig: true, hasBinPath: true},
		{flagPrefix: "scheduling", serviceID: proc.ServicePDScheduling, hasHost: true, hasConfig: true, hasBinPath: true},
		{flagPrefix: "ticdc", serviceID: proc.ServiceTiCDC, hasBinPath: true},
		{flagPrefix: "tiflash", serviceID: proc.ServiceTiFlash, hasConfig: true, hasBinPath: true},
		{flagPrefix: "tiproxy", serviceID: proc.ServiceTiProxy, hasHost: true, hasConfig: true, hasBinPath: true},
		{flagPrefix: "tso", serviceID: proc.ServicePDTSO, hasHost: true, hasConfig: true, hasBinPath: true},
	}

	flags := cmd.Flags()
	for _, spec := range specs {
		spec := spec
		if spec.serviceID == "" || spec.flagPrefix == "" {
			continue
		}
		val := &legacyScaleOutServiceFlags{}
		out.services[spec.serviceID] = val

		flags.IntVar(&val.count, spec.flagPrefix, 0, "LEGACY: scale-out count for "+spec.serviceID.String())
		_ = flags.MarkHidden(spec.flagPrefix)

		if spec.hasHost {
			name := spec.flagPrefix + ".host"
			flags.StringVar(&val.host, name, "", "LEGACY: host for "+spec.serviceID.String())
			_ = flags.MarkHidden(name)
		}
		if spec.hasConfig {
			name := spec.flagPrefix + ".config"
			flags.StringVar(&val.config, name, "", "LEGACY: config for "+spec.serviceID.String())
			_ = flags.MarkHidden(name)
		}
		if spec.hasBinPath {
			name := spec.flagPrefix + ".binpath"
			flags.StringVar(&val.binpath, name, "", "LEGACY: binpath for "+spec.serviceID.String())
			_ = flags.MarkHidden(name)
		}
	}

	return out
}

func (f *legacyScaleOutFlags) requests() ([]ScaleOutRequest, error) {
	if f == nil || len(f.services) == 0 {
		return nil, nil
	}

	reqs := make([]ScaleOutRequest, 0, len(f.services))
	serviceIDs := make([]proc.ServiceID, 0, len(f.services))
	for serviceID := range f.services {
		serviceIDs = append(serviceIDs, serviceID)
	}
	slices.SortFunc(serviceIDs, func(a, b proc.ServiceID) int {
		return strings.Compare(a.String(), b.String())
	})

	for _, serviceID := range serviceIDs {
		flags := f.services[serviceID]
		if serviceID == "" || flags == nil || flags.count <= 0 {
			continue
		}
		cfg := proc.Config{
			Host:       flags.host,
			ConfigPath: flags.config,
			BinPath:    flags.binpath,
		}
		reqs = append(reqs, ScaleOutRequest{
			ServiceID: serviceID,
			Count:     flags.count,
			Config:    cfg,
		})
	}

	for _, req := range reqs {
		if req.ServiceID == "" || req.Count <= 0 {
			return nil, fmt.Errorf("invalid legacy scale-out request: service=%q count=%d", req.ServiceID, req.Count)
		}
	}
	return reqs, nil
}

func (f *legacyScaleOutFlags) hasCount() bool {
	if f == nil {
		return false
	}
	for _, s := range f.services {
		if s != nil && s.count > 0 {
			return true
		}
	}
	return false
}
