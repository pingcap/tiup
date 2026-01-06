package main

import (
	"slices"

	"github.com/pingcap/tiup/components/playground/proc"
)

// BootOptions is the topology and options used to start a playground cluster.
//
// Per-service options are stored in Services to avoid the "add a service, update
// N different field lists" failure mode.
type BootOptions struct {
	ShOpt       proc.SharedOptions `yaml:"shared_opt"`
	Version     string             `yaml:"version"`
	Host        string             `yaml:"host"`
	Monitor     bool               `yaml:"monitor"`
	GrafanaPort int                `yaml:"grafana_port"`

	Services map[proc.ServiceID]*proc.Config `yaml:"services,omitempty"`
}

// Service returns the mutable per-service config, allocating it on demand.
func (o *BootOptions) Service(serviceID proc.ServiceID) *proc.Config {
	if o == nil || serviceID == "" {
		return nil
	}
	if o.Services == nil {
		o.Services = make(map[proc.ServiceID]*proc.Config)
	}
	if cfg := o.Services[serviceID]; cfg != nil {
		return cfg
	}
	cfg := &proc.Config{}
	o.Services[serviceID] = cfg
	return cfg
}

// ServiceConfig returns a copy of the per-service config if available.
func (o *BootOptions) ServiceConfig(serviceID proc.ServiceID) (proc.Config, bool) {
	if o == nil || serviceID == "" {
		return proc.Config{}, false
	}
	cfg := o.Service(serviceID)
	if cfg == nil {
		return proc.Config{}, false
	}
	return *cfg, true
}

// SortedServiceIDs returns all configured service IDs in deterministic order.
func (o *BootOptions) SortedServiceIDs() []proc.ServiceID {
	if o == nil || len(o.Services) == 0 {
		return nil
	}
	out := make([]proc.ServiceID, 0, len(o.Services))
	for id := range o.Services {
		out = append(out, id)
	}
	slices.SortStableFunc(out, func(a, b proc.ServiceID) int {
		if a < b {
			return -1
		}
		if a > b {
			return 1
		}
		return 0
	})
	return out
}

// SharedOptions returns the boot-time shared options.
func (o *BootOptions) SharedOptions() proc.SharedOptions {
	if o == nil {
		return proc.SharedOptions{}
	}
	return o.ShOpt
}

// BootVersion returns the boot-time version constraint.
func (o *BootOptions) BootVersion() string {
	if o == nil {
		return ""
	}
	return o.Version
}

// MonitorEnabled reports whether monitoring components are enabled.
func (o *BootOptions) MonitorEnabled() bool {
	return o != nil && o.Monitor
}

// GrafanaPortOverride returns the configured Grafana port override.
func (o *BootOptions) GrafanaPortOverride() int {
	if o == nil {
		return 0
	}
	return o.GrafanaPort
}

// ServiceConfigFor returns the current config snapshot for a service.
func (o *BootOptions) ServiceConfigFor(serviceID proc.ServiceID) proc.Config {
	if o == nil || serviceID == "" {
		return proc.Config{}
	}
	cfg := o.Service(serviceID)
	if cfg == nil {
		return proc.Config{}
	}
	return *cfg
}
