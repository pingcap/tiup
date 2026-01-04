package main

import (
	"github.com/pingcap/errors"
	"github.com/pingcap/tiup/components/playground/proc"
	pgservice "github.com/pingcap/tiup/components/playground/service"
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
		return errors.Errorf("Unknown --mode %s", opts.ShOpt.Mode)
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
