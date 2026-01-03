package main

import (
	"github.com/pingcap/errors"
	"github.com/pingcap/tiup/components/playground/proc"
	"github.com/spf13/pflag"
)

func registerServiceFlags(flagSet *pflag.FlagSet, opts *BootOptions) {
	if flagSet == nil || opts == nil {
		return
	}
	for _, def := range serviceCatalog {
		if def.FlagPrefix == "" {
			continue
		}
		cfg := opts.Service(def.ServiceID)
		if cfg == nil {
			continue
		}

		if def.HasCount {
			flagSet.IntVar(&cfg.Num, def.countFlag(), 0, def.helpCount())
		}
		if def.HasHost {
			flagSet.StringVar(&cfg.Host, def.hostFlag(), "", def.helpHost())
		}
		if def.HasPort {
			flagSet.IntVar(&cfg.Port, def.portFlag(), def.PortDefault, def.helpPort())
		}
		if def.HasConfig {
			flagSet.StringVar(&cfg.ConfigPath, def.configFlag(), "", def.helpConfig())
		}
		if def.HasBinPath {
			flagSet.StringVar(&cfg.BinPath, def.binPathFlag(), "", def.helpBinPath())
		}
		if def.HasTimeout {
			flagSet.IntVar(&cfg.UpTimeout, def.timeoutFlag(), def.TimeoutDefault, def.helpTimeout())
		}
		if def.HasVersion {
			flagSet.StringVar(&cfg.Version, def.versionFlag(), "", def.helpVersion())
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

	// Apply default counts first (some later defaults depend on these).
	for _, def := range serviceCatalog {
		if def.FlagPrefix == "" || !def.HasCount || def.DefaultNum == nil {
			continue
		}
		cfg := opts.Service(def.ServiceID)
		if cfg == nil {
			continue
		}
		defaultInt(def.countFlag(), &cfg.Num, def.DefaultNum(opts))
	}

	// Apply field defaults derived from another service.
	for _, def := range serviceCatalog {
		if def.FlagPrefix == "" {
			continue
		}
		cfg := opts.Service(def.ServiceID)
		if cfg == nil {
			continue
		}

		if def.DefaultBinPathFrom != "" && def.HasBinPath {
			defaultStr(def.binPathFlag(), &cfg.BinPath, opts.Service(def.DefaultBinPathFrom).BinPath)
		}
		if def.DefaultConfigPathFrom != "" && def.HasConfig {
			defaultStr(def.configFlag(), &cfg.ConfigPath, opts.Service(def.DefaultConfigPathFrom).ConfigPath)
		}
		if def.DefaultTimeoutFrom != "" {
			cfg.UpTimeout = opts.Service(def.DefaultTimeoutFrom).UpTimeout
		}
	}

	// Apply full config copies last.
	for _, def := range serviceCatalog {
		if def.CopyFrom == "" || def.CopyWhen == nil || !def.CopyWhen(opts) {
			continue
		}
		dst := opts.Service(def.ServiceID)
		src := opts.Service(def.CopyFrom)
		if dst == nil || src == nil {
			continue
		}
		*dst = *src
	}

	return nil
}
