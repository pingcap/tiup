package main

import (
	"fmt"
	"slices"
	"strings"

	"github.com/pingcap/errors"
	"github.com/pingcap/tiup/components/playground-ng/proc"
	pgservice "github.com/pingcap/tiup/components/playground-ng/service"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
)

func registerServiceFlags(flagSet *pflag.FlagSet, opts *BootOptions) {
	if flagSet == nil || opts == nil {
		return
	}

	serviceDisplayName := func(serviceID proc.ServiceID) string {
		if serviceID != "" {
			if s := proc.ServiceDisplayName(serviceID); s != "" {
				return s
			}
			return serviceID.String()
		}
		return "service"
	}

	for _, spec := range pgservice.AllSpecs() {
		def := spec.Catalog
		if def.FlagPrefix == "" || spec.ServiceID == "" {
			continue
		}
		cfg := opts.Service(spec.ServiceID)
		if cfg == nil {
			continue
		}

		displayName := serviceDisplayName(spec.ServiceID)
		countFlag := def.FlagPrefix
		hostFlag := def.FlagPrefix + ".host"
		portFlag := def.FlagPrefix + ".port"
		configFlag := def.FlagPrefix + ".config"
		binPathFlag := def.FlagPrefix + ".binpath"
		timeoutFlag := def.FlagPrefix + ".timeout"
		versionFlag := def.FlagPrefix + ".version"

		if def.AllowModifyNum {
			flagSet.IntVar(&cfg.Num, countFlag, 0, displayName+" instance number")
		}
		if def.AllowModifyHost {
			flagSet.StringVar(&cfg.Host, hostFlag, "", displayName+" host (default: --host)")
		}
		if def.AllowModifyPort {
			flagSet.IntVar(&cfg.Port, portFlag, def.DefaultPort, displayName+" port (0 means default)")
		}
		if def.AllowModifyConfig {
			flagSet.StringVar(&cfg.ConfigPath, configFlag, "", displayName+" instance configuration file")
		}
		if def.AllowModifyBinPath {
			flagSet.StringVar(&cfg.BinPath, binPathFlag, "", displayName+" instance binary path")
		}
		if def.AllowModifyTimeout {
			flagSet.IntVar(&cfg.UpTimeout, timeoutFlag, def.DefaultTimeout, displayName+" max wait time in seconds for starting, 0 means no limit")
		}
		if def.AllowModifyVersion {
			flagSet.StringVar(&cfg.Version, versionFlag, "", displayName+" instance version (override)")
		}
	}
}

func applyServiceDefaults(flagSet *pflag.FlagSet, opts *BootOptions) error {
	if flagSet == nil || opts == nil {
		return nil
	}

	switch opts.ShOpt.Mode {
	case proc.ModeNormal, proc.ModeNextGen, proc.ModeTiKVSlim, proc.ModeCSE, proc.ModeDisAgg:
	default:
		return errors.Errorf("Unknown --mode %s. Available modes are %s", opts.ShOpt.Mode, strings.Join([]string{
			proc.ModeNormal,
			proc.ModeNextGen,
			proc.ModeTiKVSlim,
			proc.ModeCSE,
			proc.ModeDisAgg,
		}, ", "))
	}

	switch opts.ShOpt.PDMode {
	case "pd", "ms":
	default:
		return errors.Errorf("Unknown --pd.mode %s", opts.ShOpt.PDMode)
	}

	defaultStr := func(flagName string, dst *string, v string) {
		if dst == nil || flagName == "" {
			return
		}
		f := flagSet.Lookup(flagName)
		if f == nil || f.Changed {
			return
		}
		*dst = v
	}
	defaultInt := func(flagName string, dst *int, v int) {
		if dst == nil || flagName == "" {
			return
		}
		f := flagSet.Lookup(flagName)
		if f == nil || f.Changed {
			return
		}
		*dst = v
	}

	// Apply default counts first (some later defaults depend on these).
	{
		specs := pgservice.AllSpecs()
		for i := 0; i < 8; i++ {
			changed := false
			for _, spec := range specs {
				def := spec.Catalog
				if def.FlagPrefix == "" || spec.ServiceID == "" || !def.AllowModifyNum || def.DefaultNum == nil {
					continue
				}
				cfg := opts.Service(spec.ServiceID)
				if cfg == nil {
					continue
				}
				flagName := def.FlagPrefix
				f := flagSet.Lookup(flagName)
				if f == nil || f.Changed {
					continue
				}
				want := def.DefaultNum(opts)
				if cfg.Num != want {
					cfg.Num = want
					changed = true
				}
			}
			if !changed {
				break
			}
		}
	}

	// Apply field defaults derived from another service.
	for _, spec := range pgservice.AllSpecs() {
		def := spec.Catalog
		if def.FlagPrefix == "" || spec.ServiceID == "" {
			continue
		}
		cfg := opts.Service(spec.ServiceID)
		if cfg == nil {
			continue
		}

		if def.DefaultBinPathFrom != "" && def.AllowModifyBinPath {
			defaultStr(def.FlagPrefix+".binpath", &cfg.BinPath, opts.Service(def.DefaultBinPathFrom).BinPath)
		}
		if def.DefaultConfigPathFrom != "" && def.AllowModifyConfig {
			defaultStr(def.FlagPrefix+".config", &cfg.ConfigPath, opts.Service(def.DefaultConfigPathFrom).ConfigPath)
		}
		if def.DefaultHostFrom != "" && def.AllowModifyHost {
			defaultStr(def.FlagPrefix+".host", &cfg.Host, opts.Service(def.DefaultHostFrom).Host)
		}
		if def.DefaultPortFrom != "" && def.AllowModifyPort {
			defaultInt(def.FlagPrefix+".port", &cfg.Port, opts.Service(def.DefaultPortFrom).Port)
		}
		if def.DefaultTimeoutFrom != "" {
			cfg.UpTimeout = opts.Service(def.DefaultTimeoutFrom).UpTimeout
		}
	}

	return nil
}

// --- Legacy scale-out flag compatibility ------------------------------------
//
// This block implements the legacy usage:
//   tiup playground-ng scale-out --db 1 --kv 2 ...
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
